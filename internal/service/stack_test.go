package service

import (
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

func TestFlatten_empty(t *testing.T) {
	entries := Flatten(config.ServicesConfig{})
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestFlatten_dependencyOrder(t *testing.T) {
	cfg := config.ServicesConfig{
		Deskd: &config.ServiceConfig{
			Command: "deskd serve",
			Restart: "always",
		},
		Agents: map[string]*config.ServiceConfig{
			"dev": {
				Command:   "deskd agent run dev",
				DependsOn: "deskd",
			},
			"kira": {
				Command:   "deskd agent run kira",
				DependsOn: "deskd",
			},
		},
		Adapters: map[string]*config.ServiceConfig{
			"github": {
				Command:   "myhome adapter github start",
				DependsOn: "deskd",
			},
		},
	}

	entries := Flatten(cfg)

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	// deskd must come first (all others depend on it)
	if entries[0].Name != "deskd" {
		t.Errorf("expected first entry to be deskd, got %s", entries[0].Name)
	}

	// Check all names are present
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	for _, want := range []string{"deskd", "agent-dev", "agent-kira", "adapter-github"} {
		if !names[want] {
			t.Errorf("missing entry %s", want)
		}
	}
}

func TestFlatten_deskdOnly(t *testing.T) {
	cfg := config.ServicesConfig{
		Deskd: &config.ServiceConfig{
			Command: "deskd serve",
			Restart: "always",
		},
	}

	entries := Flatten(cfg)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "deskd" {
		t.Errorf("expected deskd, got %s", entries[0].Name)
	}
	if entries[0].Restart != "always" {
		t.Errorf("expected restart=always, got %s", entries[0].Restart)
	}
}

func TestFindEntry(t *testing.T) {
	cfg := config.ServicesConfig{
		Deskd: &config.ServiceConfig{Command: "deskd serve"},
		Agents: map[string]*config.ServiceConfig{
			"dev": {Command: "agent dev", DependsOn: "deskd"},
		},
		Adapters: map[string]*config.ServiceConfig{
			"github": {Command: "adapter github", DependsOn: "deskd"},
		},
	}

	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{name: "deskd", want: "deskd"},
		{name: "agent-dev", want: "agent-dev"},
		{name: "dev", want: "agent-dev"},         // short name
		{name: "github", want: "adapter-github"},  // short name
		{name: "adapter-github", want: "adapter-github"},
		{name: "nonexistent", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := findEntry(tt.name, cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("findEntry(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
			if !tt.wantErr && entry.Name != tt.want {
				t.Errorf("findEntry(%q) = %s, want %s", tt.name, entry.Name, tt.want)
			}
		})
	}
}

func TestTopoSort_noDeps(t *testing.T) {
	entries := []Entry{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}
	sorted := topoSort(entries)
	if len(sorted) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(sorted))
	}
}

func TestTopoSort_chainDeps(t *testing.T) {
	entries := []Entry{
		{Name: "c", DependsOn: "b"},
		{Name: "b", DependsOn: "a"},
		{Name: "a"},
	}
	sorted := topoSort(entries)

	indexOf := make(map[string]int)
	for i, e := range sorted {
		indexOf[e.Name] = i
	}

	if indexOf["a"] > indexOf["b"] {
		t.Error("a should come before b")
	}
	if indexOf["b"] > indexOf["c"] {
		t.Error("b should come before c")
	}
}
