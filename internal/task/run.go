package task

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
}

// RunTask creates a worktree, launches a container with Claude, and updates the task in place.
// The task must have Repo and Branch set. Task description is used as the prompt.
func (r *Runner) RunTask(t *Task, opts RunOpts) error {
	// Determine worktree path: <projectDir>/.worktrees/<branch>
	worktreePath := filepath.Join(opts.ProjectDir, ".worktrees", t.Branch)

	// Step 1: Create worktree (skip if already exists)
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		cmd := r.execFn("git", "worktree", "add", worktreePath, "-b", t.Branch)
		cmd.Dir = opts.ProjectDir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			// Branch might already exist, try without -b
			cmd = r.execFn("git", "worktree", "add", worktreePath, t.Branch)
			cmd.Dir = opts.ProjectDir
			stderr.Reset()
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("creating worktree: %s: %w", strings.TrimSpace(stderr.String()), err)
			}
		}
	}

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

	// Mount the worktree as /workspace
	containerArgs = append(containerArgs,
		"-v", worktreePath+":/workspace",
		"-w", "/workspace",
	)

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

	return nil
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
