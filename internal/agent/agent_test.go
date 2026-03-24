package agent

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

// mockExecFunc returns a function that records commands and returns fake output.
func mockExecFunc(outputs map[string]string) ExecFunc {
	return func(name string, args ...string) *exec.Cmd {
		key := name + " " + strings.Join(args, " ")
		output := ""
		for prefix, out := range outputs {
			if strings.HasPrefix(key, prefix) {
				output = out
				break
			}
		}
		// Use echo to return the output
		return exec.Command("echo", "-n", output)
	}
}

func TestManagerCreate(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	outputs := map[string]string{
		"docker run":     "abc123container",
		"docker logs":    "",
		"docker inspect": "running",
	}

	mgr := NewManager(store, mockExecFunc(outputs), "docker", "/home/testuser")

	agentCfg := config.AgentConfig{
		Container:    "claude-personal",
		Model:        "sonnet",
		SystemPrompt: "You are a test agent",
	}
	cfg := &config.Config{InfraConfig: config.InfraConfig{
		Containers: map[string]config.Container{
			"claude-personal": {
				Image:           "claude:latest",
				StartupCommands: []string{"exec claude -p {{.Prompt}}"},
			},
		},
	}}

	if err := mgr.Create("test", agentCfg, cfg); err != nil {
		t.Fatalf("Create: %v", err)
	}

	state, err := store.Load("test")
	if err != nil {
		t.Fatalf("Load after Create: %v", err)
	}

	if state.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", state.Status, StatusRunning)
	}
	if state.ContainerID != "abc123container" {
		t.Errorf("ContainerID = %q, want %q", state.ContainerID, "abc123container")
	}
	if state.Container != "claude-personal" {
		t.Errorf("Container = %q, want %q", state.Container, "claude-personal")
	}
}

func TestManagerCreateDuplicateRunning(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Pre-create a running agent
	state := &State{
		Name:        "existing",
		Status:      StatusRunning,
		ContainerID: "running-container",
		Container:   "test",
	}
	store.Save(state)

	outputs := map[string]string{}
	mgr := NewManager(store, mockExecFunc(outputs), "docker", "/home/testuser")

	agentCfg := config.AgentConfig{Container: "test"}
	cfg := &config.Config{InfraConfig: config.InfraConfig{
		Containers: map[string]config.Container{
			"test": {Image: "test:latest"},
		},
	}}

	err = mgr.Create("existing", agentCfg, cfg)
	if err == nil {
		t.Error("Create should fail for already running agent")
	}
}

func TestManagerStop(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	state := &State{
		Name:        "to-stop",
		Status:      StatusRunning,
		ContainerID: "container-to-stop",
		Container:   "test",
	}
	store.Save(state)

	outputs := map[string]string{
		"docker stop": "",
	}
	mgr := NewManager(store, mockExecFunc(outputs), "docker", "/home/testuser")

	if err := mgr.Stop("to-stop"); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	updated, _ := store.Load("to-stop")
	if updated.Status != StatusStopped {
		t.Errorf("Status = %q, want %q", updated.Status, StatusStopped)
	}
}

func TestManagerRemove(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	state := &State{
		Name:        "to-remove",
		Status:      StatusStopped,
		ContainerID: "old-container",
		Container:   "test",
	}
	store.Save(state)

	outputs := map[string]string{
		"docker stop": "",
		"docker rm":   "",
	}
	mgr := NewManager(store, mockExecFunc(outputs), "docker", "/home/testuser")

	if err := mgr.Remove("to-remove"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	_, err = store.Load("to-remove")
	if err == nil {
		t.Error("Load after Remove should return error")
	}
}

func TestBuildContainerArgs(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	mgr := NewManager(store, exec.Command, "docker", "/home/testuser")

	agentCfg := config.AgentConfig{
		Container:    "claude-personal",
		Model:        "opus",
		SystemPrompt: "You are a test agent",
		Mounts:       []string{"life/family:/workspace"},
		Identity: config.AgentIdentity{
			Git: config.AgentGitIdentity{
				Name:  "Test User",
				Email: "test@example.com",
			},
		},
	}
	ctrCfg := config.Container{
		Image:           "claude:latest",
		Firewall:        true,
		StartupCommands: []string{"exec claude -p {{.Prompt}}"},
	}
	cfg := &config.Config{}

	args, err := mgr.ctr.buildContainerArgs("myhome-agent-test", agentCfg, ctrCfg, cfg)
	if err != nil {
		t.Fatalf("buildContainerArgs: %v", err)
	}

	// Check key args are present
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "--network none") {
		t.Error("missing --network none for firewall")
	}
	if !strings.Contains(argStr, "CLAUDE_MODEL=opus") {
		t.Error("missing CLAUDE_MODEL env var")
	}
	if !strings.Contains(argStr, "GIT_AUTHOR_NAME=Test User") {
		t.Error("missing GIT_AUTHOR_NAME env var")
	}
	if !strings.Contains(argStr, "GIT_AUTHOR_EMAIL=test@example.com") {
		t.Error("missing GIT_AUTHOR_EMAIL env var")
	}
	if !strings.Contains(argStr, "/home/testuser/life/family:/workspace") {
		t.Error("missing agent mount")
	}
	if !strings.Contains(argStr, "claude:latest") {
		t.Error("missing image name")
	}
}

