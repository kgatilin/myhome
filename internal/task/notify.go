package task

import (
	"fmt"
	"os/exec"
	"runtime"
)

// SendNotification sends a desktop notification about task completion.
// On macOS, uses osascript. No-op on other platforms.
func SendNotification(title, message string) {
	if runtime.GOOS != "darwin" {
		return
	}
	script := fmt.Sprintf(`display notification %q with title %q`, message, title)
	// Best effort — ignore errors (e.g. no GUI session)
	exec.Command("osascript", "-e", script).Run() //nolint:errcheck
}
