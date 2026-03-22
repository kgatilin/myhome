package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
)

// Status describes a repo's current state.
type Status struct {
	config.Repo
	Cloned bool
	Dirty  bool
}

// List returns the status of all repos in the resolved environment.
func List(env *config.ResolvedEnv, homeDir string) ([]Status, error) {
	var result []Status
	for _, r := range env.Repos {
		absPath := filepath.Join(homeDir, r.Path)
		s := Status{Repo: r}
		s.Cloned = isGitRepo(absPath)
		if s.Cloned {
			s.Dirty = isRepoDirty(absPath)
		}
		result = append(result, s)
	}
	return result, nil
}

// Sync clones any missing repos for the given environment.
func Sync(env *config.ResolvedEnv, homeDir string) error {
	for _, r := range env.Repos {
		absPath := filepath.Join(homeDir, r.Path)
		if isGitRepo(absPath) {
			continue
		}
		fmt.Printf("Cloning %s → %s\n", r.URL, r.Path)
		if err := gitClone(r.URL, absPath); err != nil {
			return fmt.Errorf("clone %s: %w", r.Path, err)
		}
	}
	return nil
}

// Add adds a repo to the config. If url is empty, it detects from the existing git remote.
func Add(cfg *config.Config, path, url, env, homeDir string) error {
	if url == "" {
		absPath := filepath.Join(homeDir, path)
		detected, err := detectRemoteURL(absPath)
		if err != nil {
			return fmt.Errorf("detect URL for %s: %w", path, err)
		}
		url = detected
	}
	if env == "" {
		env = "base"
	}
	// Check for duplicate
	for _, r := range cfg.Repos {
		if r.Path == path {
			return fmt.Errorf("repo %s already exists in config", path)
		}
	}
	cfg.Repos = append(cfg.Repos, config.Repo{
		Path: path,
		URL:  url,
		Env:  env,
	})
	return nil
}

// Rm removes a repo from the config by path.
func Rm(cfg *config.Config, path string) error {
	for i, r := range cfg.Repos {
		if r.Path == path {
			cfg.Repos = append(cfg.Repos[:i], cfg.Repos[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("repo %s not found in config", path)
}

// FindByName finds a repo by its short name (last path segment) or full path.
func FindByName(repos []config.Repo, name string) (*config.Repo, error) {
	var matches []*config.Repo
	for i := range repos {
		r := &repos[i]
		basename := filepath.Base(r.Path)
		if basename == name || r.Path == name {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("repo %q not found", name)
	case 1:
		return matches[0], nil
	default:
		paths := make([]string, len(matches))
		for i, m := range matches {
			paths[i] = m.Path
		}
		return nil, fmt.Errorf("ambiguous repo name %q, matches: %s", name, strings.Join(paths, ", "))
	}
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

func isRepoDirty(path string) bool {
	cmd := exec.Command("git", "-C", path, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func gitClone(url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func detectRemoteURL(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no origin remote in %s: %w", repoPath, err)
	}
	return strings.TrimSpace(string(out)), nil
}