func TestBuildContainerArgsSSHKey(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	mgr := NewManager(store, exec.Command, "docker", "/home/testuser")

	// Create a test vault with SSH key attachment
	v := createTestVaultWithSSHKey(t, "personal", "fake-ssh-key-data")
	mgr.Vault = v

	agentCfg := config.AgentConfig{
		Container: "claude-personal",
		Identity: config.AgentIdentity{
			SSH: "personal",
			Git: config.AgentGitIdentity{
				Name:  "Test User",
				Email: "test@example.com",
			},
		},
	}
	ctrCfg := config.Container{
		Image:           "claude:latest",
		StartupCommands: []string{"exec claude -p {{.Prompt}}"},
	}
	cfg := &config.Config{}

	args, err := mgr.ctr.buildContainerArgs("myhome-agent-test", agentCfg, ctrCfg, cfg)
	if err != nil {
		t.Fatalf("buildContainerArgs: %v", err)
	}

	argStr := strings.Join(args, " ")

	// Should have tmpfs mount
	if !strings.Contains(argStr, "--tmpfs /run/agent-ssh:size=1m,mode=0700") {
		t.Error("missing tmpfs mount for SSH key")
	}

	// Should have GIT_SSH_COMMAND pointing to tmpfs key
	if !strings.Contains(argStr, "GIT_SSH_COMMAND=ssh -i /run/agent-ssh/id_ed25519 -o StrictHostKeyChecking=no") {
		t.Error("missing GIT_SSH_COMMAND env var")
	}

	// Should have base64 encoded key env var
	if !strings.Contains(argStr, "_AGENT_SSH_KEY_B64=") {
		t.Error("missing _AGENT_SSH_KEY_B64 env var")
	}

	// Startup script should contain SSH key decode preamble
	if !strings.Contains(argStr, "base64 -d > /run/agent-ssh/id_ed25519") {
		t.Error("missing SSH key decode in startup script")
	}
}

func TestBuildContainerArgsVaultSecrets(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	mgr := NewManager(store, exec.Command, "docker", "/home/testuser")

	// Create a test vault with secrets
	v := createTestVaultWithSecrets(t, map[string]string{
		"github-pat-personal": "ghp_abc123",
		"alpaca-api-key":      "APCA-KEY-xyz",
	})
	mgr.Vault = v

	agentCfg := config.AgentConfig{
		Container: "claude-personal",
		Secrets: config.AgentSecrets{
			Vault: []string{
				"vault://github-pat-personal",
				"vault://alpaca-api-key",
			},
		},
	}
	ctrCfg := config.Container{
		Image:           "claude:latest",
		StartupCommands: []string{"exec claude -p {{.Prompt}}"},
	}
	cfg := &config.Config{}

	args, err := mgr.ctr.buildContainerArgs("myhome-agent-test", agentCfg, ctrCfg, cfg)
	if err != nil {
		t.Fatalf("buildContainerArgs: %v", err)
	}

	argStr := strings.Join(args, " ")

	// Should have secrets as env vars with uppercased, underscore-separated names
	if !strings.Contains(argStr, "GITHUB_PAT_PERSONAL=ghp_abc123") {
		t.Error("missing GITHUB_PAT_PERSONAL env var")
	}
	if !strings.Contains(argStr, "ALPACA_API_KEY=APCA-KEY-xyz") {
		t.Error("missing ALPACA_API_KEY env var")
	}
}

func TestBuildContainerArgsSSHNoVault(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	mgr := NewManager(store, exec.Command, "docker", "/home/testuser")
	// Vault not set

	agentCfg := config.AgentConfig{
		Container: "claude-personal",
		Identity:  config.AgentIdentity{SSH: "personal"},
	}
	ctrCfg := config.Container{
		Image:           "claude:latest",
		StartupCommands: []string{"exec claude -p {{.Prompt}}"},
	}
	cfg := &config.Config{}

	_, err := mgr.ctr.buildContainerArgs("myhome-agent-test", agentCfg, ctrCfg, cfg)
	if err == nil {
		t.Error("buildContainerArgs should fail when SSH key requested but no vault set")
	}
}
