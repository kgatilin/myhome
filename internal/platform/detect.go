package platform

import (
	"fmt"
	"runtime"
)

// Detect returns the Platform implementation for the current OS.
func Detect() (Platform, error) {
	switch runtime.GOOS {
	case "darwin":
		return newDarwin(), nil
	case "linux":
		return newLinux(), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
