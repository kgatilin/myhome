package container

import (
	"fmt"
	"os/exec"
)

// runtimeOrder is the preferred detection order for container runtimes.
var runtimeOrder = []string{"nerdctl", "podman", "docker"}

// DetectRuntime resolves which container runtime to use.
// If preferred is "auto", it tries nerdctl, podman, docker in order.
// Otherwise it validates that the preferred runtime is available.
func DetectRuntime(preferred string) (string, error) {
	if preferred != "auto" && preferred != "" {
		path, err := exec.LookPath(preferred)
		if err != nil {
			return "", fmt.Errorf("preferred container runtime %q not found: %w", preferred, err)
		}
		return path, nil
	}

	for _, name := range runtimeOrder {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no container runtime found (tried %v)", runtimeOrder)
}
