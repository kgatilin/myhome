package platform

import (
	"os"
	"os/exec"
)

// cmdRunner provides common command execution for platform operations.
type cmdRunner struct{}

// run executes a command with stdout/stderr connected to the process output.
func (r cmdRunner) run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// output executes a command and returns its stdout.
func (r cmdRunner) output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}
