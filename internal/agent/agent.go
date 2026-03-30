// Package agent manages agent lifecycle: create, start, stop, restart, and state transitions.
// An agent represents a persistent Claude instance running in a Docker container.
package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kgatilin/myhome/internal/config"
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

// AgentRuntime holds runtime/session state for an agent.
type AgentRuntime struct {
	GRPCPort     int     `yaml:"grpc_port,omitempty"`
	TotalCostUSD float64 `yaml:"total_cost_usd,omitempty"`
	NumTurns     int     `yaml:"num_turns,omitempty"`
	LogFile      string  `yaml:"log_file,omitempty"`
	Model        string  `yaml:"model,omitempty"`
	SystemPrompt string  `yaml:"system_prompt,omitempty"`
	WorkDir      string  `yaml:"work_dir,omitempty"`
}

// State holds the runtime state of an agent, persisted to disk.
type State struct {
	Name         string      `yaml:"name"`
	ContainerID  string      `yaml:"container_id,omitempty"`
	SessionID    string      `yaml:"session_id,omitempty"`
	Status       AgentStatus `yaml:"status"`
	CreatedAt    time.Time   `yaml:"created_at"`
	Container    string      `yaml:"container"` // container config name from myhome.yml
	AgentRuntime `yaml:",inline"`
}

// ExecFunc creates an *exec.Cmd for the given command and arguments.
type ExecFunc func(name string, args ...string) *exec.Cmd

// Manager handles agent lifecycle operations.
type Manager struct {
	store *Store
	ctr   containerOps
	Vault vault.Reader // optional: for resolving SSH keys and vault:// secrets
}

// NewManager creates a Manager with the given dependencies.
func NewManager(store *Store, execFn ExecFunc, runtime, homeDir string) *Manager {
	return &Manager{
		store: store,
		ctr: containerOps{
			execFn:  execFn,
			runtime: runtime,
			homeDir: homeDir,
		},
	}
}

// Create starts a new agent container from config and persists its state.
func (m *Manager) Create(name string, agentCfg config.AgentConfig, cfg *config.Config) error {
	if existing, err := m.store.Load(name); err == nil {
		if existing.Status == StatusRunning {
			return fmt.Errorf("agent %q is already running", name)
		}
		m.store.Remove(name)
	}

	ctrCfg, ok := cfg.Containers[agentCfg.Container]
	if !ok {
		return fmt.Errorf("unknown container %q for agent %q", agentCfg.Container, name)
	}

	containerName := fmt.Sprintf("myhome-agent-%s", name)
	logFile := filepath.Join(m.store.LogDir(), name+".log")

	// Set vault on container ops for SSH key and secret resolution
	m.ctr.vault = m.Vault
	args, err := m.ctr.buildContainerArgs(containerName, agentCfg, ctrCfg, cfg)
	if err != nil {
		return fmt.Errorf("building container args: %w", err)
	}

	// Resolve work dir from first mount's container path
	var workDir string
	if len(agentCfg.Mounts) > 0 {
		m0 := strings.TrimSuffix(agentCfg.Mounts[0], ":ro")
		parts := strings.SplitN(m0, ":", 2)
		hostPath := expandHome(parts[0], m.ctr.homeDir)
		if !filepath.IsAbs(hostPath) {
			hostPath = filepath.Join(m.ctr.homeDir, hostPath)
		}
		workDir = hostPath
		if len(parts) > 1 {
			workDir = expandHome(parts[1], m.ctr.homeDir)
		}
	}

	state := &State{
		Name:      name,
		Status:    StatusCreating,
		CreatedAt: time.Now(),
		Container: agentCfg.Container,
		AgentRuntime: AgentRuntime{
			LogFile:      logFile,
			Model:        agentCfg.Model,
			SystemPrompt: agentCfg.SystemPrompt,
			WorkDir:      workDir,
		},
	}
	if err := m.store.Save(state); err != nil {
		return fmt.Errorf("saving initial state: %w", err)
	}

	// Start container
	cmd := m.ctr.execFn(m.ctr.runtime, args...)
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

	if err := m.ctr.startLogStream(containerID, logFile); err != nil {
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

	if err := m.ctr.stopContainer(state.ContainerID); err != nil {
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

	if state.Status == StatusRunning && state.ContainerID != "" {
		m.ctr.stopContainer(state.ContainerID)
		m.ctr.rmContainer(state.ContainerID)
	}

	sessionID := state.SessionID
	m.store.Remove(name)

	if err := m.Create(name, agentCfg, cfg); err != nil {
		return err
	}

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

	if state.Status == StatusRunning && state.ContainerID != "" {
		m.ctr.stopContainer(state.ContainerID)
		m.ctr.rmContainer(state.ContainerID)
	}

	return m.store.Remove(name)
}

// SendOptions holds optional parameters for Send.
type SendOptions struct {
	MaxTurns int // passed through to Claude CLI --max-turns
}

// Send sends a message to a running agent and returns the response.
func (m *Manager) Send(name, message string, opts *SendOptions) (string, error) {
	state, err := m.store.Load(name)
	if err != nil {
		return "", err
	}
	if state.Status != StatusRunning {
		return "", fmt.Errorf("agent %q is not running (status: %s)", name, state.Status)
	}

	claudeArgs := []string{"exec"}
	if state.WorkDir != "" {
		claudeArgs = append(claudeArgs, "-w", state.WorkDir)
	}
	claudeArgs = append(claudeArgs, state.ContainerID,
		"claude", "--dangerously-skip-permissions", "--output-format", "stream-json", "--verbose")
	if state.Model != "" {
		claudeArgs = append(claudeArgs, "--model", state.Model)
	}
	if opts != nil && opts.MaxTurns > 0 {
		claudeArgs = append(claudeArgs, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	if state.SessionID != "" {
		claudeArgs = append(claudeArgs, "--resume", state.SessionID)
	} else if state.SystemPrompt != "" {
		claudeArgs = append(claudeArgs, "--system-prompt", state.SystemPrompt)
	}
	claudeArgs = append(claudeArgs, "-p", message)

	cmd := m.ctr.execFn(m.ctr.runtime, claudeArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("sending message to agent %q: %s: %w", name, strings.TrimSpace(stderr.String()), err)
	}

	var resultText string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		msgType, _ := msg["type"].(string)
		if state.SessionID == "" {
			if sid, ok := msg["session_id"].(string); ok && sid != "" {
				state.SessionID = sid
			}
		}
		if msgType == "result" {
			if r, ok := msg["result"].(string); ok {
				resultText = r
			}
			if cost, ok := msg["total_cost_usd"].(float64); ok {
				state.TotalCostUSD += cost
			}
		}
	}

	state.NumTurns++
	m.store.Save(state)

	return resultText, nil
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

	actual, _ := m.ctr.containerStatus(state.ContainerID)
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
