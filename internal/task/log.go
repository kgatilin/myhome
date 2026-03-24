package task

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// TailLog prints the contents of a log file.
// If follow is true, it uses "tail -f" to stream new output.
// If format is true, NDJSON lines are formatted for readability.
func TailLog(logFile string, follow, format bool) error {
	if follow {
		return tailFollow(logFile, format)
	}

	f, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	if format {
		formatter := NewLogFormatter(os.Stdout)
		formatter.Process(f)
		return nil
	}

	if _, err := io.Copy(os.Stdout, f); err != nil {
		return fmt.Errorf("reading log file: %w", err)
	}
	return nil
}

// tailFollow streams log output using "tail -f". When format is true,
// stdout from tail is piped through LogFormatter for readable output.
func tailFollow(logFile string, format bool) error {
	cmd := exec.Command("tail", "-f", logFile)
	cmd.Stderr = os.Stderr

	if !format {
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tailing log file: %w", err)
		}
		return nil
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting tail: %w", err)
	}

	formatter := NewLogFormatter(os.Stdout)
	formatter.Process(stdout)

	// Process returns when pipe closes (tail exits)
	return cmd.Wait()
}
