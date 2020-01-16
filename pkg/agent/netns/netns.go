package netns

import (
	"context"
	"fmt"
	"runtime"

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
		fmt.Println("ORIGINAL NAMEPACE----", originalNamespace)
		err := netns.Set(originalNamespace)
		fmt.Println("switching to original namespace:::", err)
		fmt.Println("original namespace")
		fmt.Println(originalNamespace.String())

		err = netns.Set(originalNamespace)
		if err != nil {
			fmt.Println("succeeded on second attempt at switching!")
			fmt.Println("original namespace")
			fmt.Println(originalNamespace.String())
			return
		}

		fmt.Println("second attempt at switching to original namespace:::", err)
		fmt.Println("original namespace")
		fmt.Println(originalNamespace.String())
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
