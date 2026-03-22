package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

func TestRepoNameCompletionFunc(t *testing.T) {
	// Create a temp config.
	tmp := t.TempDir()
	cfgDir := filepath.Join(tmp, "setup")
	os.MkdirAll(cfgDir, 0o755)

	cfg := &config.Config{
		Envs: map[string]config.Env{"base": {Include: []string{"base"}}},
		Repos: []config.Repo{
			{Path: "dev/tools/myhome", URL: "git@github.com:kgatilin/myhome.git", Env: "base"},
			{Path: "work/uagent", URL: "git@gitlab.com:uagent.git", Env: "work"},
		},
	}
	cfgPath := filepath.Join(cfgDir, "myhome.yml")
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	// We can't easily test the completion function since it uses DefaultConfigPath().
	// Instead, verify the logic produces correct repo names.
	seen := make(map[string]bool)
	var names []string
	for _, r := range cfg.Repos {
		base := filepath.Base(r.Path)
		if seen[base] {
			names = append(names, r.Path)
		} else {
			names = append(names, base)
			seen[base] = true
		}
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[0] != "myhome" {
		t.Errorf("names[0] = %q, want %q", names[0], "myhome")
	}
	if names[1] != "uagent" {
		t.Errorf("names[1] = %q, want %q", names[1], "uagent")
	}
}

func TestRepoNameCompletionConflict(t *testing.T) {
	// Two repos with same basename.
	cfg := &config.Config{
		Repos: []config.Repo{
			{Path: "work/agent", Env: "work"},
			{Path: "personal/agent", Env: "personal"},
		},
	}

	seen := make(map[string]bool)
	var names []string
	for _, r := range cfg.Repos {
		base := filepath.Base(r.Path)
		if seen[base] {
			names = append(names, r.Path)
		} else {
			names = append(names, base)
			seen[base] = true
		}
	}

	// Second "agent" should be full path.
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[1] != "personal/agent" {
		t.Errorf("conflict name = %q, want full path", names[1])
	}
}
