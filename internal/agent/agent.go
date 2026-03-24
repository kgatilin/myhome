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

// AgentMode represents how an agent runs: in a container or as a local process.
type AgentMode string

const (
	ModeContainer AgentMode = "container"
	ModeProcess   AgentMode = "process"
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
	Mode         AgentMode   `yaml:"mode,omitempty"`
	ContainerID  string      `yaml:"container_id,omitempty"`
	PID          int         `yaml:"pid,omitempty"`
	SessionID    string      `yaml:"session_id,omitempty"`
	Status       AgentStatus `yaml:"status"`
	CreatedAt    time.Time   `yaml:"created_at"`
	Container    string      `yaml:"container,omitempty"` // container config name from myhome.yml
	AgentRuntime `yaml:",inline"`
}

// ExecFunc creates an *exec.Cmd for the given command and arguments.
type ExecFunc func(name string, args ...string) *exec.Cmd

// Manager handles agent lifecycle operations.
type Manager struct {
	store *Store
	ctr   containerOps
	proc  processOps
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
		proc: processOps{
			execFn:  execFn,
			homeDir: homeDir,
		},
	}
}

// CreateOpts holds options for agent creation that aren't part of config.
type CreateOpts struct {
	Mode    AgentMode
	Prompt  string // initial prompt for process-mode agents
	WorkDir string // override work dir (for process mode without mounts config)
}

// Create starts a new agent from config and persists its state.
// For container mode, it starts a Docker container. For process mode, it spawns claude as a background process.
func (m *Manager) Create(name string, agentCfg config.AgentConfig, cfg *config.Config, opts ...CreateOpts) error {
	var opt CreateOpts
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.Mode == "" {
		opt.Mode = ModeContainer
	}

	if opt.Mode == ModeProcess {
		return m.createProcess(name, agentCfg, opt)
	}
	return m.createContainer(name, agentCfg, cfg)
}

