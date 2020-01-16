package netns

import (
	"context"
	"runtime"

	"github.com/apex/log"
	"github.com/pkg/errors"
	"github.com/vishvananda/netns"

	"github.com/deviceplane/deviceplane/pkg/engine"
)

type Manager struct {
	engine engine.Engine
}

func NewManager(engine engine.Engine) *Manager {
	return &Manager{
		engine: engine,
	}
}

func (m *Manager) RunInContainerNamespace(ctx context.Context, containerID string, f func()) error {
	inspectResponse, err := m.engine.InspectContainer(ctx, containerID)
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	failedToSwitchNamespace := false
	defer func() {
		if !failedToSwitchNamespace {
			runtime.UnlockOSThread()
		}
	}()

	originalNamespace, err := netns.Get()
	if err != nil {
		return err
	}
	defer originalNamespace.Close()
	defer func() {
		err := netns.Set(originalNamespace)
		log.WithField("original namespace", originalNamespace).
			WithError(errors.Wrap(err, "switching to original namespace"))

		err = netns.Set(originalNamespace)
		if err != nil {
			log.WithField("original namespace", originalNamespace).
				Debug("succeeded on second attempt at switching!")
			return
		}

		log.WithField("original namespace", originalNamespace).
			WithError(errors.Wrap(err, "second attempt at switching to original namespace"))
		failedToSwitchNamespace = true
	}()

	containerNamespace, err := netns.GetFromPid(inspectResponse.PID)
	if err != nil {
		return err
	}
	defer containerNamespace.Close()

	if err := netns.Set(containerNamespace); err != nil {
		return err
	}

	f()

	return nil
}
