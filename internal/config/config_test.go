package config

import (
	"os"
	"path/filepath"
	"testing"
)

const testConfig = `
envs:
  base:
    include: [base]
  work:
    include: [base, work]
  personal:
    include: [base, personal]
  full:
    include: [base, work, personal]

repos:
  - path: dev/tools/myhome
    url: git@github.com:kgatilin/myhome.git
    env: base
  - path: work/uagent
    url: git@gitlab.example.com:team/project/repo.git
    env: work
    worktrees:
      dir: .worktrees
      default_branch: main
  - path: dev/tools/go-arch-lint
    url: git@github.com:kgatilin/go-arch-lint.git
    env: personal

tools:
  base:
    go: "1.26"
  work:
    python: "3.11"
    terraform: "1.7"
  personal:
    node: "20"
    rust: "latest"

packages:
  base:
    brew: [git, gh, jq]
    apt: [git, gh, jq]
  work:
    brew: [vault, cdt]
    apt: [vault]
  full:
    brew_cask: [docker, cursor]

auth:
  github.com:
    key: id_personal
  gitlab.example.com:
    key: id_work

agent_templates:
  claude-agent:
    template_repo: git@github.com:kgatilin/agent-template-claude.git
    service:
      command: "claude --config-dir ~/.claude"
      restart: always

users:
  agent:
    env: work
    template: claude-agent

container_runtime: auto

containers:
  claude-code:
    dockerfile: containers/claude-code/official
    image: claude-code-local:official
    firewall: true
    git_backup: true
    startup_commands:
      - "pip install -r requirements.txt"
    mounts:
      - ~/.ssh:ro
      - ~/.gitconfig:ro
  cursor:
    dockerfile: containers/cursor
    image: cursor-local:latest
    firewall: false
    mounts:
      - ~/.cursor

claude:
  config_dir: ~/.claude
  auth_profiles:
    personal:
      auth_file: ~/.claude.json
    work:
      auth_file: ~/.claude-work.json
    vertex-work:
      auth_file: ~/.claude-vertex.json
      env:
        CLAUDE_CODE_USE_VERTEX: "1"
        ANTHROPIC_VERTEX_PROJECT_ID: my-vertex-project
`

func TestParse(t *testing.T) {
	cfg, err := Parse([]byte(testConfig))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	tests := []struct {
		name string
		check func() bool
	}{
		{"envs count", func() bool { return len(cfg.Envs) == 4 }},
		{"repos count", func() bool { return len(cfg.Repos) == 3 }},
		{"first repo path", func() bool { return cfg.Repos[0].Path == "dev/tools/myhome" }},
		{"worktree config", func() bool { return cfg.Repos[1].Worktrees != nil && cfg.Repos[1].Worktrees.Dir == ".worktrees" }},
		{"tools base go", func() bool { return cfg.Tools["base"]["go"] == "1.26" }},
		{"packages base brew", func() bool { return len(cfg.Packages["base"].Brew) == 3 }},
		{"auth github key", func() bool { return cfg.Auth["github.com"].Key == "id_personal" }},
		{"agent template", func() bool { return cfg.AgentTemplates["claude-agent"].TemplateRepo != "" }},
		{"users agent env", func() bool { return cfg.Users["agent"].Env == "work" }},
		{"container runtime", func() bool { return cfg.ContainerRuntime == "auto" }},
		{"containers count", func() bool { return len(cfg.Containers) == 2 }},
		{"claude-code firewall", func() bool { return cfg.Containers["claude-code"].Firewall == true }},
		{"claude-code mounts", func() bool { return len(cfg.Containers["claude-code"].Mounts) == 2 }},
		{"claude config dir", func() bool { return cfg.Claude.ConfigDir == "~/.claude" }},
		{"claude auth profiles", func() bool { return len(cfg.Claude.AuthProfiles) == 3 }},
		{"vertex env vars", func() bool { return cfg.Claude.AuthProfiles["vertex-work"].Env["CLAUDE_CODE_USE_VERTEX"] == "1" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check() {
				t.Error("check failed")
			}
		})
	}
}

