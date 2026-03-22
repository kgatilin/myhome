package cleanup

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
)

// IssueType categorizes a cleanup finding.
type IssueType string

const (
	OrphanWorktree IssueType = "orphan-worktree"
	StaleBranch    IssueType = "stale-branch"
	LargeFile      IssueType = "large-file"
	EmptyDir       IssueType = "empty-dir"
)

// Issue represents a single cleanup finding.
type Issue struct {
	Type    IssueType
	Path    string
	Details string // e.g., file size for large files
}

const largeFileThreshold = 10 * 1024 * 1024 // 10MB

// Scan checks for garbage across all repos in the environment.
func Scan(repos []config.Repo, homeDir string) ([]Issue, error) {
	var issues []Issue

	for _, r := range repos {
		repoPath := filepath.Join(homeDir, r.Path)
		if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
			continue // not cloned
		}

		orphans, err := findOrphanWorktrees(repoPath)
		if err == nil {
			issues = append(issues, orphans...)
		}

		stale, err := findStaleBranches(repoPath)
		if err == nil {
			issues = append(issues, stale...)
		}

		large, err := findLargeUntrackedFiles(repoPath)
		if err == nil {
			issues = append(issues, large...)
		}
	}

	// Check for empty dirs in home
	empties := findEmptyDirs(homeDir)
	issues = append(issues, empties...)

	return issues, nil
}

// Apply interactively prompts for each issue and performs cleanup.
func Apply(issues []Issue, reader *bufio.Reader) error {
	for _, issue := range issues {
		fmt.Printf("[%s] %s", issue.Type, issue.Path)
		if issue.Details != "" {
			fmt.Printf(" (%s)", issue.Details)
		}
		fmt.Print(" — delete? [y/N] ")

		line, _ := reader.ReadString('\n')
		answer := strings.TrimSpace(strings.ToLower(line))
		if answer != "y" && answer != "yes" {
			fmt.Println("  skipped")
			continue
		}

		if err := applyFix(issue); err != nil {
			fmt.Printf("  error: %v\n", err)
		} else {
			fmt.Println("  removed")
		}
	}
	return nil
}

func applyFix(issue Issue) error {
	switch issue.Type {
	case OrphanWorktree:
		cmd := exec.Command("git", "worktree", "remove", "--force", issue.Path)
		return cmd.Run()
	case StaleBranch:
		// issue.Path is "repoPath:branchName"
		parts := strings.SplitN(issue.Path, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid stale branch path: %s", issue.Path)
		}
		cmd := exec.Command("git", "-C", parts[0], "branch", "-d", parts[1])
		return cmd.Run()
	case LargeFile, EmptyDir:
		return os.RemoveAll(issue.Path)
	default:
		return fmt.Errorf("unknown issue type: %s", issue.Type)
	}
}

func findOrphanWorktrees(repoPath string) ([]Issue, error) {
	out, err := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, err
	}

	var issues []Issue
	var wtPath string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			wtPath = path
		}
		if line == "prunable" && wtPath != "" {
			issues = append(issues, Issue{
				Type: OrphanWorktree,
				Path: wtPath,
			})
		}
		if line == "" {
			wtPath = ""
		}
	}
	return issues, nil
}

func findStaleBranches(repoPath string) ([]Issue, error) {
	// Find merged branches (excluding current and main/master)
	out, err := exec.Command("git", "-C", repoPath, "branch", "--merged").Output()
	if err != nil {
		return nil, err
	}

	var issues []Issue
	for _, line := range strings.Split(string(out), "\n") {
		branch := strings.TrimSpace(line)
		if branch == "" || strings.HasPrefix(branch, "*") {
			continue
		}
		if branch == "main" || branch == "master" {
			continue
		}
		issues = append(issues, Issue{
			Type:    StaleBranch,
			Path:    repoPath + ":" + branch,
			Details: "merged",
		})
	}
	return issues, nil
}

func findLargeUntrackedFiles(repoPath string) ([]Issue, error) {
	out, err := exec.Command("git", "-C", repoPath, "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, err
	}

	var issues []Issue
	for _, line := range strings.Split(string(out), "\n") {
		file := strings.TrimSpace(line)
		if file == "" {
			continue
		}
		absPath := filepath.Join(repoPath, file)
		info, err := os.Stat(absPath)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Size() >= largeFileThreshold {
			issues = append(issues, Issue{
				Type:    LargeFile,
				Path:    absPath,
				Details: formatSize(info.Size()),
			})
		}
	}
	return issues, nil
}

func findEmptyDirs(homeDir string) []Issue {
	var issues []Issue
	// Only check top-level dirs in home that are likely user-created
	entries, err := os.ReadDir(homeDir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dirPath := filepath.Join(homeDir, e.Name())
		subEntries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		if len(subEntries) == 0 {
			issues = append(issues, Issue{
				Type: EmptyDir,
				Path: dirPath,
			})
		}
	}
	return issues
}

func formatSize(bytes int64) string {
	const (
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	switch {
	case bytes >= gb:
		return strconv.FormatFloat(float64(bytes)/float64(gb), 'f', 1, 64) + "GB"
	default:
		return strconv.FormatFloat(float64(bytes)/float64(mb), 'f', 1, 64) + "MB"
	}
}
