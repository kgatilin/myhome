package vault

import (
	"bytes"
	"fmt"
	"strings"
)

// SSHAdd imports an SSH private key into the vault under a given entry name.
// The key is stored as an attachment in the vault entry.
func SSHAdd(dbPath, keyFile, masterPassword, keyName, keyPath string, execFn ExecFunc) error {
	if execFn == nil {
		execFn = DefaultExec
	}

	// First, create an entry in the vault for the SSH key.
	entryPath := "SSH Keys/" + keyName
	cmd := execFn("keepassxc-cli", "add", dbPath,
		"--key-file", keyFile,
		"--username", keyName,
		"--no-password",
		entryPath,
	)
	cmd.Stdin = strings.NewReader(masterPassword + "\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create vault entry: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	// Attach the private key file to the entry.
	cmd = execFn("keepassxc-cli", "attachment-import", dbPath,
		entryPath,
		keyName,
		keyPath,
		"--key-file", keyFile,
	)
	cmd.Stdin = strings.NewReader(masterPassword + "\n")
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("import SSH key into vault: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	return nil
}

// SSHAgent configures KeePassXC SSH agent integration by enabling the SSH agent
// flag on a vault entry. This requires KeePassXC to be running with SSH agent
// integration enabled in its settings.
func SSHAgent(dbPath, keyFile, masterPassword, keyName string, execFn ExecFunc) error {
	if execFn == nil {
		execFn = DefaultExec
	}

	// Edit the entry to set a custom attribute that KeePassXC uses for SSH agent.
	entryPath := "SSH Keys/" + keyName
	cmd := execFn("keepassxc-cli", "edit", dbPath,
		"--key-file", keyFile,
		entryPath,
		"--set", "KeeAgent.enabled:true",
	)
	cmd.Stdin = strings.NewReader(masterPassword + "\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("enable SSH agent for %s: %s: %w", keyName, strings.TrimSpace(stderr.String()), err)
	}

	return nil
}
