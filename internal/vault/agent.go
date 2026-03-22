package vault

import (
	"fmt"
	"os"
	"path/filepath"
)

// statFile is a variable so tests can override it.
var statFile = os.Stat

// CreateAgentVault creates a vault for an agent user during user creation.
// It creates:
//   - <agentHome>/vault.kdbx — the agent's vault
//   - <parentSecretsDir>/<agentName>-vault.key — key file stored in parent's ~/.secrets/
//
// The agent's SSH keypair (if present at <agentHome>/.ssh/id_ed25519) is imported
// into the vault.
func CreateAgentVault(agentName, agentHome, parentSecretsDir, masterPassword string, execFn ExecFunc) error {
	if execFn == nil {
		execFn = DefaultExec
	}

	dbPath := filepath.Join(agentHome, "vault.kdbx")
	keyName := agentName + "-vault.key"

	// Create the agent's vault.
	v, err := Init(dbPath, parentSecretsDir, keyName, masterPassword, execFn)
	if err != nil {
		return fmt.Errorf("create agent vault for %s: %w", agentName, err)
	}

	// Import agent's SSH key if it exists.
	sshKeyPath := filepath.Join(agentHome, ".ssh", "id_ed25519")
	if fileExists(sshKeyPath) {
		if err := SSHAdd(v.DBPath, v.KeyFile, masterPassword, "id_ed25519", sshKeyPath, execFn); err != nil {
			// Best-effort: log but don't fail vault creation.
			fmt.Printf("Warning: import SSH key into agent vault: %v\n", err)
		}
	}

	fmt.Printf("Agent vault created: %s (key: %s)\n", dbPath, v.KeyFile)
	return nil
}

// fileExists returns true if the path exists and is a regular file.
func fileExists(path string) bool {
	info, err := statFile(path)
	return err == nil && !info.IsDir()
}
