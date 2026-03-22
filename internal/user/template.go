package user

import (
	"fmt"
	"os"
	"os/exec"
)

// CloneTemplate clones the template repo into the agent's home directory.
func CloneTemplate(templateRepo, agentHome string) error {
	if templateRepo == "" {
		return nil
	}
	// Clone into a temp dir, then move contents to agent home.
	// Using "." as target clones directly into the existing directory.
	cmd := exec.Command("git", "clone", templateRepo, agentHome+"/template-staging")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clone template %s: %w", templateRepo, err)
	}

	// Move template contents into agent home root.
	moveCmd := exec.Command("sh", "-c", fmt.Sprintf(
		"cp -a %q/template-staging/. %q/ && rm -rf %q/template-staging",
		agentHome, agentHome, agentHome,
	))
	moveCmd.Stdout = os.Stdout
	moveCmd.Stderr = os.Stderr
	if err := moveCmd.Run(); err != nil {
		return fmt.Errorf("apply template contents: %w", err)
	}
	return nil
}

// InitAgentRepo initializes git in agent's home and makes an initial commit.
func InitAgentRepo(agentHome, agentName string) error {
	cmds := [][]string{
		{"git", "-C", agentHome, "init"},
		{"git", "-C", agentHome, "add", "."},
		{"git", "-C", agentHome, "commit", "-m", fmt.Sprintf("Initial setup for agent %s", agentName)},
	}

	env := append(os.Environ(),
		fmt.Sprintf("GIT_AUTHOR_NAME=%s", agentName),
		fmt.Sprintf("GIT_AUTHOR_EMAIL=%s@myhome", agentName),
		fmt.Sprintf("GIT_COMMITTER_NAME=%s", agentName),
		fmt.Sprintf("GIT_COMMITTER_EMAIL=%s@myhome", agentName),
	)

	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git %s: %w", c[1], err)
		}
	}
	return nil
}
