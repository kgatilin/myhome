package remote

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
)

// InitOpts configures the remote init bootstrap process.
type InitOpts struct {
	Remote     config.Remote
	HomeRepo   string // git clone URL for the home repo
	VaultKey   string // local path to vault key file (~/.secrets/vault.key)
	RemoteUser string // optional: override user parsed from host
}

// Init bootstraps a remote VPS: copies SSH key, clones home repo, copies vault key,
// runs bootstrap.sh, and runs myhome init.
func Init(opts InitOpts, execFn ExecFunc) error {
	if execFn == nil {
		execFn = DefaultExec
	}

	host := opts.Remote.Host

	// Step 1: ensure SSH key is on VPS (skip if already accessible)
	if err := runSSH(execFn, host, "true"); err != nil {
		// Can't connect — try ssh-copy-id
		if err := runCmd(execFn, "ssh-copy-id", host); err != nil {
			return fmt.Errorf("ssh-copy-id to %s: %w", host, err)
		}
	}

	// Step 2: SSH in, set up home repo (handles non-empty home dirs)
	setupRepoCmd := fmt.Sprintf(
		"cd ~ && if [ -d .git ]; then git pull origin main || true; "+
			"else git init && git remote add origin %s && git fetch origin && "+
			"git checkout -f main; fi",
		opts.HomeRepo)
	if err := runSSH(execFn, host, setupRepoCmd); err != nil {
		return fmt.Errorf("setting up home repo on %s: %w", host, err)
	}

	// Step 3: scp vault key file to remote ~/.secrets/vault.key
	if opts.VaultKey != "" {
		// Ensure remote ~/.secrets/ directory exists
		if err := runSSH(execFn, host, "mkdir -p ~/.secrets"); err != nil {
			return fmt.Errorf("creating ~/.secrets on %s: %w", host, err)
		}
		remotePath := host + ":~/.secrets/vault.key"
		if err := runCmd(execFn, "scp", opts.VaultKey, remotePath); err != nil {
			return fmt.Errorf("copying vault key to %s: %w", host, err)
		}
	}

	// Step 4: SSH in, run bootstrap.sh
	if err := runSSH(execFn, host, "~/setup/bootstrap.sh"); err != nil {
		return fmt.Errorf("running bootstrap.sh on %s: %w", host, err)
	}

	// Step 5: SSH in, run myhome init --env <env>
	initCmd := fmt.Sprintf("myhome init --env %s", opts.Remote.Env)
	if err := runSSH(execFn, host, initCmd); err != nil {
		return fmt.Errorf("running myhome init on %s: %w", host, err)
	}

	return nil
}

// runSSH executes a command on the remote host via SSH.
func runSSH(execFn ExecFunc, host, command string) error {
	return runCmd(execFn, "ssh", host, command)
}

// runCmd executes a command and returns an error with stderr on failure.
func runCmd(execFn ExecFunc, name string, args ...string) error {
	cmd := execFn(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%s: %w", msg, err)
		}
		return err
	}
	return nil
}
