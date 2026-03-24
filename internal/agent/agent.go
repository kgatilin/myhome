// Package agent manages agent lifecycle: create, start, stop, restart, and state transitions.
// An agent represents a persistent Claude instance running in a Docker container.
package agent

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/container"
	"github.com/kgatilin/myhome/internal/vault"
)

// AgentStatus represents the lifecycle state of an agent.
type AgentStatus string

const (
	StatusCreating AgentStatus = "creating"
	StatusRunning  AgentStatus = "running"
	StatusStopped  AgentStatus = "stopped"
	StatusFailed   AgentStatus = "failed"
)

// State holds the runtime state of an agent, persisted to disk.
type State struct {
	Name         string      `yaml:"name"`
	ContainerID  string      `yaml:"container_id,omitempty"`
	SessionID    string      `yaml:"session_id,omitempty"`
	Status       AgentStatus `yaml:"status"`
	CreatedAt    time.Time   `yaml:"created_at"`
	Container    string      `yaml:"container"`      // container config name from myhome.yml
	GRPCPort     int         `yaml:"grpc_port,omitempty"`
	TotalCostUSD float64     `yaml:"total_cost_usd,omitempty"`
	NumTurns     int         `yaml:"num_turns,omitempty"`
	LogFile      string      `yaml:"log_file,omitempty"`
}

// ExecFunc creates an *exec.Cmd for the given command and arguments.
type ExecFunc func(name string, args ...string) *exec.Cmd

// Manager handles agent lifecycle operations.
type Manager struct {
	store   *Store
	execFn  ExecFunc
	runtime string
	homeDir string
	Vault   *vault.KDBXVault // optional: for resolving SSH keys and vault:// secrets
}

// NewManager creates a Manager with the given dependencies.
func NewManager(store *Store, execFn ExecFunc, runtime, homeDir string) *Manager {
	return &Manager{
		store:   store,
		execFn:  execFn,
		runtime: runtime,
		homeDir: homeDir,
	}
}

