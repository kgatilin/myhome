package vault

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// fakeExec returns an ExecFunc that records calls and succeeds.
func fakeExec(calls *[]string) ExecFunc {
	return func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, name+" "+joinArgs(args))
		return exec.Command("true")
	}
}

func joinArgs(args []string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}

// fakeExecFail returns an ExecFunc that always fails.
func fakeExecFail() ExecFunc {
	return func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
}

func TestInit(t *testing.T) {
	tmp := t.TempDir()
	secretsDir := filepath.Join(tmp, ".secrets")
	dbPath := filepath.Join(tmp, "setup", "vault.kdbx")

	var calls []string
	v, err := Init(dbPath, secretsDir, "vault.key", "testpass", fakeExec(&calls))
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	if v.DBPath != dbPath {
		t.Errorf("DBPath = %q, want %q", v.DBPath, dbPath)
	}
	wantKeyFile := filepath.Join(secretsDir, "vault.key")
	if v.KeyFile != wantKeyFile {
		t.Errorf("KeyFile = %q, want %q", v.KeyFile, wantKeyFile)
	}

	// Key file should exist.
	if _, err := os.Stat(wantKeyFile); os.IsNotExist(err) {
		t.Error("key file not created")
	}

	// Key file should have restricted permissions.
	info, err := os.Stat(wantKeyFile)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("key file permissions = %o, want 600", perm)
	}

	// Secrets dir should have restricted permissions.
	info, err = os.Stat(secretsDir)
	if err != nil {
		t.Fatalf("stat secrets dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("secrets dir permissions = %o, want 700", perm)
	}

	// Should have called keepassxc-cli db-create.
	if len(calls) == 0 {
		t.Fatal("no exec calls recorded")
	}
	if got := calls[0]; got != "keepassxc-cli db-create "+dbPath+" --set-key-file "+wantKeyFile+" --set-password" {
		t.Errorf("exec call = %q", got)
	}
}

func TestInitFailsOnExecError(t *testing.T) {
	tmp := t.TempDir()
	secretsDir := filepath.Join(tmp, ".secrets")
	dbPath := filepath.Join(tmp, "vault.kdbx")

	_, err := Init(dbPath, secretsDir, "vault.key", "testpass", fakeExecFail())
	if err == nil {
		t.Fatal("Init() should fail when keepassxc-cli fails")
	}
}

func TestCheckStatus(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "vault.kdbx")
	keyFile := filepath.Join(tmp, "vault.key")

	// Vault doesn't exist.
	s := CheckStatus(dbPath, keyFile, fakeExecFail())
	if s.Exists {
		t.Error("Exists should be false for non-existent vault")
	}

	// Create fake vault file.
	os.WriteFile(dbPath, []byte("fake"), 0o644)
	s = CheckStatus(dbPath, keyFile, fakeExecFail())
	if !s.Exists {
		t.Error("Exists should be true after creating vault file")
	}
}

func TestDefaultPaths(t *testing.T) {
	home := "/home/test"
	if got := DefaultVaultPath(home); got != "/home/test/setup/vault.kdbx" {
		t.Errorf("DefaultVaultPath = %q", got)
	}
	if got := DefaultSecretsDir(home); got != "/home/test/.secrets" {
		t.Errorf("DefaultSecretsDir = %q", got)
	}
	if got := DefaultKeyFile(home); got != "/home/test/.secrets/vault.key" {
		t.Errorf("DefaultKeyFile = %q", got)
	}
}

func TestSSHAdd(t *testing.T) {
	var calls []string
	err := SSHAdd("/vault.kdbx", "/vault.key", "pass", "id_ed25519", "/ssh/id_ed25519", fakeExec(&calls))
	if err != nil {
		t.Fatalf("SSHAdd() error: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 exec calls, got %d: %v", len(calls), calls)
	}
	// First call: create entry.
	// Second call: import attachment.
}

func TestSSHAgent(t *testing.T) {
	var calls []string
	err := SSHAgent("/vault.kdbx", "/vault.key", "pass", "id_ed25519", fakeExec(&calls))
	if err != nil {
		t.Fatalf("SSHAgent() error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(calls))
	}
}

func TestCreateAgentVault(t *testing.T) {
	tmp := t.TempDir()
	agentHome := filepath.Join(tmp, "agent")
	secretsDir := filepath.Join(tmp, ".secrets")

	// Create agent SSH key so the import path is exercised.
	sshDir := filepath.Join(agentHome, ".ssh")
	os.MkdirAll(sshDir, 0o700)
	os.WriteFile(filepath.Join(sshDir, "id_ed25519"), []byte("fake-key"), 0o600)

	var calls []string
	err := CreateAgentVault("agent1", agentHome, secretsDir, "pass", fakeExec(&calls))
	if err != nil {
		t.Fatalf("CreateAgentVault() error: %v", err)
	}

	// Should have called db-create and then ssh-add (2 calls for add).
	if len(calls) < 1 {
		t.Fatalf("expected exec calls, got %d", len(calls))
	}
}

func TestCreateAgentVaultNoSSHKey(t *testing.T) {
	tmp := t.TempDir()
	agentHome := filepath.Join(tmp, "agent")
	secretsDir := filepath.Join(tmp, ".secrets")

	var calls []string
	err := CreateAgentVault("agent1", agentHome, secretsDir, "pass", fakeExec(&calls))
	if err != nil {
		t.Fatalf("CreateAgentVault() error: %v", err)
	}

	// Should only have the db-create call (no SSH import).
	if len(calls) != 1 {
		t.Fatalf("expected 1 exec call (db-create only), got %d: %v", len(calls), calls)
	}
}

func TestGenerateKeyFile(t *testing.T) {
	tmp := t.TempDir()
	keyPath := filepath.Join(tmp, "test.key")

	if err := generateKeyFile(keyPath); err != nil {
		t.Fatalf("generateKeyFile() error: %v", err)
	}

	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key file: %v", err)
	}
	// 64 bytes -> 128 hex chars.
	if len(data) != 128 {
		t.Errorf("key file length = %d, want 128", len(data))
	}
}
