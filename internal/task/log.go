package task

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// TailLog prints the contents of a log file.
// If follow is true, it uses "tail -f" to stream new output.
func TailLog(logFile string, follow bool) error {
	if follow {
		cmd := exec.Command("tail", "-f", logFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tailing log file: %w", err)
		}
		return nil
	}

	f, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(os.Stdout, f); err != nil {
		return fmt.Errorf("reading log file: %w", err)
	}
	return nil
}
