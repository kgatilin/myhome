package service

import (
	"fmt"
	"os/user"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/platform"
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

// resolveServiceCommand returns the command for a service entry. For agent
// services (name starts with "agent-") where a matching AgentConfig exists in
// the full config, it builds a container run command instead.
func resolveServiceCommand(e Entry, o startOptions) (string, error) {
	if o.cfg == nil || !strings.HasPrefix(e.Name, "agent-") {
		return e.Command, nil
	}
	agentName := strings.TrimPrefix(e.Name, "agent-")
	agentCfg, ok := o.cfg.Agents[agentName]
	if !ok {
		return e.Command, nil
	}
	ctrCfg, ok := o.cfg.Containers[agentCfg.Container]
	if !ok {
		return "", fmt.Errorf("container %q referenced by agent %q not found", agentCfg.Container, agentName)
	}
	return BuildAgentContainerCommand(agentName, agentCfg, ctrCfg, o.cfg)
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
	username := currentUsername()
	entries := Flatten(svcCfg)
	for _, e := range entries {
		command, err := resolveServiceCommand(e, o)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", e.Name, err)
		}
		sc := config.ServiceConfig{Command: command, Restart: e.Restart}
		if err := Install(e.Name, sc, username, plat); err != nil {
			return fmt.Errorf("start %s: %w", e.Name, err)
		}
		fmt.Printf("started %s\n", e.Name)
	}
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
	command, err := resolveServiceCommand(entry, o)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", entry.Name, err)
	}
	username := currentUsername()
	sc := config.ServiceConfig{Command: command, Restart: entry.Restart}
	return Install(entry.Name, sc, username, plat)
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
