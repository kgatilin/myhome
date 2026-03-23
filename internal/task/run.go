package task

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
)

// ExecFunc creates an *exec.Cmd for the given command and arguments.
// This indirection enables testing without running real processes.
type ExecFunc func(name string, args ...string) *exec.Cmd

// Runner orchestrates dev run tasks: worktree creation, container launch, and log capture.
type Runner struct {
	store   *Store
	execFn  ExecFunc
	runtime string
}

// NewRunner creates a Runner with the given store, exec function, and container runtime.
func NewRunner(store *Store, execFn ExecFunc, runtime string) *Runner {
	return &Runner{
		store:   store,
		execFn:  execFn,
		runtime: runtime,
	}
}

// RunOpts configures how to launch a task's container.
type RunOpts struct {
	ContainerName   string
	ContainerConfig config.Container // container definition from myhome.yml
	AuthProfile     string
	ClaudeConfig    *config.ClaudeConfig // Claude auth profiles config
	ProjectDir      string
	HomeDir         string // user home dir for mount resolution
	Notify          bool   // send desktop notification on completion
}

// RunTask creates a worktree (via Worktrunk or git), launches a container with Claude,
// and updates the task in place. The task must have Repo and Branch set.
// Task description is used as the prompt. If the worktree already exists (re-run),
// skips creation and launches a new container on the existing worktree.
func (r *Runner) RunTask(t *Task, opts RunOpts) error {
	// Determine worktree path: <projectDir>/.worktrees/<sanitizedBranch>
	sanitizedBranch := strings.ReplaceAll(t.Branch, "/", "--")
	worktreePath := filepath.Join(opts.ProjectDir, ".worktrees", sanitizedBranch)

	// Step 1: Create worktree (skip if already exists — supports re-runs)
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		if err := r.createWorktree(t.Branch, worktreePath, opts.ProjectDir); err != nil {
			return err
		}
	}

	// Track iteration count
	t.Iterations++

	// Step 2: Build container run command matching the cod alias behavior
	logFile := filepath.Join(r.store.LogDir(), fmt.Sprintf("%d.log", t.ID))

	image := opts.ContainerConfig.Image
	if image == "" {
		image = opts.ContainerName
	}

	containerArgs := []string{
		"run", "-d", "--rm",
		"--name", fmt.Sprintf("myhome-task-%d", t.ID),
	}

	// Firewall: use NET_ADMIN/NET_RAW caps + host networking (like cod alias)
	// The container's init-firewall.sh handles the actual firewall rules
	if opts.ContainerConfig.Firewall {
		containerArgs = append(containerArgs,
			"--cap-add=NET_ADMIN",
			"--cap-add=NET_RAW",
			"--network=host",
		)
	}

	// Mount the full project dir so git worktree pointers resolve inside the container.
	// Git worktree .git files contain absolute host paths that don't exist in the container.
	// We create temp copies with paths rewritten to /project and mount them over the originals.
	containerWorkdir := "/project/.worktrees/" + sanitizedBranch
	containerArgs = append(containerArgs,
		"-v", opts.ProjectDir+":/project",
		"-w", containerWorkdir,
	)

	// Create temp .git pointer file with container-relative path
	wtGitFile := filepath.Join(worktreePath, ".git")
	if data, err := os.ReadFile(wtGitFile); err == nil {
		fixed := strings.ReplaceAll(string(data), opts.ProjectDir, "/project")
		tmpGitFile := filepath.Join(os.TempDir(), fmt.Sprintf("myhome-task-%d-dotgit", t.ID))
		if err := os.WriteFile(tmpGitFile, []byte(fixed), 0o644); err == nil {
			containerArgs = append(containerArgs,
				"-v", tmpGitFile+":"+containerWorkdir+"/.git:ro",
			)
		}
	}

	// Create temp gitdir reverse pointer with container-relative path
	wtName := filepath.Base(worktreePath)
	gitdirFile := filepath.Join(opts.ProjectDir, ".git", "worktrees", wtName, "gitdir")
	if data, err := os.ReadFile(gitdirFile); err == nil {
		fixed := strings.ReplaceAll(string(data), opts.ProjectDir, "/project")
		tmpGitdirFile := filepath.Join(os.TempDir(), fmt.Sprintf("myhome-task-%d-gitdir", t.ID))
		if err := os.WriteFile(tmpGitdirFile, []byte(fixed), 0o644); err == nil {
			containerArgs = append(containerArgs,
				"-v", tmpGitdirFile+":/project/.git/worktrees/"+wtName+"/gitdir:ro",
			)
		}
	}

	// Mount Claude config dir
	claudeConfigDir := "~/.claude"
	if opts.ClaudeConfig != nil && opts.ClaudeConfig.ConfigDir != "" {
		claudeConfigDir = opts.ClaudeConfig.ConfigDir
	}
	resolvedConfigDir := expandHome(claudeConfigDir, opts.HomeDir)
	containerArgs = append(containerArgs,
		"-v", resolvedConfigDir+":/home/node/.claude",
		"-e", "CLAUDE_CONFIG_DIR=/home/node/.claude",
	)

	// Mount Claude auth file based on profile
	if opts.AuthProfile != "" && opts.ClaudeConfig != nil {
		if profile, ok := opts.ClaudeConfig.AuthProfiles[opts.AuthProfile]; ok {
			authFile := expandHome(profile.AuthFile, opts.HomeDir)
			containerArgs = append(containerArgs,
				"-v", authFile+":/home/node/.claude.json:ro",
			)
			// Add env vars from auth profile (e.g. CLAUDE_CODE_USE_VERTEX)
			for k, v := range profile.Env {
				containerArgs = append(containerArgs, "-e", k+"="+v)
			}
		}
	}

	// Apply mounts from container config
	for _, m := range resolveMounts(opts.ContainerConfig.Mounts, opts.HomeDir) {
		containerArgs = append(containerArgs, "-v", m)
	}

	// Apply volumes from container config
	for _, v := range opts.ContainerConfig.Volumes {
		containerArgs = append(containerArgs, "-v", v)
	}

	// Node memory limit
	containerArgs = append(containerArgs, "-e", "NODE_OPTIONS=--max-old-space-size=4096")

	// PATH for tools inside container
	containerArgs = append(containerArgs,
		"-e", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/share/npm-global/bin:/usr/local/go/bin:/home/node/go/bin:/go/bin:/home/node/.local/bin",
	)

	// Image
	containerArgs = append(containerArgs, image)

	// Command: build startup script + claude with prompt
	prompt := t.Description
	startupScript := ""

	for _, sc := range opts.ContainerConfig.StartupCommands {
		startupScript += sc + " ; "
	}
	startupScript += fmt.Sprintf("exec claude --dangerously-skip-permissions -p %q", prompt)

	containerArgs = append(containerArgs, "/bin/bash", "-c", startupScript)

	// Step 3: Start container, capture container ID from stdout
	cmd := r.execFn(r.runtime, containerArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("starting container: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	containerID := strings.TrimSpace(stdout.String())

	// Step 4: Set up log streaming in background
	if err := r.startLogStream(r.runtime, containerID, logFile); err != nil {
		return fmt.Errorf("setting up log stream: %w", err)
	}

	// Step 5: Update task fields and persist
	t.Status = TaskStatusRunning
	t.Container = opts.ContainerName
	t.ContainerID = containerID
	t.AuthProfile = opts.AuthProfile
	t.WorktreePath = worktreePath
	t.LogFile = logFile

	if err := r.store.Save(t); err != nil {
		return fmt.Errorf("saving task: %w", err)
	}

	// Spawn background completion watcher
	notify := opts.Notify
	go r.watchCompletion(t.ID, containerID, notify)

	return nil
}

// watchCompletion waits for the container to exit, updates task status, and optionally sends a notification.
func (r *Runner) watchCompletion(taskID int, containerID string, notify bool) {
	// docker wait returns the exit code when the container stops
	cmd := r.execFn(r.runtime, "wait", containerID)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return // container may already be gone
	}

	exitCodeStr := strings.TrimSpace(stdout.String())
	exitCode := 0
	if v, err := strconv.Atoi(exitCodeStr); err == nil {
		exitCode = v
	}

	t, err := r.store.Load(taskID)
	if err != nil {
		return
	}

	t.ExitCode = &exitCode
	if exitCode == 0 {
		t.Status = TaskStatusDone
	} else {
		t.Status = TaskStatusFailed
	}
	r.store.Save(t) //nolint:errcheck

	if notify {
		status := "completed"
		if exitCode != 0 {
			status = fmt.Sprintf("failed (exit %d)", exitCode)
		}
		SendNotification(
			fmt.Sprintf("Task %d %s", taskID, status),
			t.Description,
		)
	}
}

