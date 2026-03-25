package service

import (
	"fmt"
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
func BuildAgentContainerCommand(name string, agentCfg config.AgentConfig, ctrCfg config.Container, cfg *config.Config) (string, error) {
	runtime, err := container.DetectRuntime(cfg.ContainerRuntime)
	if err != nil {
		return "", fmt.Errorf("detect container runtime: %w", err)
	}

	homeDir := currentHomeDir()
	containerName := "agent-" + name

	args := []string{runtime, "run", "--name", containerName, "--replace"}

	// Firewall: use host network + NET_ADMIN caps
	if ctrCfg.Firewall {
		args = append(args,
			"--cap-add=NET_ADMIN",
			"--cap-add=NET_RAW",
			"--network=host",
		)
	}

	// Container config mounts
	for _, mount := range container.ResolveMounts(ctrCfg.Mounts, homeDir) {
		args = append(args, "-v", mount)
	}

	// Agent-specific mounts
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
		containerPath := hostPath
		if len(parts) > 1 {
			containerPath = expandHome(parts[1], homeDir)
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

	// Container home dir
	containerHome := ctrCfg.HomeDir
	if containerHome == "" {
		containerHome = "/home/node"
	}

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

	// Container env vars
	for k, v := range ctrCfg.Env {
		if v != "" {
			args = append(args, "-e", k+"="+v)
		}
	}

	// Agent-specific env vars
	for k, v := range agentCfg.Env {
		if v != "" {
			args = append(args, "-e", k+"="+v)
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

	// Image
	args = append(args, ctrCfg.Image)

	// Build startup script: run container startup commands (except {{.Prompt}})
	// then exec the deskd agent command
	var scriptParts []string
	for _, sc := range ctrCfg.StartupCommands {
		if strings.Contains(sc, "{{.Prompt}}") {
			continue
		}
		scriptParts = append(scriptParts, sc)
	}

	// The actual agent command (from the service config)
	scriptParts = append(scriptParts, fmt.Sprintf("exec deskd agent run %s --socket %s", name, busSocket))
	script := strings.Join(scriptParts, " ; ")
	args = append(args, "/bin/bash", "-c", script)

	return strings.Join(args, " "), nil
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
