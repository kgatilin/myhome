package service

import (
	"fmt"
	"os/user"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/platform"
)

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
func StartAll(cfg config.ServicesConfig, plat platform.Platform) error {
	username := currentUsername()
	entries := Flatten(cfg)
	for _, e := range entries {
		svcCfg := config.ServiceConfig{Command: e.Command, Restart: e.Restart}
		if err := Install(e.Name, svcCfg, username, plat); err != nil {
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
func StartOne(name string, cfg config.ServicesConfig, plat platform.Platform) error {
	entry, err := findEntry(name, cfg)
	if err != nil {
		return err
	}
	username := currentUsername()
	svcCfg := config.ServiceConfig{Command: entry.Command, Restart: entry.Restart}
	return Install(entry.Name, svcCfg, username, plat)
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