// createContainer starts a new agent in a Docker container.
func (m *Manager) createContainer(name string, agentCfg config.AgentConfig, cfg *config.Config) error {
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
		Mode:      ModeContainer,
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

// createProcess starts a new agent as a local claude process.
func (m *Manager) createProcess(name string, agentCfg config.AgentConfig, opt CreateOpts) error {
	if existing, err := m.store.Load(name); err == nil {
		if existing.Status == StatusRunning {
			return fmt.Errorf("agent %q is already running", name)
		}
		m.store.Remove(name)
	}

	logFile := filepath.Join(m.store.LogDir(), name+".log")
	workDir := opt.WorkDir
	if workDir == "" && len(agentCfg.Mounts) > 0 {
		// Use first mount's host path as work dir
		m0 := strings.TrimSuffix(agentCfg.Mounts[0], ":ro")
		parts := strings.SplitN(m0, ":", 2)
		workDir = expandHome(parts[0], m.proc.homeDir)
		if !filepath.IsAbs(workDir) {
			workDir = filepath.Join(m.proc.homeDir, workDir)
		}
	}

	prompt := opt.Prompt
	if prompt == "" {
		prompt = "You are agent " + name + ". Wait for instructions."
	}

	state := &State{
		Name:      name,
		Mode:      ModeProcess,
		Status:    StatusCreating,
		CreatedAt: time.Now(),
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

	pid, err := m.proc.startProcess(prompt, logFile, workDir, agentCfg.Model, agentCfg.SystemPrompt, agentCfg.Env)
	if err != nil {
		state.Status = StatusFailed
		m.store.Save(state)
		return fmt.Errorf("starting agent process: %w", err)
	}

	state.PID = pid
	state.Status = StatusRunning
	if err := m.store.Save(state); err != nil {
		return fmt.Errorf("saving running state: %w", err)
	}

	return nil
}

// Stop gracefully stops an agent.
func (m *Manager) Stop(name string) error {
	state, err := m.store.Load(name)
	if err != nil {
		return err
	}
	if state.Status != StatusRunning {
		return fmt.Errorf("agent %q is not running (status: %s)", name, state.Status)
	}

	if state.Mode == ModeProcess {
		if state.PID <= 0 {
			return fmt.Errorf("agent %q has no PID", name)
		}
		if err := m.proc.killProcess(state.PID); err != nil {
			state.Status = StatusStopped
			m.store.Save(state)
			return fmt.Errorf("stopping agent process: %w", err)
		}
	} else {
		if state.ContainerID == "" {
			return fmt.Errorf("agent %q has no container ID", name)
		}
		if err := m.ctr.stopContainer(state.ContainerID); err != nil {
			state.Status = StatusStopped
			m.store.Save(state)
			return fmt.Errorf("stopping agent container: %w", err)
		}
	}

	state.Status = StatusStopped
	return m.store.Save(state)
}

// Restart stops and starts an agent, preserving session for continuity.
func (m *Manager) Restart(name string, agentCfg config.AgentConfig, cfg *config.Config, opts ...CreateOpts) error {
	state, err := m.store.Load(name)
	if err != nil {
		return err
	}

	if state.Status == StatusRunning {
		if state.Mode == ModeProcess {
			if state.PID > 0 {
				m.proc.killProcess(state.PID)
			}
		} else if state.ContainerID != "" {
			m.ctr.stopContainer(state.ContainerID)
			m.ctr.rmContainer(state.ContainerID)
		}
	}

	sessionID := state.SessionID
	mode := state.Mode
	m.store.Remove(name)

	// Preserve mode from previous state if not specified in opts
	if len(opts) == 0 && mode == ModeProcess {
		opts = []CreateOpts{{Mode: ModeProcess}}
	}

	if err := m.Create(name, agentCfg, cfg, opts...); err != nil {
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

	if state.Status == StatusRunning {
		if state.Mode == ModeProcess {
			if state.PID > 0 {
				m.proc.killProcess(state.PID)
			}
		} else if state.ContainerID != "" {
			m.ctr.stopContainer(state.ContainerID)
			m.ctr.rmContainer(state.ContainerID)
		}
	}

	return m.store.Remove(name)
}

// Send sends a message to a running agent and returns the response.
func (m *Manager) Send(name, message string) (string, error) {
	state, err := m.store.Load(name)
	if err != nil {
		return "", err
	}
	if state.Status != StatusRunning {
		return "", fmt.Errorf("agent %q is not running (status: %s)", name, state.Status)
	}

	var rawOutput string

	if state.Mode == ModeProcess {
		out, err := m.proc.sendMessage(message, state.WorkDir, state.Model, state.SessionID, state.SystemPrompt, nil)
		if err != nil {
			return "", fmt.Errorf("sending message to agent %q: %w", name, err)
		}
		rawOutput = out
	} else {
		claudeArgs := []string{"exec"}
		if state.WorkDir != "" {
			claudeArgs = append(claudeArgs, "-w", state.WorkDir)
		}
		claudeArgs = append(claudeArgs, state.ContainerID,
			"claude", "--dangerously-skip-permissions", "--output-format", "stream-json", "--verbose")
		if state.Model != "" {
			claudeArgs = append(claudeArgs, "--model", state.Model)
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
		rawOutput = stdout.String()
	}

	resultText := parseStreamJSON(rawOutput, state)

	state.NumTurns++
	m.store.Save(state)

	return resultText, nil
}

// parseStreamJSON extracts the result text from stream-json output and captures session ID.
func parseStreamJSON(output string, state *State) string {
	var resultText string
	for _, line := range strings.Split(output, "\n") {
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
		}
	}
	return resultText
}

// RefreshStatus syncs persisted state with actual runtime status.
func (m *Manager) RefreshStatus(name string) (*State, error) {
	state, err := m.store.Load(name)
	if err != nil {
		return nil, err
	}

	if state.Mode == ModeProcess {
		if state.PID <= 0 {
			return state, nil
		}
		if m.proc.isProcessRunning(state.PID) {
			if state.Status != StatusRunning {
				state.Status = StatusRunning
				m.store.Save(state)
			}
		} else {
			if state.Status == StatusRunning {
				state.Status = StatusStopped
				m.store.Save(state)
			}
		}
		return state, nil
	}

	// Container mode
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