func TestResolveEnv(t *testing.T) {
	cfg, err := Parse([]byte(testConfig))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	tests := []struct {
		envName   string
		wantRepos int
		wantTools []string
		wantBrews int
	}{
		{"base", 1, []string{"go"}, 3},
		{"work", 2, []string{"go", "python", "terraform"}, 5},
		{"personal", 2, []string{"go", "node", "rust"}, 3},
		{"full", 3, []string{"go", "python", "terraform", "node", "rust"}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.envName, func(t *testing.T) {
			resolved, err := cfg.ResolveEnv(tt.envName)
			if err != nil {
				t.Fatalf("ResolveEnv(%s) failed: %v", tt.envName, err)
			}
			if len(resolved.Repos) != tt.wantRepos {
				t.Errorf("repos: got %d, want %d", len(resolved.Repos), tt.wantRepos)
			}
			for _, tool := range tt.wantTools {
				if _, ok := resolved.Tools[tool]; !ok {
					t.Errorf("missing tool %s", tool)
				}
			}
			if len(resolved.Packages.Brew) != tt.wantBrews {
				t.Errorf("brew packages: got %d, want %d", len(resolved.Packages.Brew), tt.wantBrews)
			}
		})
	}
}

func TestResolveEnvUnknown(t *testing.T) {
	cfg, err := Parse([]byte(testConfig))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	_, err = cfg.ResolveEnv("nonexistent")
	if err == nil {
		t.Error("expected error for unknown env")
	}
}

func TestLoadAndSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myhome.yml")

	if err := os.WriteFile(path, []byte(testConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Repos) != 3 {
		t.Errorf("repos: got %d, want 3", len(cfg.Repos))
	}

	// Save and reload
	outPath := filepath.Join(dir, "out.yml")
	if err := cfg.Save(outPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	cfg2, err := Load(outPath)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if len(cfg2.Repos) != len(cfg.Repos) {
		t.Error("round-trip changed repos count")
	}
}

func TestState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yml")

	// Load non-existent returns empty state
	state, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if state.CurrentEnv != "" {
		t.Error("expected empty current env")
	}

	// Set and save
	state.CurrentEnv = "work"
	state.SetSynced("repos")
	state.Users = []string{"agent"}

	if err := state.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload
	state2, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState reload failed: %v", err)
	}
	if state2.CurrentEnv != "work" {
		t.Errorf("current env: got %q, want %q", state2.CurrentEnv, "work")
	}
	if _, ok := state2.LastSync["repos"]; !ok {
		t.Error("missing last_sync for repos")
	}
	if len(state2.Users) != 1 || state2.Users[0] != "agent" {
		t.Error("users not preserved")
	}
}

func TestRepoBuildConfig(t *testing.T) {
	cfg, err := Parse([]byte(`
envs:
  base:
    include: [base]
repos:
  - path: dev/tools/deskd
    url: git@github.com:kgatilin/deskd.git
    env: base
    build:
      command: cargo build --release
      install: cp target/release/deskd ~/bin/deskd
  - path: dev/tools/myhome
    url: git@github.com:kgatilin/myhome.git
    env: base
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Repos[0].Build == nil {
		t.Fatal("first repo should have build config")
	}
	if cfg.Repos[0].Build.Command != "cargo build --release" {
		t.Errorf("build command = %q", cfg.Repos[0].Build.Command)
	}
	if cfg.Repos[0].Build.Install != "cp target/release/deskd ~/bin/deskd" {
		t.Errorf("build install = %q", cfg.Repos[0].Build.Install)
	}
	if cfg.Repos[1].Build != nil {
		t.Error("second repo should not have build config")
	}
}

func TestStateBuildCommits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yml")

	state, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}

	// Initially empty
	if state.GetBuildCommit("dev/tools/deskd") != "" {
		t.Error("expected empty build commit for new state")
	}

	// Set and verify
	state.SetBuildCommit("dev/tools/deskd", "abc123")
	if state.GetBuildCommit("dev/tools/deskd") != "abc123" {
		t.Error("build commit not set correctly")
	}

	// Save and reload
	if err := state.Save(path); err != nil {
		t.Fatal(err)
	}
	state2, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state2.GetBuildCommit("dev/tools/deskd") != "abc123" {
		t.Error("build commit not preserved after save/load")
	}
}

func TestDefaultContainerRuntime(t *testing.T) {
	cfg, err := Parse([]byte(`
envs:
  base:
    include: [base]
repos: []
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContainerRuntime != "auto" {
		t.Errorf("default container_runtime: got %q, want %q", cfg.ContainerRuntime, "auto")
	}
}

func TestFullEnvBrewCask(t *testing.T) {
	// The "full" packages tag is only included if the env's include list contains "full".
	// The test config's "full" env includes [base, work, personal] — not itself.
	// Create a config where full env also includes the "full" tag.
	cfg, err := Parse([]byte(`
envs:
  full:
    include: [base, work, personal, full]
packages:
  base:
    brew: [git]
  full:
    brew_cask: [docker, cursor]
repos: []
`))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := cfg.ResolveEnv("full")
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Packages.BrewCask) != 2 {
		t.Errorf("brew_cask: got %d, want 2", len(resolved.Packages.BrewCask))
	}
}
