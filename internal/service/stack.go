package service

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/platform"
	"gopkg.in/yaml.v3"
)

// StartOption configures how services are started.
type StartOption func(*startOptions)

type startOptions struct {
	cfg *config.Config
}

// WithConfig provides the full config so agent services can be launched as containers.
func WithConfig(cfg *config.Config) StartOption {
	return func(o *startOptions) {
		o.cfg = cfg
	}
}

func resolveStartOptions(opts []StartOption) startOptions {
	var o startOptions
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// resolveServiceCommand returns the command args for a service entry. For agent
// services (name starts with "agent-") where a matching AgentConfig exists in
// the full config, it builds a container run command instead.
// Simple commands return a single-element slice; container commands return
// the full args slice.
func resolveServiceCommand(e Entry, o startOptions) ([]string, error) {
	if o.cfg == nil || !strings.HasPrefix(e.Name, "agent-") {
		return []string{e.Command}, nil
	}
	agentName := strings.TrimPrefix(e.Name, "agent-")
	agentCfg, ok := o.cfg.Agents[agentName]
	if !ok {
		return []string{e.Command}, nil
	}
	ctrCfg, ok := o.cfg.Containers[agentCfg.Container]
	if !ok {
		// Fall back to an empty container config — BuildAgentContainerCommand
		// will apply the default image.
		ctrCfg = config.Container{}
	}
	return BuildAgentContainerCommand(agentName, agentCfg, ctrCfg, o.cfg, e.Command)
}

// Entry is a flattened service with its resolved name and config.
type Entry struct {
	Name      string
	Command   string
	Restart   string
	DependsOn string
}

// Flatten extracts all services from ServicesConfig into an ordered list.
// Services are returned in dependency order: deskd first, then agents and adapters.
func Flatten(cfg config.ServicesConfig) []Entry {
	var entries []Entry

	if cfg.Deskd != nil {
		entries = append(entries, Entry{
			Name:    "deskd",
			Command: cfg.Deskd.Command,
			Restart: cfg.Deskd.Restart,
		})
	}

	for name, svc := range cfg.Agents {
		entries = append(entries, Entry{
			Name:      "agent-" + name,
			Command:   svc.Command,
			Restart:   svc.Restart,
			DependsOn: svc.DependsOn,
		})
	}

	for name, svc := range cfg.Adapters {
		entries = append(entries, Entry{
			Name:      "adapter-" + name,
			Command:   svc.Command,
			Restart:   svc.Restart,
			DependsOn: svc.DependsOn,
		})
	}

	return topoSort(entries)
}

// topoSort orders entries so dependencies come first.
func topoSort(entries []Entry) []Entry {
	byName := make(map[string]Entry, len(entries))
	for _, e := range entries {
		byName[e.Name] = e
	}

	visited := make(map[string]bool, len(entries))
	var sorted []Entry

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		e, ok := byName[name]
		if !ok {
			return
		}
		if e.DependsOn != "" {
			visit(e.DependsOn)
		}
		sorted = append(sorted, e)
	}

	for _, e := range entries {
		visit(e.Name)
	}
	return sorted
}

// StartAll installs and starts all services in dependency order.
// For agent services that have a matching entry in cfg.Agents, the service
// command is rewritten to a container run command with bus socket mount.
func StartAll(svcCfg config.ServicesConfig, plat platform.Platform, opts ...StartOption) error {
	o := resolveStartOptions(opts)

	// Ensure deskd agent state exists before starting agent services.
	if o.cfg != nil {
		for name, agentCfg := range o.cfg.Agents {
			if err := ensureAgentState(name, agentCfg); err != nil {
				return fmt.Errorf("ensure agent state %s: %w", name, err)
			}
		}
	}

	username := currentUsername()
	entries := Flatten(svcCfg)
	for _, e := range entries {
		args, err := resolveServiceCommand(e, o)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", e.Name, err)
		}
		if err := Install(e.Name, args, e.Restart, username, plat); err != nil {
			return fmt.Errorf("start %s: %w", e.Name, err)
		}
		fmt.Printf("started %s\n", e.Name)
	}
	return nil
}

