package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

func TestParseWorktreeList(t *testing.T) {
	output := `worktree /home/user/work/uagent
branch refs/heads/main

worktree /home/user/work/uagent/.worktrees/TICKET-123
branch refs/heads/TICKET-123

`
	result := parseWorktreeList("uagent", "/home/user/work/uagent", output)
	if len(result) != 2 {
		t.Fatalf("got %d worktrees, want 2", len(result))
	}
	if result[0].Branch != "main" {
		t.Errorf("first branch = %q, want main", result[0].Branch)
	}
	if result[1].Branch != "TICKET-123" {
		t.Errorf("second branch = %q, want TICKET-123", result[1].Branch)
	}
	if result[0].RepoName != "uagent" {
		t.Errorf("repo name = %q, want uagent", result[0].RepoName)
	}
}

func TestWorktreeDir(t *testing.T) {
	tests := []struct {
		name     string
		repo     *config.Repo
		repoPath string
		want     string
	}{
		{
			name:     "default",
			repo:     &config.Repo{Path: "work/uagent"},
			repoPath: "/home/user/work/uagent",
			want:     "/home/user/work/uagent/.worktrees",
		},
		{
			name: "custom dir",
			repo: &config.Repo{
				Path:      "work/uagent",
				Worktrees: &config.WorktreeConfig{Dir: ".wt"},
			},
			repoPath: "/home/user/work/uagent",
			want:     "/home/user/work/uagent/.wt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := worktreeDir(tt.repo, tt.repoPath)
			if got != tt.want {
				t.Errorf("worktreeDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListForRepo(t *testing.T) {
	homeDir := t.TempDir()
	repoPath := filepath.Join(homeDir, "myrepo")

	// Create a real git repo
	cmds := [][]string{
		{"git", "init", repoPath},
		{"git", "-C", repoPath, "commit", "--allow-empty", "-m", "init"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
		if err := cmd.Run(); err != nil {
			t.Fatalf("setup: %v (%v)", err, c)
		}
	}

	repo := &config.Repo{Path: "myrepo"}
	infos, err := ListForRepo(repo, homeDir)
	if err != nil {
		t.Fatalf("ListForRepo() error: %v", err)
	}
	// Should have at least the main worktree
	if len(infos) == 0 {
		t.Error("expected at least 1 worktree (main)")
	}
}

func TestListAll(t *testing.T) {
	homeDir := t.TempDir()

	// Create two git repos
	for _, name := range []string{"repo1", "repo2"} {
		repoPath := filepath.Join(homeDir, name)
		cmds := [][]string{
			{"git", "init", repoPath},
			{"git", "-C", repoPath, "commit", "--allow-empty", "-m", "init"},
		}
		for _, c := range cmds {
			cmd := exec.Command(c[0], c[1:]...)
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
				"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
			if err := cmd.Run(); err != nil {
				t.Fatalf("setup: %v", err)
			}
		}
	}

	repos := []config.Repo{
		{Path: "repo1"},
		{Path: "repo2"},
		{Path: "nonexistent"}, // should be skipped
	}

	all, err := ListAll(repos, homeDir)
	if err != nil {
		t.Fatalf("ListAll() error: %v", err)
	}
	if len(all) < 2 {
		t.Errorf("expected at least 2 worktrees, got %d", len(all))
	}
}

func TestCreateAndRemoveWorktree(t *testing.T) {
	homeDir := t.TempDir()
	repoPath := filepath.Join(homeDir, "myrepo")

	// Setup git repo with initial commit
	cmds := [][]string{
		{"git", "init", repoPath},
		{"git", "-C", repoPath, "commit", "--allow-empty", "-m", "init"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
		if err := cmd.Run(); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	repo := &config.Repo{Path: "myrepo"}

	// Create worktree
	wtPath, err := Create(repo, "feature-branch", homeDir)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(wtPath); err != nil {
		t.Errorf("worktree path should exist: %s", wtPath)
	}

	// List should show it
	infos, _ := ListForRepo(repo, homeDir)
	found := false
	for _, info := range infos {
		if info.Branch == "feature-branch" {
			found = true
		}
	}
	if !found {
		t.Error("created worktree not found in list")
	}

	// Remove worktree using the branch name (Remove resolves the path)
	if err := Remove(repo, "feature-branch", homeDir); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}
}
