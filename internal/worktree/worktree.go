package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
)

// WorktreeInfo holds information about a single worktree.
type WorktreeInfo struct {
	RepoName string
	RepoPath string
	Branch   string
	Path     string
}

// Create creates a new worktree in the given repo.
// It delegates to `wt switch --create <branch>` if wt is available,
// otherwise falls back to `git worktree add`.
func Create(repo *config.Repo, branch, homeDir string) (string, error) {
	repoPath := filepath.Join(homeDir, repo.Path)
	wtDir := worktreeDir(repo, repoPath)
	sanitizedBranch := strings.ReplaceAll(branch, "/", "--")
	wtPath := filepath.Join(wtDir, sanitizedBranch)

	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return "", fmt.Errorf("create worktree dir: %w", err)
	}

	// Try wt first, fall back to git worktree
	if _, err := exec.LookPath("wt"); err == nil {
		cmd := exec.Command("wt", "switch", "--create", branch)
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("wt switch --create %s: %w", branch, err)
		}
	} else {
		cmd := exec.Command("git", "worktree", "add", wtPath, "-b", branch)
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git worktree add %s: %w", branch, err)
		}
	}

	return wtPath, nil
}

// Remove removes a worktree from the given repo.
func Remove(repo *config.Repo, branch, homeDir string) error {
	repoPath := filepath.Join(homeDir, repo.Path)

	// Try wt first, fall back to git worktree
	if _, err := exec.LookPath("wt"); err == nil {
		cmd := exec.Command("wt", "remove", branch)
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("wt remove %s: %w", branch, err)
		}
	} else {
		wtDir := worktreeDir(repo, repoPath)
		sanitizedBranch := strings.ReplaceAll(branch, "/", "--")
		wtPath := filepath.Join(wtDir, sanitizedBranch)
		cmd := exec.Command("git", "worktree", "remove", wtPath)
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git worktree remove %s: %w", branch, err)
		}
	}
	return nil
}

// ListForRepo lists worktrees for a single repo using git worktree list.
func ListForRepo(repo *config.Repo, homeDir string) ([]WorktreeInfo, error) {
	repoPath := filepath.Join(homeDir, repo.Path)
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		return nil, nil // not cloned
	}

	repoName := filepath.Base(repo.Path)
	return listWorktrees(repoName, repoPath)
}

// ListAll lists worktrees across all repos in the environment.
func ListAll(repos []config.Repo, homeDir string) ([]WorktreeInfo, error) {
	var all []WorktreeInfo
	for i := range repos {
		infos, err := ListForRepo(&repos[i], homeDir)
		if err != nil {
			continue // skip repos that can't be listed
		}
		all = append(all, infos...)
	}
	return all, nil
}

func listWorktrees(repoName, repoPath string) ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list in %s: %w", repoPath, err)
	}
	return parseWorktreeList(repoName, repoPath, string(out)), nil
}

func parseWorktreeList(repoName, repoPath, output string) []WorktreeInfo {
	var result []WorktreeInfo
	var current WorktreeInfo
	current.RepoName = repoName
	current.RepoPath = repoPath

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				result = append(result, current)
			}
			current = WorktreeInfo{RepoName: repoName, RepoPath: repoPath}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		}
		if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimPrefix(line, "branch ")
			// Convert refs/heads/main → main
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		}
	}
	// Flush last entry
	if current.Path != "" {
		result = append(result, current)
	}
	return result
}

func worktreeDir(repo *config.Repo, repoPath string) string {
	if repo.Worktrees != nil && repo.Worktrees.Dir != "" {
		return filepath.Join(repoPath, repo.Worktrees.Dir)
	}
	return filepath.Join(repoPath, ".worktrees")
}