// deskdAgentState is the YAML structure for ~/.deskd/agents/<name>.yaml.
type deskdAgentState struct {
	Name         string `yaml:"name"`
	SystemPrompt string `yaml:"system_prompt,omitempty"`
	WorkDir      string `yaml:"work_dir,omitempty"`
}

// ensureAgentState creates the deskd agent state file if it doesn't exist.
// The state file at ~/.deskd/agents/<name>.yaml is required by deskd agent run.
func ensureAgentState(name string, agentCfg config.AgentConfig) error {
	homeDir := currentHomeDir()
	if homeDir == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	stateDir := filepath.Join(homeDir, ".deskd", "agents")
	statePath := filepath.Join(stateDir, name+".yaml")

	if _, err := os.Stat(statePath); err == nil {
		return nil // already exists
	}

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	// Resolve work_dir: use first mount path if not otherwise determinable
	workDir := ""
	if len(agentCfg.Mounts) > 0 {
		mount := agentCfg.Mounts[0]
		mount = strings.TrimSuffix(mount, ":ro")
		parts := strings.SplitN(mount, ":", 2)
		workDir = expandHome(parts[0], homeDir)
		if !filepath.IsAbs(workDir) {
			workDir = filepath.Join(homeDir, workDir)
		}
	}

	state := deskdAgentState{
		Name:         name,
		SystemPrompt: agentCfg.SystemPrompt,
		WorkDir:      workDir,
	}

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal agent state: %w", err)
	}
	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		return fmt.Errorf("write agent state: %w", err)
	}

	fmt.Printf("created deskd agent state: %s\n", statePath)
	return nil
}

// StopAll stops all services in reverse dependency order.
func StopAll(cfg config.ServicesConfig, plat platform.Platform) error {
	entries := Flatten(cfg)
	// Reverse for stop order
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	for _, e := range entries {
		if err := plat.ServiceStop(e.Name); err != nil {
			fmt.Printf("warning: failed to stop %s: %v\n", e.Name, err)
			continue
		}
		fmt.Printf("stopped %s\n", e.Name)
	}
	return nil
}

// StartOne installs and starts a single named service.
func StartOne(name string, svcCfg config.ServicesConfig, plat platform.Platform, opts ...StartOption) error {
	entry, err := findEntry(name, svcCfg)
	if err != nil {
		return err
	}
	o := resolveStartOptions(opts)
	args, err := resolveServiceCommand(entry, o)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", entry.Name, err)
	}
	username := currentUsername()
	return Install(entry.Name, args, entry.Restart, username, plat)
}

// StopOne stops a single named service.
func StopOne(name string, cfg config.ServicesConfig, plat platform.Platform) error {
	entry, err := findEntry(name, cfg)
	if err != nil {
		return err
	}
	return plat.ServiceStop(entry.Name)
}

// StatusAll returns the running status of all configured services.
func StatusAll(cfg config.ServicesConfig, plat platform.Platform) ([]StatusInfo, error) {
	entries := Flatten(cfg)
	var results []StatusInfo
	for _, e := range entries {
		running, err := plat.ServiceStatus(e.Name)
		if err != nil {
			running = false
		}
		results = append(results, StatusInfo{
			Name:    e.Name,
			Running: running,
		})
	}
	return results, nil
}

// StatusInfo holds the status of a single service.
type StatusInfo struct {
	Name    string
	Running bool
}

// findEntry looks up a service by short name (e.g. "deskd", "dev", "github")
// and returns the resolved Entry.
func findEntry(name string, cfg config.ServicesConfig) (Entry, error) {
	entries := Flatten(cfg)
	// Try exact match first
	for _, e := range entries {
		if e.Name == name {
			return e, nil
		}
	}
	// Try short name (without prefix)
	for _, e := range entries {
		if e.Name == "agent-"+name || e.Name == "adapter-"+name {
			return e, nil
		}
	}
	return Entry{}, fmt.Errorf("unknown service: %s", name)
}

func currentUsername() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.Username
}
