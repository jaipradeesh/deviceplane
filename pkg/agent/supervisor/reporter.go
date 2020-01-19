package supervisor

import (
	"context"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/deviceplane/deviceplane/pkg/models"
)

type Reporter struct {
	applicationID           string
	reportApplicationStatus func(ctx context.Context, applicationID string, currentRelease string) error
	reportServiceStatus     func(ctx context.Context, applicationID, service string, currentRelease string, state models.ServiceState, containerError string) error

	desiredApplicationRelease      string
	desiredApplicationServiceNames map[string]struct{}
	reportedApplicationRelease     string
	applicationStatusReporterDone  chan struct{}

	serviceStatuses           map[string]models.SetDeviceServiceStatusRequest
	reportedServiceStatuses   map[string]models.SetDeviceServiceStatusRequest
	serviceStatusReporterDone chan struct{}

	once   sync.Once
	lock   sync.RWMutex
	ctx    context.Context
	cancel func()
}

func NewReporter(
	applicationID string,
	reportApplicationStatus func(ctx context.Context, applicationID, currentRelease string) error,
	reportServiceStatus func(ctx context.Context, applicationID, service string, currentRelease string, state models.ServiceState, containerError string) error,
) *Reporter {
	ctx, cancel := context.WithCancel(context.Background())
	return &Reporter{
		applicationID:           applicationID,
		reportApplicationStatus: reportApplicationStatus,
		reportServiceStatus:     reportServiceStatus,

		desiredApplicationServiceNames: make(map[string]struct{}),
		applicationStatusReporterDone:  make(chan struct{}),
		serviceStatuses:                make(map[string]models.SetDeviceServiceStatusRequest),
		reportedServiceStatuses:        make(map[string]models.SetDeviceServiceStatusRequest),
		serviceStatusReporterDone:      make(chan struct{}),

		ctx:    ctx,
		cancel: cancel,
	}
}

func (r *Reporter) SetDesiredApplication(release string, applicationConfig map[string]models.Service) {
	serviceNames := make(map[string]struct{})
	for serviceName := range applicationConfig {
		serviceNames[serviceName] = struct{}{}
	}

	r.lock.Lock()
	r.desiredApplicationRelease = release
	r.desiredApplicationServiceNames = serviceNames
	r.lock.Unlock()

	r.once.Do(func() {
		go r.applicationStatusReporter()
		go r.serviceStatusReporter()
	})
}

func (r *Reporter) SetServiceStatus(serviceName string, status models.SetDeviceServiceStatusRequest) {
	r.lock.Lock()
	r.serviceStatuses[serviceName] = status
	r.lock.Unlock()
}

func (r *Reporter) Stop() {
	r.cancel()
	// TODO: don't do this if SetDesiredApplication was never called
	<-r.applicationStatusReporterDone
	<-r.serviceStatusReporterDone
}

func (r *Reporter) applicationStatusReporter() {
	ticker := time.NewTicker(defaultTickerFrequency)
	defer ticker.Stop()

	for {
		r.lock.RLock()
		releaseToReport := r.desiredApplicationRelease
		if releaseToReport == r.reportedApplicationRelease {
			r.lock.RUnlock()
			goto cont
		}
		for serviceName := range r.desiredApplicationServiceNames {
			status, ok := r.serviceStatuses[serviceName]
			if !ok || status.CurrentReleaseID != releaseToReport {
				r.lock.RUnlock()
				goto cont
			}
		}
		r.lock.RUnlock()

		if err := r.reportApplicationStatus(r.ctx, r.applicationID, releaseToReport); err != nil {
			log.WithError(err).Error("report application status")
			goto cont
		}

		r.reportedApplicationRelease = releaseToReport

	cont:
		select {
		case <-r.ctx.Done():
			r.applicationStatusReporterDone <- struct{}{}
			return
		case <-ticker.C:
			continue
		}
	}
}

func (r *Reporter) serviceStatusReporter() {
	ticker := time.NewTicker(defaultTickerFrequency)
	defer ticker.Stop()

	for {
		r.lock.RLock()
		diff := make(map[string]models.SetDeviceServiceStatusRequest)
		copy := make(map[string]models.SetDeviceServiceStatusRequest)
		for service, status := range r.serviceStatuses {
			reportedStatus, ok := r.reportedServiceStatuses[service]
			if !ok ||
				(reportedStatus.CurrentReleaseID != status.CurrentReleaseID ||
					reportedStatus.CurrentState != status.CurrentState ||
					reportedStatus.ErrorMessage != status.ErrorMessage) {
				diff[service] = status
			}
			copy[service] = status
		}
		r.lock.RUnlock()

		for serviceName, status := range diff {
			if err := r.reportServiceStatus(
				r.ctx,
				r.applicationID,
				serviceName,
				status.CurrentReleaseID,
				status.CurrentState,
				status.ErrorMessage,
			); err != nil {
				log.WithError(err).Error("report service status")
				goto cont
			}
		}

		r.reportedServiceStatuses = copy

	cont:
		select {
		case <-r.ctx.Done():
			r.serviceStatusReporterDone <- struct{}{}
			return
		case <-ticker.C:
			continue
		}
	}
}
