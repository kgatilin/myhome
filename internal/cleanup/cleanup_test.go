package cleanup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

func TestScanEmptyDirs(t *testing.T) {
	homeDir := t.TempDir()
	// Create an empty dir (non-hidden)
	os.MkdirAll(filepath.Join(homeDir, "emptydir"), 0o755)
	// Create a non-empty dir
	os.MkdirAll(filepath.Join(homeDir, "notempty"), 0o755)
	os.WriteFile(filepath.Join(homeDir, "notempty", "file.txt"), []byte("x"), 0o644)

	issues, err := Scan(nil, homeDir)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	found := false
	for _, issue := range issues {
		if issue.Type == EmptyDir && filepath.Base(issue.Path) == "emptydir" {
			found = true
		}
		if issue.Type == EmptyDir && filepath.Base(issue.Path) == "notempty" {
			t.Error("notempty should not be reported as empty")
		}
	}
	if !found {
		t.Error("emptydir should be reported")
	}
}

func TestScanLargeFiles(t *testing.T) {
	homeDir := t.TempDir()
	repoPath := filepath.Join(homeDir, "myrepo")

	// Create a git repo
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

	// Create a large untracked file (>10MB)
	largeFile := filepath.Join(repoPath, "bigfile.bin")
	f, err := os.Create(largeFile)
	if err != nil {
		t.Fatalf("create large file: %v", err)
	}
	f.Truncate(11 * 1024 * 1024) // 11MB
	f.Close()

	repos := []config.Repo{{Path: "myrepo"}}
	issues, err := Scan(repos, homeDir)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	found := false
	for _, issue := range issues {
		if issue.Type == LargeFile {
			found = true
		}
	}
	if !found {
		t.Error("large file should be reported")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{10 * 1024 * 1024, "10.0MB"},
		{1536 * 1024 * 1024, "1.5GB"},
		{500 * 1024, "0.5MB"},
	}
	for _, tt := range tests {
		got := formatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestScanStaleBranches(t *testing.T) {
	homeDir := t.TempDir()
	repoPath := filepath.Join(homeDir, "myrepo")

	// Create repo with a merged branch
	cmds := [][]string{
		{"git", "init", repoPath},
		{"git", "-C", repoPath, "commit", "--allow-empty", "-m", "init"},
		{"git", "-C", repoPath, "checkout", "-b", "feature-done"},
		{"git", "-C", repoPath, "commit", "--allow-empty", "-m", "feature"},
		{"git", "-C", repoPath, "checkout", "master"},
		{"git", "-C", repoPath, "merge", "feature-done"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
		if err := cmd.Run(); err != nil {
			t.Fatalf("setup %v: %v", c, err)
		}
	}

	repos := []config.Repo{{Path: "myrepo"}}
	issues, err := Scan(repos, homeDir)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	found := false
	for _, issue := range issues {
		if issue.Type == StaleBranch {
			// The stale branch path format is "repoPath:branchName"
			parts := strings.SplitN(issue.Path, ":", 2)
			if len(parts) == 2 && parts[1] == "feature-done" {
				found = true
			}
		}
	}
	if !found {
		t.Error("merged branch should be reported as stale")
	}
}
