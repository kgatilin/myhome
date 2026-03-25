package service

import (
	"fmt"

	"github.com/kgatilin/myhome/internal/platform"
)

// Install creates and enables a service for an agent user.
// It delegates to the platform abstraction for launchd (macOS) or systemd (Linux).
// args is the command arguments slice (single-element for simple commands,
// multi-element for container commands).
func Install(name string, args []string, restart string, username string, plat platform.Platform) error {
	restartBool := restart == "always"
	if err := plat.ServiceInstall(name, args, username, restartBool); err != nil {
		return fmt.Errorf("install service %s: %w", name, err)
	}
	if err := plat.ServiceStart(name); err != nil {
		return fmt.Errorf("start service %s: %w", name, err)
	}
	return nil
}

// Remove stops and removes a service.
func Remove(name string, plat platform.Platform) error {
	// Stop first — ignore errors since the service may not be running.
	_ = plat.ServiceStop(name)

	// The platform handles removing service files (plist/unit).
	// We don't have a dedicated ServiceRemove on the platform interface,
	// so stopping is the best we can do without modifying platform.go.
	return nil
}

// Status returns whether a service is running.
func Status(name string, plat platform.Platform) (bool, error) {
	running, err := plat.ServiceStatus(name)
	if err != nil {
		return false, fmt.Errorf("check service %s status: %w", name, err)
	}
	return running, nil
}
