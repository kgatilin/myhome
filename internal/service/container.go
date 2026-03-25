package service

import (
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/container"
)

// BuildAgentContainerCommand builds the full docker/nerdctl run command string
// for an agent service. The resulting command is used as ExecStart in
// systemd/launchd units. The container runs in the foreground (no -d) so the
// service manager handles lifecycle.
func BuildAgentContainerCommand(name string, agentCfg config.AgentConfig, ctrCfg config.Container, cfg *config.Config, serviceCommand string) ([]string, error) {
	runtime, err := container.DetectRuntime(cfg.ContainerRuntime)
	if err != nil {
		return nil, fmt.Errorf("detect container runtime: %w", err)
	}

	homeDir := currentHomeDir()
	containerName := "agent-" + name

	args := []string{runtime, "run", "--rm", "--name", containerName}

	// Run as the host user so mounted volumes are writable
	u, _ := user.Current()
	if u != nil {
		args = append(args, "--user", u.Uid+":"+u.Gid)
	}

	// Firewall: use host network + NET_ADMIN caps
	if ctrCfg.Firewall {
		args = append(args,
			"--cap-add=NET_ADMIN",
			"--cap-add=NET_RAW",
			"--network=host",
		)
	}

	// Container home dir
	containerHome := ctrCfg.HomeDir
	if containerHome == "" {
		containerHome = "/home/node"
	}

	// Container config mounts
	for _, mount := range container.ResolveMounts(ctrCfg.Mounts, homeDir) {
		args = append(args, "-v", mount)
	}

	// Agent-specific mounts
	// When no explicit container path is given (e.g. "~/dev"), map host ~/X
	// to containerHome/X so the container user can access it.
	for _, mount := range agentCfg.Mounts {
		readOnly := false
		if strings.HasSuffix(mount, ":ro") {
			readOnly = true
			mount = strings.TrimSuffix(mount, ":ro")
		}

		parts := strings.SplitN(mount, ":", 2)
		hostPath := expandHome(parts[0], homeDir)
		if !filepath.IsAbs(hostPath) {
			hostPath = filepath.Join(homeDir, hostPath)
		}
		var containerPath string
		if len(parts) > 1 {
			containerPath = expandHome(parts[1], homeDir)
		} else {
			// Map ~/X to containerHome/X
			rel, err := filepath.Rel(homeDir, hostPath)
			if err == nil && !strings.HasPrefix(rel, "..") {
				containerPath = filepath.Join(containerHome, rel)
			} else {
				containerPath = hostPath
			}
		}

		flag := hostPath + ":" + containerPath
		if readOnly {
			flag += ":ro"
		}
		args = append(args, "-v", flag)
	}

	// Bus socket mount
	busSocket := agentCfg.BusSocket
	if busSocket == "" {
		busSocket = "/tmp/deskd.sock"
	}
	args = append(args, "-v", busSocket+":"+busSocket)

	// Mount only this agent's deskd state file (not all agents)
	// Map host path to container home equivalent
	deskdHostFile := filepath.Join(homeDir, ".deskd", "agents", name+".yaml")
	deskdContainerFile := filepath.Join(containerHome, ".deskd", "agents", name+".yaml")
	args = append(args, "-v", deskdHostFile+":"+deskdContainerFile)

	// Mount Claude config + auth
	claudeConfigDir := "~/.claude"
	if cfg.Claude.ConfigDir != "" {
		claudeConfigDir = cfg.Claude.ConfigDir
	}
	resolvedConfigDir := expandHome(claudeConfigDir, homeDir)
	args = append(args,
		"-v", resolvedConfigDir+":"+containerHome+"/.claude",
		"-e", "CLAUDE_CONFIG_DIR="+containerHome+"/.claude",
	)

	// Merge env: agent overrides container
	mergedEnv := make(map[string]string)
	for k, v := range ctrCfg.Env {
		mergedEnv[k] = v
	}
	for k, v := range agentCfg.Env {
		mergedEnv[k] = v
	}
	for k, v := range mergedEnv {
		if v != "" {
			args = append(args, "-e", k+"="+resolveEnvValue(v))
		}
	}

	// Model override
	if agentCfg.Model != "" {
		args = append(args, "-e", "CLAUDE_MODEL="+agentCfg.Model)
	}

	// Git identity
	if agentCfg.Identity.Git.Name != "" {
		args = append(args, "-e", "GIT_AUTHOR_NAME="+agentCfg.Identity.Git.Name)
		args = append(args, "-e", "GIT_COMMITTER_NAME="+agentCfg.Identity.Git.Name)
	}
	if agentCfg.Identity.Git.Email != "" {
		args = append(args, "-e", "GIT_AUTHOR_EMAIL="+agentCfg.Identity.Git.Email)
		args = append(args, "-e", "GIT_COMMITTER_EMAIL="+agentCfg.Identity.Git.Email)
	}

	// Mount deskd binary from host so it's available inside the container
	deskdPath := filepath.Join(homeDir, ".local", "bin", "deskd")
	args = append(args, "-v", deskdPath+":/usr/local/bin/deskd:ro")

	// Image — fall back to claude-code-local:official if not specified
	image := ctrCfg.Image
	if image == "" {
		image = "claude-code-local:official"
	}
	args = append(args, image)

	// The agent command comes from the service config (services.agents.<name>.command).
	// Not hardcoded — supports deskd, uagent, claude, or any runtime.
	// Shell from agent config, defaults to container's shell or /bin/sh.
	shell := agentCfg.Shell
	if shell == "" && ctrCfg.Shell != "" {
		shell = ctrCfg.Shell
	}
	if shell == "" {
		shell = "/bin/sh"
	}
	args = append(args, shell, "-c", fmt.Sprintf("exec %s", serviceCommand))

	return args, nil
}

// resolveEnvValue evaluates shell commands in env values like $(gh auth token).
func resolveEnvValue(v string) string {
	if !strings.HasPrefix(v, "$(") || !strings.HasSuffix(v, ")") {
		return v
	}
	cmd := v[2 : len(v)-1] // strip $( and )
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return v // return original if execution fails
	}
	return strings.TrimSpace(string(out))
}

func expandHome(path, homeDir string) string {
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func currentHomeDir() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.HomeDir
}
