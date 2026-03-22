package vault

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExecFunc creates an *exec.Cmd for the given command and arguments.
// This indirection enables testing without running real processes.
type ExecFunc func(name string, args ...string) *exec.Cmd

// DefaultExec uses os/exec.Command.
func DefaultExec(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// Vault represents a KeePassXC vault (kdbx file + key file).
type Vault struct {
	DBPath  string // path to .kdbx file
	KeyFile string // path to key file in ~/.secrets/
}

// Status holds information about a vault's current state.
type Status struct {
	Exists           bool
	DBPath           string
	KeyFile          string
	KeePassXCRunning bool
}

// Init creates a new KeePassXC vault at the given path with a key file.
// The key file is generated in secretsDir (typically ~/.secrets/).
// masterPassword is used to encrypt the vault.
func Init(dbPath, secretsDir, keyName, masterPassword string, execFn ExecFunc) (*Vault, error) {
	if execFn == nil {
		execFn = DefaultExec
	}

	// Ensure secrets directory exists with restricted permissions.
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		return nil, fmt.Errorf("create secrets dir: %w", err)
	}

	keyFile := filepath.Join(secretsDir, keyName)

	// Generate a random key file.
	if err := generateKeyFile(keyFile); err != nil {
		return nil, fmt.Errorf("generate key file: %w", err)
	}

	// Ensure parent directory of the vault exists.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create vault dir: %w", err)
	}

	// Create vault using keepassxc-cli.
	cmd := execFn("keepassxc-cli", "db-create", dbPath,
		"--set-key-file", keyFile,
		"--set-password",
	)
	cmd.Stdin = strings.NewReader(masterPassword + "\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("create vault: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	return &Vault{DBPath: dbPath, KeyFile: keyFile}, nil
}

// CheckStatus returns the status of a vault.
func CheckStatus(dbPath, keyFile string, execFn ExecFunc) *Status {
	if execFn == nil {
		execFn = DefaultExec
	}

	s := &Status{
		DBPath:  dbPath,
		KeyFile: keyFile,
	}

	// Check if vault file exists.
	if _, err := os.Stat(dbPath); err == nil {
		s.Exists = true
	}

	// Check if KeePassXC is running.
	cmd := execFn("pgrep", "-x", "KeePassXC")
	if err := cmd.Run(); err == nil {
		s.KeePassXCRunning = true
	}

	return s
}

// DefaultVaultPath returns ~/setup/vault.kdbx.
func DefaultVaultPath(homeDir string) string {
	return filepath.Join(homeDir, "setup", "vault.kdbx")
}

// DefaultSecretsDir returns ~/.secrets/.
func DefaultSecretsDir(homeDir string) string {
	return filepath.Join(homeDir, ".secrets")
}

// DefaultKeyFile returns ~/.secrets/vault.key.
func DefaultKeyFile(homeDir string) string {
	return filepath.Join(homeDir, ".secrets", "vault.key")
}

// generateKeyFile creates a random 64-byte key file.
func generateKeyFile(path string) error {
	key := make([]byte, 64)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("generate random bytes: %w", err)
	}
	content := hex.EncodeToString(key)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}
	return nil
}
