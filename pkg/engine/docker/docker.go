package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/deviceplane/deviceplane/pkg/engine"
	"github.com/deviceplane/deviceplane/pkg/models"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

var _ engine.Engine = &Engine{}

type Engine struct {
	client *client.Client
}

func NewEngine() (*Engine, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return &Engine{
		client: client,
	}, nil
}

func (e *Engine) CreateContainer(ctx context.Context, name string, s models.Service) (string, error) {
	config, hostConfig, err := convert(s)
	if err != nil {
		return "", err
	}

	resp, err := e.client.ContainerCreate(ctx, config, hostConfig, nil, name)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (e *Engine) InspectContainer(ctx context.Context, id string) (*engine.InspectResponse, error) {
	container, err := e.client.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	return &engine.InspectResponse{
		PID: container.State.Pid,
	}, nil
}

// TODO: remove this for this PR
func (e *Engine) GetContainerStderr(ctx context.Context, id string) (*string, error) {
	rc, err := e.client.ContainerLogs(ctx, id, types.ContainerLogsOptions{
		ShowStdout: false,
		ShowStderr: true,
		Since:      "1m",
		Tail:       "10",
		Follow:     false,
		Details:    false,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "could not get container logs")
	}

	defer rc.Close()
	buf, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, errors.WithMessage(err, "could not read from container logs buffer")
	}

	str := string(buf)
	return &str, nil
}

func (e *Engine) StartContainer(ctx context.Context, id string) error {
	if err := e.client.ContainerStart(ctx, id, types.ContainerStartOptions{}); err != nil {
		// TODO
		if strings.Contains(err.Error(), "No such container") {
			return engine.ErrInstanceNotFound
		}
		return err
	}
	return nil
}

func (e *Engine) ListContainers(ctx context.Context, keyFilters map[string]struct{}, keyAndValueFilters map[string]string, all bool) ([]engine.Instance, error) {
	args := filters.NewArgs()
	for k := range keyFilters {
		args.Add("label", k)
	}
	for k, v := range keyAndValueFilters {
		args.Add("label", fmt.Sprintf("%s=%s", k, v))
	}

	containers, err := e.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: args,
		All:     all,
	})
	if err != nil {
		return nil, err
	}

	var instances []engine.Instance
	for _, container := range containers {
		instances = append(instances, convertToInstance(container))
	}

	return instances, nil
}

func (e *Engine) StopContainer(ctx context.Context, id string) error {
	if err := e.client.ContainerStop(ctx, id, nil); err != nil {
		// TODO
		if strings.Contains(err.Error(), "No such container") {
			return engine.ErrInstanceNotFound
		}
		return engine.ErrInstanceNotFound
	}
	return nil
}

func (e *Engine) RemoveContainer(ctx context.Context, id string) error {
	if err := e.client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{}); err != nil {
		// TODO
		if strings.Contains(err.Error(), "No such container") {
			return engine.ErrInstanceNotFound
		}
		return engine.ErrInstanceNotFound
	}
	return nil
}

func (e *Engine) PullImage(ctx context.Context, image, registryAuth string, w io.Writer) error {
	processedRegistryAuth := ""
	if registryAuth != "" {
		var err error
		processedRegistryAuth, err = getProcessedRegistryAuth(registryAuth)
		if err != nil {
			return err
		}
	}

	out, err := e.client.ImagePull(ctx, image, types.ImagePullOptions{
		RegistryAuth: processedRegistryAuth,
	})
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(w, out)
	return err
}

func getProcessedRegistryAuth(registryAuth string) (string, error) {
	decodedRegistryAuth, err := base64.StdEncoding.DecodeString(registryAuth)
	if err != nil {
		return "", errors.Wrap(err, "invalid registry auth")
	}

	registryAuthParts := strings.SplitN(string(decodedRegistryAuth), ":", 2)
	if len(registryAuthParts) != 2 {
		return "", errors.New("invalid registry auth")
	}

	processedRegistryAuthBytes, err := json.Marshal(types.AuthConfig{
		Username: registryAuthParts[0],
		Password: registryAuthParts[1],
	})
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(processedRegistryAuthBytes), nil
}
