package user

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/platform"
)

// Sync pushes/pulls an agent's home repo.
func Sync(agentName string, plat platform.Platform) error {
	agentHome := plat.UserHome(agentName)
	gitDir := filepath.Join(agentHome, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return fmt.Errorf("agent %s has no git repo at %s: %w", agentName, agentHome, err)
	}

	// Pull first, then push.
	pullCmd := exec.Command("git", "-C", agentHome, "pull", "--rebase", "--autostash")
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	if err := pullCmd.Run(); err != nil {
		// Pull may fail if no remote is configured — not fatal.
		fmt.Printf("Warning: pull failed for %s (may have no remote): %v\n", agentName, err)
	}

	// Add and commit any changes.
	addCmd := exec.Command("git", "-C", agentHome, "add", "-A")
	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	_ = addCmd.Run()

	commitCmd := exec.Command("git", "-C", agentHome, "commit", "-m", "sync")
	commitCmd.Env = append(os.Environ(),
		fmt.Sprintf("GIT_AUTHOR_NAME=%s", agentName),
		fmt.Sprintf("GIT_AUTHOR_EMAIL=%s@myhome", agentName),
		fmt.Sprintf("GIT_COMMITTER_NAME=%s", agentName),
		fmt.Sprintf("GIT_COMMITTER_EMAIL=%s@myhome", agentName),
	)
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	_ = commitCmd.Run() // No changes is fine.

	pushCmd := exec.Command("git", "-C", agentHome, "push")
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		fmt.Printf("Warning: push failed for %s (may have no remote): %v\n", agentName, err)
	}

	return nil
}

// SyncAll syncs all registered agent users.
func SyncAll(state *config.State, plat platform.Platform) error {
	if len(state.Users) == 0 {
		fmt.Println("No registered agent users to sync")
		return nil
	}
	var lastErr error
	for _, name := range state.Users {
		fmt.Printf("Syncing %s...\n", name)
		if err := Sync(name, plat); err != nil {
			fmt.Printf("Error syncing %s: %v\n", name, err)
			lastErr = err
		}
	}
	return lastErr
}
