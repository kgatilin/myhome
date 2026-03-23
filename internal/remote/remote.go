package remote

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
)

// ExecFunc creates an *exec.Cmd for the given command and arguments.
type ExecFunc func(name string, args ...string) *exec.Cmd

// DefaultExec uses os/exec.Command.
func DefaultExec(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// SessionInfo describes a tmux session on a remote host.
type SessionInfo struct {
	Name    string
	Windows int
	Created string
}

// Run creates a tmux session on a remote host and runs a command inside it.
// Session name is derived from repo + prompt for uniqueness.
func Run(remote config.Remote, repo, prompt, authProfile string, execFn ExecFunc) (string, error) {
	if execFn == nil {
		execFn = DefaultExec
	}

	sessionName := sanitizeSessionName(repo + "-" + firstWords(prompt, 3))

	// Build the remote command: cd to repo dir, then run claude.
	repoDir := remote.Home + "/" + repo
	remoteCmd := fmt.Sprintf(
		"tmux new-session -d -s %s -c %s '%s'",
		sessionName,
		repoDir,
		buildClaudeCommand(prompt, authProfile, remote.Command),
	)

	cmd := execFn("ssh", remote.Host, remoteCmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ssh run on %s: %s: %w", remote.Host, strings.TrimSpace(stderr.String()), err)
	}

	return sessionName, nil
}

// List returns active tmux sessions on a remote host.
func List(remote config.Remote, execFn ExecFunc) ([]SessionInfo, error) {
	if execFn == nil {
		execFn = DefaultExec
	}

	cmd := execFn("ssh", remote.Host, "tmux list-sessions -F '#{session_name}|#{session_windows}|#{session_created_string}' 2>/dev/null || true")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ssh list on %s: %s: %w", remote.Host, strings.TrimSpace(stderr.String()), err)
	}

	var sessions []SessionInfo
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 1 {
			continue
		}
		s := SessionInfo{Name: parts[0]}
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &s.Windows)
		}
		if len(parts) > 2 {
			s.Created = parts[2]
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// Attach connects to a tmux session on a remote host interactively.
func Attach(remote config.Remote, session string, execFn ExecFunc) error {
	if execFn == nil {
		execFn = DefaultExec
	}

	cmd := execFn("ssh", "-t", remote.Host, "tmux attach-session -t "+session)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("attach to %s on %s: %w", session, remote.Host, err)
	}
	return nil
}

// Stop kills a tmux session on a remote host.
func Stop(remote config.Remote, session string, execFn ExecFunc) error {
	if execFn == nil {
		execFn = DefaultExec
	}

	cmd := execFn("ssh", remote.Host, "tmux kill-session -t "+session)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stop %s on %s: %s: %w", session, remote.Host, strings.TrimSpace(stderr.String()), err)
	}
	return nil
}

// sanitizeSessionName cleans a string for use as a tmux session name.
func sanitizeSessionName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
			b.WriteRune(c)
		case c == ' ', c == '/', c == '.':
			b.WriteRune('-')
		}
	}
	result := b.String()
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

// firstWords returns the first n words from a string.
func firstWords(s string, n int) string {
	words := strings.Fields(s)
	if len(words) > n {
		words = words[:n]
	}
	return strings.Join(words, "-")
}

// buildClaudeCommand constructs the claude CLI command for remote execution.
// command is the base command (e.g. "claude -p"); defaults to "claude -p" if empty.
func buildClaudeCommand(prompt, authProfile, command string) string {
	if command == "" {
		command = "claude -p"
	}
	cmd := fmt.Sprintf("%s %q", command, prompt)
	if authProfile != "" {
		cmd = fmt.Sprintf("CLAUDE_AUTH_PROFILE=%s %s", authProfile, cmd)
	}
	return cmd
}
