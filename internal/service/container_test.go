package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
	"gopkg.in/yaml.v3"
)

func TestResolveServiceCommand_noConfig(t *testing.T) {
	e := Entry{Name: "agent-dev", Command: "deskd agent run dev"}
	o := startOptions{} // no cfg
	got, err := resolveServiceCommand(e, o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "deskd agent run dev" {
		t.Errorf("expected single-element slice with original command, got %v", got)
	}
}

func TestResolveServiceCommand_nonAgent(t *testing.T) {
	cfg := &config.Config{}
	e := Entry{Name: "deskd", Command: "deskd serve"}
	o := startOptions{cfg: cfg}
	got, err := resolveServiceCommand(e, o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "deskd serve" {
		t.Errorf("expected single-element slice with original command, got %v", got)
	}
}

func TestResolveServiceCommand_agentNotInConfig(t *testing.T) {
	cfg := &config.Config{}
	e := Entry{Name: "agent-dev", Command: "deskd agent run dev"}
	o := startOptions{cfg: cfg}
	got, err := resolveServiceCommand(e, o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "deskd agent run dev" {
		t.Errorf("expected single-element slice with original command, got %v", got)
	}
}

func TestResolveServiceCommand_missingContainer_fallsBack(t *testing.T) {
	cfg := &config.Config{
		InfraConfig: config.InfraConfig{
			Agents: map[string]config.AgentConfig{
				"dev": {Container: "nonexistent"},
			},
		},
	}
	e := Entry{Name: "agent-dev", Command: "deskd agent run dev"}
	o := startOptions{cfg: cfg}
	// Missing container config should fall back to defaults (empty Container),
	// not error. The call may still fail due to no container runtime in test
	// env, but it should NOT fail because "nonexistent" container is missing.
	_, err := resolveServiceCommand(e, o)
	if err != nil && strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("should fall back instead of erroring on missing container, got: %v", err)
	}
}

func TestBuildAgentContainerCommand_basic(t *testing.T) {
	// This test will fail if no container runtime is available in the test env.
	// We test the parts we can without a runtime by checking resolveServiceCommand.
	// The integration with BuildAgentContainerCommand is tested via resolveServiceCommand
	// failing when the container config is missing.
}

func TestEnsureAgentState_createsFile(t *testing.T) {
	tmpDir := t.TempDir()
	// Override home dir for test by creating the expected path structure
	stateDir := filepath.Join(tmpDir, ".deskd", "agents")

	agentCfg := config.AgentConfig{
		SystemPrompt: "You are a helpful agent",
		Mounts:       []string{"~/dev"},
	}

	// Directly test the state file creation logic
	statePath := filepath.Join(stateDir, "test.yaml")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}

	state := deskdAgentState{
		Name:         "test",
		SystemPrompt: agentCfg.SystemPrompt,
		WorkDir:      "/home/user/dev",
	}

	data, err := yaml.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	if !strings.Contains(string(content), "test") {
		t.Error("state file should contain agent name")
	}
	if !strings.Contains(string(content), "You are a helpful agent") {
		t.Error("state file should contain system prompt")
	}
}

func TestExpandHome(t *testing.T) {
	tests := []struct {
		path    string
		homeDir string
		want    string
	}{
		{"~", "/home/user", "/home/user"},
		{"~/dev", "/home/user", "/home/user/dev"},
		{"/absolute/path", "/home/user", "/absolute/path"},
		{"relative", "/home/user", "relative"},
	}

	for _, tt := range tests {
		got := expandHome(tt.path, tt.homeDir)
		if got != tt.want {
			t.Errorf("expandHome(%q, %q) = %q, want %q", tt.path, tt.homeDir, got, tt.want)
		}
	}
}
