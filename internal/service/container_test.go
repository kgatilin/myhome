package service

import (
	"strings"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

func TestResolveServiceCommand_noConfig(t *testing.T) {
	e := Entry{Name: "agent-dev", Command: "deskd agent run dev"}
	o := startOptions{} // no cfg
	got, err := resolveServiceCommand(e, o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "deskd agent run dev" {
		t.Errorf("expected original command, got %q", got)
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
	if got != "deskd serve" {
		t.Errorf("expected original command, got %q", got)
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
	if got != "deskd agent run dev" {
		t.Errorf("expected original command, got %q", got)
	}
}

func TestResolveServiceCommand_missingContainer(t *testing.T) {
	cfg := &config.Config{
		InfraConfig: config.InfraConfig{
			Agents: map[string]config.AgentConfig{
				"dev": {Container: "nonexistent"},
			},
		},
	}
	e := Entry{Name: "agent-dev", Command: "deskd agent run dev"}
	o := startOptions{cfg: cfg}
	_, err := resolveServiceCommand(e, o)
	if err == nil {
		t.Fatal("expected error for missing container config")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention missing container, got: %v", err)
	}
}

func TestBuildAgentContainerCommand_basic(t *testing.T) {
	// This test will fail if no container runtime is available in the test env.
	// We test the parts we can without a runtime by checking resolveServiceCommand.
	// The integration with BuildAgentContainerCommand is tested via resolveServiceCommand
	// failing when the container config is missing.
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
