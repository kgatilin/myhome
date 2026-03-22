package user

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GenerateSSHKeypair generates an ed25519 SSH keypair for the agent user.
// Stores in <agentHome>/.ssh/id_ed25519 and id_ed25519.pub.
func GenerateSSHKeypair(agentHome, agentName string) error {
	sshDir := filepath.Join(agentHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("create .ssh dir: %w", err)
	}

	keyPath := filepath.Join(sshDir, "id_ed25519")
	cmd := exec.Command("ssh-keygen",
		"-t", "ed25519",
		"-C", fmt.Sprintf("%s@myhome", agentName),
		"-f", keyPath,
		"-N", "", // empty passphrase for agent keys
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("generate SSH keypair: %w", err)
	}
	return nil
}
