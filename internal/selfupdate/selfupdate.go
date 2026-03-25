package selfupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Run performs a self-update of the myhome binary:
// 1. Pulls latest changes in the source repo
// 2. Builds a new binary
// 3. Replaces the current binary
func Run(sourceDir string) error {
	fmt.Printf("Pulling latest changes in %s\n", sourceDir)
	if err := gitPull(sourceDir); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}

	currentBin := installPath()


	tmpBin := currentBin + ".new"
	fmt.Printf("Building myhome to %s\n", tmpBin)
	if err := goBuild(sourceDir, tmpBin); err != nil {
		os.Remove(tmpBin)
		return fmt.Errorf("go build: %w", err)
	}

	fmt.Printf("Replacing %s\n", currentBin)
	if err := replaceBinary(tmpBin, currentBin); err != nil {
		os.Remove(tmpBin)
		return fmt.Errorf("replace binary: %w", err)
	}

	fmt.Println("Self-update complete")
	return nil
}

// FindSourceDir locates the myhome source repo by checking the well-known path
// and falling back to searching config repo paths.
func FindSourceDir(homeDir string, repoPaths []string) (string, error) {
	wellKnown := filepath.Join(homeDir, "dev", "tools", "myhome")
	if isGitRepo(wellKnown) {
		return wellKnown, nil
	}

	for _, p := range repoPaths {
		if filepath.Base(p) == "myhome" {
			abs := filepath.Join(homeDir, p)
			if isGitRepo(abs) {
				return abs, nil
			}
		}
	}

	return "", fmt.Errorf("myhome source repo not found (checked %s and config repos)", wellKnown)
}

func gitPull(dir string) error {
	cmd := exec.Command("git", "-C", dir, "pull", "--ff-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func goBuild(sourceDir, output string) error {
	// Get git commit for version stamp
	version := gitVersion(sourceDir)
	ldflags := fmt.Sprintf("-X github.com/kgatilin/myhome/internal/cmd.Version=%s", version)
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", output, "./cmd/myhome/")
	cmd.Dir = sourceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = envWithMiseShims()
	return cmd.Run()
}

func gitVersion(dir string) string {
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%h").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func replaceBinary(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		// Cross-device fallback: copy then remove
		data, readErr := os.ReadFile(src)
		if readErr != nil {
			return readErr
		}
		info, statErr := os.Stat(dst)
		mode := os.FileMode(0o755)
		if statErr == nil {
			mode = info.Mode()
		}
		if writeErr := os.WriteFile(dst, data, mode); writeErr != nil {
			return writeErr
		}
		os.Remove(src)
	}
	return nil
}

// installPath returns the stable install location for the myhome binary.
// Prefers /usr/local/bin if writable (Linux VPS as root), otherwise ~/.local/bin.
func installPath() string {
	const systemPath = "/usr/local/bin/myhome"
	if f, err := os.OpenFile(systemPath, os.O_WRONLY, 0); err == nil {
		f.Close()
		return systemPath
	}
	home, _ := os.UserHomeDir()
	localBin := filepath.Join(home, ".local", "bin")
	os.MkdirAll(localBin, 0o755)
	return filepath.Join(localBin, "myhome")
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

// envWithMiseShims returns the current environment with mise shims prepended to PATH.
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
	return append(env, "PATH="+shimsDir)
}