// createWorktree creates a worktree using Worktrunk (wt) if available, falling back to git.
func (r *Runner) createWorktree(branch, worktreePath, projectDir string) error {
	var stderr bytes.Buffer

	// Try Worktrunk first: wt switch --create <branch>
	wtCmd := r.execFn("wt", "switch", "--create", branch)
	wtCmd.Dir = projectDir
	wtCmd.Stderr = &stderr
	if err := wtCmd.Run(); err == nil {
		return nil // Worktrunk handled it (creates in its configured location)
	}

	// Fallback to git worktree
	stderr.Reset()
	cmd := r.execFn("git", "worktree", "add", worktreePath, "-b", branch)
	cmd.Dir = projectDir
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Branch might already exist, try without -b
		stderr.Reset()
		cmd = r.execFn("git", "worktree", "add", worktreePath, branch)
		cmd.Dir = projectDir
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("creating worktree: %s: %w", strings.TrimSpace(stderr.String()), err)
		}
	}
	return nil
}

// Done completes a task: pushes the branch and cleans up the worktree.
// Uses Worktrunk (wt merge/remove) if available, falls back to git.
func (r *Runner) Done(id int, merge bool) error {
	t, err := r.store.Load(id)
	if err != nil {
		return fmt.Errorf("loading task: %w", err)
	}
	if t.WorktreePath == "" {
		return r.store.MarkDone(id)
	}

	// Only push and clean up if worktree path still exists on disk
	if _, err := os.Stat(t.WorktreePath); err == nil {
		var stderr bytes.Buffer

		// Push the branch first
		cmd := r.execFn("git", "push", "origin", t.Branch)
		cmd.Dir = t.WorktreePath
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pushing branch: %s: %w", strings.TrimSpace(stderr.String()), err)
		}

		if merge {
			// Try Worktrunk merge (squash → rebase → merge → remove)
			stderr.Reset()
			wtCmd := r.execFn("wt", "merge")
			wtCmd.Dir = t.WorktreePath
			wtCmd.Stderr = &stderr
			if err := wtCmd.Run(); err != nil {
				// Fallback: just remove the worktree, let CI handle the merge
				fmt.Printf("wt merge failed (%v), removing worktree only\n", err)
			}
		}

		// Remove worktree if it still exists
		if _, err := os.Stat(t.WorktreePath); err == nil {
			stderr.Reset()
			cmd = r.execFn("git", "worktree", "remove", t.WorktreePath)
			cmd.Stderr = &stderr
			_ = cmd.Run() // Best effort
		}
	}

	return r.store.MarkDone(id)
}

