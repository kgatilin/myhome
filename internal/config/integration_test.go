package config

import (
	"os"
	"path/filepath"
	"testing"
)

const fullConfig = `
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
    url: git@gitlab.iponweb.net:bidcore/uagent.git
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
  personal:
    node: "20"

packages:
  base:
    brew: [git, gh]
    apt: [git, gh]
  work:
    brew: [vault]

auth:
  github.com:
    key: id_personal
  gitlab.iponweb.net:
    key: id_work

agent_templates:
  claude-agent:
    template_repo: git@github.com:kgatilin/agent-template.git
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
    mounts:
      - ~/.ssh:ro

remotes:
  vps-work:
    host: user@work-vps.example.com
    home: ~/
    env: work
  vps-personal:
    host: user@personal-vps.example.com
    home: ~/
    env: personal

schedules:
  - id: daily-blog
    prompt: "Write blog post for {date}"
    cron: "0 18 * * 1-5"
    container: claude-code
    auth: work
    workdir: ~/work/blog
    domain: work
`

func TestFullConfigRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "myhome.yml")

	if err := os.WriteFile(cfgPath, []byte(fullConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify all sections parsed.
	if len(cfg.Envs) != 4 {
		t.Errorf("envs count = %d, want 4", len(cfg.Envs))
	}
	if len(cfg.Repos) != 3 {
		t.Errorf("repos count = %d, want 3", len(cfg.Repos))
	}
	if len(cfg.Auth) != 2 {
		t.Errorf("auth count = %d, want 2", len(cfg.Auth))
	}
	if len(cfg.Containers) != 1 {
		t.Errorf("containers count = %d, want 1", len(cfg.Containers))
	}
	if len(cfg.Remotes) != 2 {
		t.Errorf("remotes count = %d, want 2", len(cfg.Remotes))
	}
	if len(cfg.Schedules) != 1 {
		t.Errorf("schedules count = %d, want 1", len(cfg.Schedules))
	}

	// Verify remotes.
	vps, ok := cfg.Remotes["vps-work"]
	if !ok {
		t.Fatal("missing remote vps-work")
	}
	if vps.Host != "user@work-vps.example.com" {
		t.Errorf("vps-work host = %q", vps.Host)
	}
	if vps.Env != "work" {
		t.Errorf("vps-work env = %q", vps.Env)
	}

	// Verify schedules.
	sched := cfg.Schedules[0]
	if sched.ID != "daily-blog" {
		t.Errorf("schedule id = %q", sched.ID)
	}
	if sched.Cron != "0 18 * * 1-5" {
		t.Errorf("schedule cron = %q", sched.Cron)
	}
	if sched.Container != "claude-code" {
		t.Errorf("schedule container = %q", sched.Container)
	}

	// Verify env resolution.
	work, err := cfg.ResolveEnv("work")
	if err != nil {
		t.Fatalf("ResolveEnv(work) error: %v", err)
	}
	if len(work.Repos) != 2 {
		t.Errorf("work repos = %d, want 2 (base + work)", len(work.Repos))
	}
	if work.Tools["go"] != "1.26" {
		t.Errorf("work go tool = %q, want 1.26", work.Tools["go"])
	}
	if work.Tools["python"] != "3.11" {
		t.Errorf("work python tool = %q, want 3.11", work.Tools["python"])
	}

	full, err := cfg.ResolveEnv("full")
	if err != nil {
		t.Fatalf("ResolveEnv(full) error: %v", err)
	}
	if len(full.Repos) != 3 {
		t.Errorf("full repos = %d, want 3", len(full.Repos))
	}

	// Round-trip: save and reload.
	outPath := filepath.Join(tmp, "out.yml")
	if err := cfg.Save(outPath); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	cfg2, err := Load(outPath)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if len(cfg2.Remotes) != 2 {
		t.Errorf("reloaded remotes count = %d, want 2", len(cfg2.Remotes))
	}
	if len(cfg2.Schedules) != 1 {
		t.Errorf("reloaded schedules count = %d, want 1", len(cfg2.Schedules))
	}
}

func TestMissingDepsFriendlyErrors(t *testing.T) {
	// Test that unknown env gives clear error.
	cfg := &Config{
		Envs: map[string]Env{"base": {Include: []string{"base"}}},
	}
	_, err := cfg.ResolveEnv("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown env")
	}
	want := "unknown env: nonexistent"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestNameConflictDetection(t *testing.T) {
	// Two repos with same basename in different envs.
	cfg := &Config{
		Envs: map[string]Env{
			"full": {Include: []string{"work", "personal"}},
		},
		Repos: []Repo{
			{Path: "work/agent", Env: "work"},
			{Path: "personal/agent", Env: "personal"},
		},
	}

	env, err := cfg.ResolveEnv("full")
	if err != nil {
		t.Fatalf("ResolveEnv error: %v", err)
	}

	// Both repos should be included.
	if len(env.Repos) != 2 {
		t.Errorf("full env repos = %d, want 2", len(env.Repos))
	}
}