// Create starts a new agent container from config and persists its state.
func (m *Manager) Create(name string, agentCfg config.AgentConfig, cfg *config.Config) error {
	// Check if agent already exists
	if existing, err := m.store.Load(name); err == nil {
		if existing.Status == StatusRunning {
			return fmt.Errorf("agent %q is already running", name)
		}
		// Clean up stale state
		m.store.Remove(name)
	}

	// Resolve container config
	ctrCfg, ok := cfg.Containers[agentCfg.Container]
	if !ok {
		return fmt.Errorf("unknown container %q for agent %q", agentCfg.Container, name)
	}

	// Build container args
	containerName := fmt.Sprintf("myhome-agent-%s", name)
	logFile := filepath.Join(m.store.LogDir(), name+".log")

	args, err := m.buildContainerArgs(containerName, name, agentCfg, ctrCfg, cfg)
	if err != nil {
		return fmt.Errorf("building container args: %w", err)
	}

	// Create initial state
	state := &State{
		Name:      name,
		Status:    StatusCreating,
		CreatedAt: time.Now(),
		Container: agentCfg.Container,
		LogFile:   logFile,
	}
	if err := m.store.Save(state); err != nil {
		return fmt.Errorf("saving initial state: %w", err)
	}

	// Start container
	cmd := m.execFn(m.runtime, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		state.Status = StatusFailed
		m.store.Save(state)
		return fmt.Errorf("starting agent container: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	containerID := strings.TrimSpace(stdout.String())
	state.ContainerID = containerID
	state.Status = StatusRunning

	if err := m.store.Save(state); err != nil {
		return fmt.Errorf("saving running state: %w", err)
	}

	// Start log streaming
	if err := m.startLogStream(containerID, logFile); err != nil {
		return fmt.Errorf("setting up log stream: %w", err)
	}

	return nil
}

// Stop gracefully stops an agent's container.
func (m *Manager) Stop(name string) error {
	state, err := m.store.Load(name)
	if err != nil {
		return err
	}
	if state.Status != StatusRunning {
		return fmt.Errorf("agent %q is not running (status: %s)", name, state.Status)
	}
	if state.ContainerID == "" {
		return fmt.Errorf("agent %q has no container ID", name)
	}

	if err := m.stopContainer(state.ContainerID); err != nil {
		// Container might already be stopped — check and update state
		state.Status = StatusStopped
		m.store.Save(state)
		return fmt.Errorf("stopping agent container: %w", err)
	}

	state.Status = StatusStopped
	return m.store.Save(state)
}

// Restart stops and starts an agent, preserving session for continuity.
func (m *Manager) Restart(name string, agentCfg config.AgentConfig, cfg *config.Config) error {
	state, err := m.store.Load(name)
	if err != nil {
		return err
	}

	// Stop if running
	if state.Status == StatusRunning && state.ContainerID != "" {
		m.stopContainer(state.ContainerID)
		m.rmContainer(state.ContainerID)
	}

	// Save session ID for continuity
	sessionID := state.SessionID

	// Remove old state and create fresh
	m.store.Remove(name)

	if err := m.Create(name, agentCfg, cfg); err != nil {
		return err
	}

	// Restore session ID if present
	if sessionID != "" {
		state, _ = m.store.Load(name)
		state.SessionID = sessionID
		m.store.Save(state)
	}

	return nil
}

// Remove stops an agent (if running) and removes its state.
func (m *Manager) Remove(name string) error {
	state, err := m.store.Load(name)
	if err != nil {
		return err
	}

	// Stop container if running
	if state.Status == StatusRunning && state.ContainerID != "" {
		m.stopContainer(state.ContainerID)
		m.rmContainer(state.ContainerID)
	}

	return m.store.Remove(name)
}

// stopContainer stops a container using the configured runtime and exec function.
func (m *Manager) stopContainer(containerID string) error {
	cmd := m.execFn(m.runtime, "stop", containerID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stop container %s: %s: %w", containerID, strings.TrimSpace(stderr.String()), err)
	}
	return nil
}

// rmContainer removes a stopped container.
func (m *Manager) rmContainer(containerID string) error {
	cmd := m.execFn(m.runtime, "rm", containerID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove container %s: %s: %w", containerID, strings.TrimSpace(stderr.String()), err)
	}
	return nil
}

// Send sends a message to a running agent and returns the response.
// It uses docker exec to run a command inside the agent's container.
func (m *Manager) Send(name, message string) (string, error) {
	state, err := m.store.Load(name)
	if err != nil {
		return "", err
	}
	if state.Status != StatusRunning {
		return "", fmt.Errorf("agent %q is not running (status: %s)", name, state.Status)
	}

	// Send message via docker exec — run claude with the prompt inside the container
	cmd := m.execFn(m.runtime, "exec", state.ContainerID,
		"claude", "--output-format", "text", "-p", message)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("sending message to agent %q: %s: %w", name, strings.TrimSpace(stderr.String()), err)
	}

	// Update turn count
	state.NumTurns++
	m.store.Save(state)

	return strings.TrimSpace(stdout.String()), nil
}

// ContainerStatus checks the actual container status from the runtime.
func (m *Manager) ContainerStatus(containerID string) (string, error) {
	cmd := m.execFn(m.runtime, "inspect", "--format", "{{.State.Status}}", containerID)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "unknown", nil
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RefreshStatus syncs persisted state with actual container status.
func (m *Manager) RefreshStatus(name string) (*State, error) {
	state, err := m.store.Load(name)
	if err != nil {
		return nil, err
	}
	if state.ContainerID == "" {
		return state, nil
	}

	actual, _ := m.ContainerStatus(state.ContainerID)
	switch actual {
	case "running":
		if state.Status != StatusRunning {
			state.Status = StatusRunning
			m.store.Save(state)
		}
	case "exited", "dead", "unknown":
		if state.Status == StatusRunning {
			state.Status = StatusStopped
			m.store.Save(state)
		}
	}
	return state, nil
}

// buildContainerArgs constructs the docker run arguments for an agent container.
func (m *Manager) buildContainerArgs(containerName, agentName string, agentCfg config.AgentConfig, ctrCfg config.Container, cfg *config.Config) ([]string, error) {
	args := []string{"run", "-d", "--name", containerName}

	// Firewall
	if ctrCfg.Firewall {
		args = append(args, "--network", "none")
	}

	// Container config mounts
	for _, mount := range container.ResolveMounts(ctrCfg.Mounts, m.homeDir) {
		args = append(args, "-v", mount)
	}

	// Agent-specific mounts (workspace paths like life/family:/workspace)
	for _, mount := range agentCfg.Mounts {
		parts := strings.SplitN(mount, ":", 2)
		hostPath := parts[0]
		if !filepath.IsAbs(hostPath) {
			hostPath = filepath.Join(m.homeDir, hostPath)
		}
		containerPath := hostPath
		if len(parts) > 1 {
			containerPath = parts[1]
		}
		readOnly := false
		if strings.HasSuffix(containerPath, ":ro") {
			readOnly = true
			containerPath = strings.TrimSuffix(containerPath, ":ro")
		}
		flag := hostPath + ":" + containerPath
		if readOnly {
			flag += ":ro"
		}
		args = append(args, "-v", flag)
	}

	// Container home dir
	containerHome := ctrCfg.HomeDir
	if containerHome == "" {
		containerHome = "/home/node"
	}

	// Mount Claude config + auth
	claudeConfigDir := "~/.claude"
	if cfg.Claude.ConfigDir != "" {
		claudeConfigDir = cfg.Claude.ConfigDir
	}
	resolvedConfigDir := expandHome(claudeConfigDir, m.homeDir)
	args = append(args,
		"-v", resolvedConfigDir+":"+containerHome+"/.claude",
		"-e", "CLAUDE_CONFIG_DIR="+containerHome+"/.claude",
	)

	// SSH key injection: tmpfs mount + key as base64 env var (decoded in startup preamble)
	var sshKeyB64 string
	if agentCfg.Identity.SSH != "" {
		keyData, err := m.extractSSHKey(agentCfg.Identity.SSH)
		if err != nil {
			return nil, fmt.Errorf("extracting SSH key %q: %w", agentCfg.Identity.SSH, err)
		}
		sshKeyB64 = base64.StdEncoding.EncodeToString(keyData)
		args = append(args, "--tmpfs", "/run/agent-ssh:size=1m,mode=0700")
		args = append(args, "-e", "_AGENT_SSH_KEY_B64="+sshKeyB64)
		args = append(args, "-e", "GIT_SSH_COMMAND=ssh -i /run/agent-ssh/id_ed25519 -o StrictHostKeyChecking=no")
	}

	// Vault secrets injection: resolve vault:// refs to env vars
	for _, ref := range agentCfg.Secrets.Vault {
		envName, envVal, err := m.resolveSecretRef(ref)
		if err != nil {
			return nil, fmt.Errorf("resolving secret %q: %w", ref, err)
		}
		args = append(args, "-e", envName+"="+envVal)
	}

	// Container env vars
	for k, v := range ctrCfg.Env {
		resolved := resolveEnvValue(v, m.execFn)
		if resolved != "" {
			args = append(args, "-e", k+"="+resolved)
		}
	}

	// Agent-specific env vars
	for k, v := range agentCfg.Env {
		args = append(args, "-e", k+"="+v)
	}

	// Model override
	if agentCfg.Model != "" {
		args = append(args, "-e", "CLAUDE_MODEL="+agentCfg.Model)
	}

	// Git identity
	if agentCfg.Identity.Git.Name != "" {
		args = append(args, "-e", "GIT_AUTHOR_NAME="+agentCfg.Identity.Git.Name)
		args = append(args, "-e", "GIT_COMMITTER_NAME="+agentCfg.Identity.Git.Name)
	}
	if agentCfg.Identity.Git.Email != "" {
		args = append(args, "-e", "GIT_AUTHOR_EMAIL="+agentCfg.Identity.Git.Email)
		args = append(args, "-e", "GIT_COMMITTER_EMAIL="+agentCfg.Identity.Git.Email)
	}

	// Image
	args = append(args, ctrCfg.Image)

	// Startup commands — render with system prompt as the prompt
	if len(ctrCfg.StartupCommands) > 0 {
		prompt := agentCfg.SystemPrompt
		if prompt == "" {
			prompt = fmt.Sprintf("You are the %s agent.", agentName)
		}

		var scriptParts []string

		// SSH key setup preamble: decode key from env to tmpfs, set permissions
		if sshKeyB64 != "" {
			scriptParts = append(scriptParts,
				`echo "$_AGENT_SSH_KEY_B64" | base64 -d > /run/agent-ssh/id_ed25519 && chmod 600 /run/agent-ssh/id_ed25519`)
		}

		for _, sc := range ctrCfg.StartupCommands {
			scriptParts = append(scriptParts, renderStartupCommand(sc, prompt))
		}
		script := strings.Join(scriptParts, " ; ")
		args = append(args, "/bin/bash", "-c", script)
	}

	return args, nil
}

// extractSSHKey reads an SSH private key from the vault.
// The key name maps to a vault entry "SSH Keys/<name>" with an attachment named <name>.
func (m *Manager) extractSSHKey(keyName string) ([]byte, error) {
	if m.Vault == nil {
		return nil, fmt.Errorf("vault required for SSH key injection but not provided")
	}
	entryName := "SSH Keys/" + keyName
	return m.Vault.GetAttachment(entryName, keyName)
}

// resolveSecretRef resolves a vault:// reference to an env var name and value.
// "vault://github-pat-personal" → env name "GITHUB_PAT_PERSONAL", value from vault.
func (m *Manager) resolveSecretRef(ref string) (string, string, error) {
	if m.Vault == nil {
		return "", "", fmt.Errorf("vault required for secret %q but not provided", ref)
	}
	val, err := vault.ResolveVaultRef(ref, m.Vault)
	if err != nil {
		return "", "", err
	}
	// Derive env var name: strip vault://, uppercase, replace - with _
	name := strings.TrimPrefix(ref, "vault://")
	name = strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	return name, val, nil
}

// renderStartupCommand replaces {{.Prompt}} with shell-quoted prompt.
func renderStartupCommand(cmd, prompt string) string {
	quoted := "'" + strings.ReplaceAll(prompt, "'", "'\"'\"'") + "'"
	return strings.ReplaceAll(cmd, "{{.Prompt}}", quoted)
}

// expandHome replaces ~ with the actual home directory.
func expandHome(path, homeDir string) string {
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return homeDir + path[1:]
	}
	return path
}

// resolveEnvValue evaluates a container env value. If wrapped in $(...), runs as shell command.
func resolveEnvValue(val string, execFn ExecFunc) string {
	if strings.HasPrefix(val, "$(") && strings.HasSuffix(val, ")") {
		shellCmd := val[2 : len(val)-1]
		cmd := execFn("sh", "-c", shellCmd)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			return ""
		}
		return strings.TrimSpace(stdout.String())
	}
	return val
}

// startLogStream pipes container logs to a file in the background.
func (m *Manager) startLogStream(containerID, logFile string) error {
	f, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("creating log file: %w", err)
	}

	cmd := m.execFn(m.runtime, "logs", "-f", containerID)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("starting log stream: %w", err)
	}

	go func() {
		cmd.Wait()
		f.Close()
	}()

	return nil
}
