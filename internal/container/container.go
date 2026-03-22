package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
)

// RunOpts holds options for running a container.
type RunOpts struct {
	AuthProfile    *config.AuthProfile
	ClaudeConfigDir string
	ProjectDir     string
	Detach         bool
	ExtraArgs      []string
}

// ContainerInfo represents a running or stopped container from ps output.
type ContainerInfo struct {
	ID     string `json:"ID"`
	Names  string `json:"Names"`
	Image  string `json:"Image"`
	Status string `json:"Status"`
	State  string `json:"State"`
}

// Build builds a container image from the specified Dockerfile.
func Build(runtime string, name string, ctr config.Container, homeDir string) error {
	args := BuildArgs(name, ctr, homeDir)
	cmd := exec.Command(runtime, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build container %s: %w", name, err)
	}
	return nil
}

// BuildArgs returns the arguments for a container build command.
func BuildArgs(name string, ctr config.Container, homeDir string) []string {
	dockerfilePath := expandTilde(ctr.Dockerfile, homeDir)
	if !filepath.IsAbs(dockerfilePath) {
		dockerfilePath = filepath.Join(homeDir, dockerfilePath)
	}
	contextDir := filepath.Dir(dockerfilePath)

	return []string{
		"build",
		"-t", ctr.Image,
		"-f", dockerfilePath,
		contextDir,
	}
}

// Run starts a container with the given configuration and options.
// Returns the container ID on success.
func Run(runtime string, name string, ctr config.Container, homeDir string, opts RunOpts) (string, error) {
	args := RunArgs(name, ctr, homeDir, opts)
	cmd := exec.Command(runtime, args...)
	if !opts.Detach {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("run container %s: %w", name, err)
		}
		return "", nil
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("run container %s: %w", name, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// RunArgs builds the argument list for a container run command.
func RunArgs(name string, ctr config.Container, homeDir string, opts RunOpts) []string {
	args := []string{"run", "--name", name}

	if opts.Detach {
		args = append(args, "-d")
	} else {
		args = append(args, "-it", "--rm")
	}

	// Firewall: drop all network capabilities.
	if ctr.Firewall {
		args = append(args, "--network", "none")
	}

	// Mounts from container config.
	for _, m := range ResolveMounts(ctr.Mounts, homeDir) {
		args = append(args, "-v", m)
	}

	// Project directory mount.
	if opts.ProjectDir != "" {
		projectDir := expandTilde(opts.ProjectDir, homeDir)
		args = append(args, "-v", projectDir+":"+projectDir, "-w", projectDir)
	}

	// Auth profile mounts and env vars.
	if opts.AuthProfile != nil {
		authMounts, authEnvs := ResolveAuth(*opts.AuthProfile, opts.ClaudeConfigDir, homeDir)
		for _, m := range authMounts {
			args = append(args, "-v", m)
		}
		for _, e := range authEnvs {
			args = append(args, "-e", e)
		}
	}

	// Extra args from caller.
	args = append(args, opts.ExtraArgs...)

	// Image.
	args = append(args, ctr.Image)

	// Startup commands run as shell commands inside the container.
	if len(ctr.StartupCommands) > 0 {
		script := strings.Join(ctr.StartupCommands, " && ")
		args = append(args, "/bin/sh", "-c", script)
	}

	return args
}

// List returns information about all containers visible to the runtime.
func List(runtime string) ([]ContainerInfo, error) {
	cmd := exec.Command(runtime, "ps", "-a", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil, nil
	}

	var containers []ContainerInfo

	// Some runtimes output one JSON object per line (nerdctl, docker),
	// others output a JSON array (podman). Try array first, then per-line.
	if err := json.Unmarshal([]byte(trimmed), &containers); err == nil {
		return containers, nil
	}

	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var info ContainerInfo
		if err := json.Unmarshal([]byte(line), &info); err != nil {
			return nil, fmt.Errorf("parse container info %q: %w", line, err)
		}
		containers = append(containers, info)
	}

	return containers, nil
}

// Shell opens an interactive bash shell inside a running container.
func Shell(runtime string, containerID string) error {
	cmd := exec.Command(runtime, "exec", "-it", containerID, "/bin/bash")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("shell into container %s: %w", containerID, err)
	}
	return nil
}

// Stop stops a running container.
func Stop(runtime string, containerID string) error {
	cmd := exec.Command(runtime, "stop", containerID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stop container %s: %s: %w", containerID, string(output), err)
	}
	return nil
}

// Rm removes a stopped container.
func Rm(runtime string, containerID string) error {
	cmd := exec.Command(runtime, "rm", containerID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove container %s: %s: %w", containerID, string(output), err)
	}
	return nil
}
