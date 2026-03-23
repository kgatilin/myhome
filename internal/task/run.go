package task

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

// RunOpts configures a dev run task.
type RunOpts struct {
	Repo            string
	Branch          string
	Description     string
	Container       string
	ContainerConfig config.Container // container definition from myhome.yml
	AuthProfile     string
	Domain          string
	ProjectDir      string
	HomeDir         string // user home dir for mount resolution
	Runtime         string // overrides Runner.runtime if set
}

// Run creates a worktree, launches a container, and returns the resulting task.
func (r *Runner) Run(opts RunOpts) (*Task, error) {
	runtime := r.runtime
	if opts.Runtime != "" {
		runtime = opts.Runtime
	}

	id, err := r.store.NextID()
	if err != nil {
		return nil, fmt.Errorf("getting next task ID: %w", err)
	}

	// Determine worktree path: <projectDir>/.worktrees/<branch>
	worktreePath := filepath.Join(opts.ProjectDir, ".worktrees", opts.Branch)

	// Step 1: Create worktree
	cmd := r.execFn("git", "worktree", "add", worktreePath, "-b", opts.Branch)
	cmd.Dir = opts.ProjectDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("creating worktree: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	// Step 2: Build container run command using config from myhome.yml
	logFile := filepath.Join(r.store.LogDir(), fmt.Sprintf("%d.log", id))

	image := opts.ContainerConfig.Image
	if image == "" {
		image = opts.Container // fallback to bare name
	}

	containerArgs := []string{
		"run", "-d",
		"--name", fmt.Sprintf("task-%d", id),
		"-v", worktreePath + ":/workspace",
	}

	// Apply firewall settings from container config
	if opts.ContainerConfig.Firewall {
		containerArgs = append(containerArgs, "--network", "none")
	}

	// Apply mounts from container config
	for _, m := range resolveMounts(opts.ContainerConfig.Mounts, opts.HomeDir) {
		containerArgs = append(containerArgs, "-v", m)
	}

	if opts.AuthProfile != "" {
		containerArgs = append(containerArgs, "--env", "AUTH_PROFILE="+opts.AuthProfile)
	}
	containerArgs = append(containerArgs, image)

	// Apply startup commands from container config
	if len(opts.ContainerConfig.StartupCommands) > 0 {
		script := strings.Join(opts.ContainerConfig.StartupCommands, " && ")
		containerArgs = append(containerArgs, "/bin/sh", "-c", script)
	}

	// Step 3: Start container, capture container ID from stdout
	cmd = r.execFn(runtime, containerArgs...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	stderr.Reset()
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("starting container: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	containerID := strings.TrimSpace(stdout.String())

	// Step 4: Set up log streaming in background
	if err := r.startLogStream(runtime, containerID, logFile); err != nil {
		return nil, fmt.Errorf("setting up log stream: %w", err)
	}

	// Step 5: Create and save the task
	task := &Task{
		ID:           id,
		Type:         TaskTypeRun,
		Domain:       opts.Domain,
		Description:  opts.Description,
		Status:       TaskStatusRunning,
		CreatedAt:    time.Now(),
		Repo:         opts.Repo,
		Branch:       opts.Branch,
		ContainerID:  containerID,
		Container:    opts.Container,
		AuthProfile:  opts.AuthProfile,
		WorktreePath: worktreePath,
		LogFile:      logFile,
	}

	if err := r.store.Save(task); err != nil {
		return nil, fmt.Errorf("saving task: %w", err)
	}

	return task, nil
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

	runtime := r.runtime
	cmd := r.execFn(runtime, "stop", task.ContainerID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stopping container %s: %s: %w", task.ContainerID, strings.TrimSpace(stderr.String()), err)
	}

	return nil
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
		hostPath := spec
		if hostPath == "~" {
			hostPath = homeDir
		} else if strings.HasPrefix(hostPath, "~/") {
			hostPath = homeDir + hostPath[1:]
		}
		result := hostPath + ":" + hostPath
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