// Stop halts the container associated with a running task.
func (r *Runner) Stop(id int) error {
	task, err := r.store.Load(id)
	if err != nil {
		return fmt.Errorf("loading task: %w", err)
	}
	if task.Type != TaskTypeRun {
		return fmt.Errorf("task %d is not a run task", id)
	}
	if task.ContainerID == "" {
		return fmt.Errorf("task %d has no container ID", id)
	}

	cmd := r.execFn(r.runtime, "stop", task.ContainerID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stopping container %s: %s: %w", task.ContainerID, strings.TrimSpace(stderr.String()), err)
	}

	return nil
}

// WaitForContainer blocks until the container exits and returns its exit code.
func (r *Runner) WaitForContainer(containerID string) (int, error) {
	cmd := r.execFn(r.runtime, "wait", containerID)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 1, fmt.Errorf("waiting for container: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	exitCodeStr := strings.TrimSpace(stdout.String())
	exitCode, err := strconv.Atoi(exitCodeStr)
	if err != nil {
		return 1, fmt.Errorf("parsing exit code %q: %w", exitCodeStr, err)
	}
	return exitCode, nil
}

// expandHome replaces ~ with the actual home directory.
func expandHome(path, homeDir string) string {
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return homeDir + path[1:]
	}
	return path
}

// resolveMounts expands mount specs (tilde, :ro suffix) for container -v flags.
func resolveMounts(mounts []string, homeDir string) []string {
	var flags []string
	for _, m := range mounts {
		readOnly := false
		spec := m
		if strings.HasSuffix(spec, ":ro") {
			readOnly = true
			spec = strings.TrimSuffix(spec, ":ro")
		}

		// Handle source:dest mapping (e.g. ~/.uagent/.env:~/.uagent/.env:ro)
		parts := strings.SplitN(spec, ":", 2)
		hostPath := expandHome(parts[0], homeDir)
		containerPath := hostPath
		if len(parts) > 1 {
			containerPath = expandHome(parts[1], homeDir)
		}

		result := hostPath + ":" + containerPath
		if readOnly {
			result += ":ro"
		}
		flags = append(flags, result)
	}
	return flags
}

// startLogStream pipes container logs to a file in the background.
func (r *Runner) startLogStream(runtime, containerID, logFile string) error {
	f, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("creating log file: %w", err)
	}

	cmd := r.execFn(runtime, "logs", "-f", containerID)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("starting log stream: %w", err)
	}

	// Let the goroutine handle cleanup when the process exits
	go func() {
		cmd.Wait()
		f.Close()
	}()

	return nil
}
