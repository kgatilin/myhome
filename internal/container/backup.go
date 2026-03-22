package container

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitBackup copies the .git directory from projectDir to ~/.git-backups/<project-name>/
// using rsync. This preserves git history before potentially destructive container operations.
func GitBackup(projectDir string, homeDir string) error {
	projectName := filepath.Base(projectDir)
	gitDir := filepath.Join(projectDir, ".git")
	backupDir := filepath.Join(homeDir, ".git-backups", projectName)

	// Ensure trailing slash so rsync copies contents correctly.
	src := strings.TrimSuffix(gitDir, "/") + "/"
	dst := strings.TrimSuffix(backupDir, "/") + "/"

	cmd := exec.Command("rsync", "-a", "--delete", src, dst)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git backup rsync failed: %s: %w", string(output), err)
	}
	return nil
}
