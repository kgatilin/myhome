package repo

import (
	"errors"
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
// If a repo has a build config, it builds and installs binaries after clone or pull.
// It continues on failures and returns all errors joined.
func Sync(env *config.ResolvedEnv, homeDir string) error {
	return SyncWithState(env, homeDir, nil, "")
}

// SyncWithState is like Sync but also tracks build commits in the state file.
func SyncWithState(env *config.ResolvedEnv, homeDir string, state *config.State, statePath string) error {
	var errs []error
	stateChanged := false
	for _, r := range env.Repos {
		absPath := filepath.Join(homeDir, r.Path)
		if isGitRepo(absPath) {
			// Pull latest changes
			if err := gitPull(absPath); err != nil {
				fmt.Printf("  ✗ pull %s: %v\n", r.Path, err)
				// Continue even if pull fails (might have local changes)
			}
			if r.Build != nil {
				if err := buildIfNeeded(r, absPath, state); err != nil {
					fmt.Printf("  ✗ build %s: %v\n", r.Path, err)
					errs = append(errs, fmt.Errorf("build %s: %w", r.Path, err))
				} else {
					stateChanged = true
				}
			}
			continue
		}
		fmt.Printf("Cloning %s → %s\n", r.URL, r.Path)
		if err := gitClone(r.URL, absPath); err != nil {
			fmt.Printf("  ✗ %s: %v\n", r.Path, err)
			errs = append(errs, fmt.Errorf("clone %s: %w", r.Path, err))
			continue
		}
		if r.Build != nil {
			if err := buildRepo(r, absPath, state); err != nil {
				fmt.Printf("  ✗ build %s: %v\n", r.Path, err)
				errs = append(errs, fmt.Errorf("build %s: %w", r.Path, err))
			} else {
				stateChanged = true
			}
		}
	}
	if state != nil && statePath != "" && stateChanged {
		if err := state.Save(statePath); err != nil {
			errs = append(errs, fmt.Errorf("save state: %w", err))
		}
	}
	return errors.Join(errs...)
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

// buildIfNeeded builds a repo only if its HEAD has changed since the last build.
func buildIfNeeded(r config.Repo, absPath string, state *config.State) error {
	head, err := gitHead(absPath)
	if err != nil {
		return err
	}
	if state != nil && state.GetBuildCommit(r.Path) == head {
		return nil
	}
	return buildRepo(r, absPath, state)
}

// buildRepo runs the build command and install command for a repo.
func buildRepo(r config.Repo, absPath string, state *config.State) error {
	fmt.Printf("Building %s\n", r.Path)
	if err := runShell(r.Build.Command, absPath); err != nil {
		return fmt.Errorf("build command: %w", err)
	}
	if r.Build.Install != "" {
		fmt.Printf("Installing %s\n", r.Path)
		if err := runShell(r.Build.Install, absPath); err != nil {
			return fmt.Errorf("install command: %w", err)
		}
	}
	if state != nil {
		head, err := gitHead(absPath)
		if err == nil {
			state.SetBuildCommit(r.Path, head)
		}
	}
	return nil
}

func gitHead(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD in %s: %w", repoPath, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runShell(command, dir string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = envWithMiseShims()
	return cmd.Run()
}

// envWithMiseShims returns the current environment with mise shims prepended to PATH,
// so build commands can find mise-managed tools (go, cargo, node, etc.).
func envWithMiseShims() []string {
	home, _ := os.UserHomeDir()
	shimsDir := filepath.Join(home, ".local", "share", "mise", "shims")
	env := os.Environ()
	for i, e := range env {
		if val, ok := strings.CutPrefix(e, "PATH="); ok {
			env[i] = "PATH=" + shimsDir + string(os.PathListSeparator) + val
			return env
		}
	}
	// No PATH found — set one
	return append(env, "PATH="+shimsDir)
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

func gitPull(path string) error {
	// Stash local changes, pull, then pop
	dirty := isRepoDirty(path)
	if dirty {
		stash := exec.Command("git", "-C", path, "stash")
		stash.Stdout = os.Stdout
		stash.Stderr = os.Stderr
		if err := stash.Run(); err != nil {
			return fmt.Errorf("git stash: %w", err)
		}
	}
	pull := exec.Command("git", "-C", path, "pull", "--ff-only")
	pull.Stdout = os.Stdout
	pull.Stderr = os.Stderr
	pullErr := pull.Run()
	if dirty {
		pop := exec.Command("git", "-C", path, "stash", "pop")
		pop.Stdout = os.Stdout
		pop.Stderr = os.Stderr
		pop.Run() // best effort
	}
	return pullErr
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
