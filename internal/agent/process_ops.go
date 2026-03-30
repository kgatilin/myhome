package agent

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// processOps handles low-level process operations for process-mode agents.
type processOps struct {
	execFn  ExecFunc
	homeDir string
}

// startProcess launches claude as a background process and returns its PID.
func (p *processOps) startProcess(prompt, logFile, workDir, model, systemPrompt string, env map[string]string) (int, error) {
	claudeArgs := []string{"--dangerously-skip-permissions", "--output-format", "stream-json", "--verbose"}
	if model != "" {
		claudeArgs = append(claudeArgs, "--model", model)
	}
	if systemPrompt != "" {
		claudeArgs = append(claudeArgs, "--system-prompt", systemPrompt)
	}
	claudeArgs = append(claudeArgs, "-p", prompt)

	cmd := p.execFn("claude", claudeArgs...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Set up env vars
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	// Redirect output to log file
	f, err := os.Create(logFile)
	if err != nil {
		return 0, fmt.Errorf("creating log file: %w", err)
	}
	cmd.Stdout = f
	cmd.Stderr = f

	// Detach from parent process group so the child survives
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		f.Close()
		return 0, fmt.Errorf("starting claude process: %w", err)
	}

	pid := cmd.Process.Pid

	// Release the process so it runs independently
	go func() {
		cmd.Wait()
		f.Close()
	}()

	return pid, nil
}

// sendMessage runs a new claude process with a message and returns the output.
// For process agents, each send is a separate claude invocation with --resume.
func (p *processOps) sendMessage(message, workDir, model, sessionID, systemPrompt string, env map[string]string) (string, error) {
	claudeArgs := []string{"--dangerously-skip-permissions", "--output-format", "stream-json", "--verbose"}
	if model != "" {
		claudeArgs = append(claudeArgs, "--model", model)
	}
	if sessionID != "" {
		claudeArgs = append(claudeArgs, "--resume", sessionID)
	} else if systemPrompt != "" {
		claudeArgs = append(claudeArgs, "--system-prompt", systemPrompt)
	}
	claudeArgs = append(claudeArgs, "-p", message)

	cmd := p.execFn("claude", claudeArgs...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude process: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	return stdout.String(), nil
}

// killProcess sends SIGTERM to a process by PID.
func (p *processOps) killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process %d: %w", pid, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("killing process %d: %w", pid, err)
	}
	return nil
}

// isProcessRunning checks if a process with the given PID is alive.
func (p *processOps) isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks if process exists without sending a signal
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// processStatus returns a human-readable status string for a PID.
func (p *processOps) processStatus(pid int) string {
	if pid <= 0 {
		return "unknown"
	}
	// Read /proc/<pid>/stat for process state on Linux
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "exited"
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return "unknown"
	}
	switch fields[2] {
	case "R":
		return "running"
	case "S":
		return "running" // sleeping (waiting for I/O) counts as running
	case "Z":
		return "zombie"
	case "T":
		return "stopped"
	default:
		return "running"
	}
}
