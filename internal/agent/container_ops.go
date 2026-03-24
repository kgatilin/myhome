package agent

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/container"
	"github.com/kgatilin/myhome/internal/vault"
)

// containerOps handles low-level container operations for agents.
type containerOps struct {
	execFn  ExecFunc
	runtime string
	homeDir string
	vault   vault.Reader
}

// stopContainer stops a container using the configured runtime and exec function.
func (c *containerOps) stopContainer(containerID string) error {
	cmd := c.execFn(c.runtime, "stop", containerID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stop container %s: %s: %w", containerID, strings.TrimSpace(stderr.String()), err)
	}
	return nil
}

// rmContainer removes a stopped container.
func (c *containerOps) rmContainer(containerID string) error {
	cmd := c.execFn(c.runtime, "rm", containerID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove container %s: %s: %w", containerID, strings.TrimSpace(stderr.String()), err)
	}
	return nil
}

// containerStatus checks the actual container status from the runtime.
func (c *containerOps) containerStatus(containerID string) (string, error) {
	cmd := c.execFn(c.runtime, "inspect", "--format", "{{.State.Status}}", containerID)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "unknown", nil
	}
	return strings.TrimSpace(stdout.String()), nil
}

// startLogStream pipes container logs to a file in the background.
func (c *containerOps) startLogStream(containerID, logFile string) error {
	f, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("creating log file: %w", err)
	}

	cmd := c.execFn(c.runtime, "logs", "-f", containerID)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("starting log stream: %w", err)
	}

	go func() {
		cmd.Wait()
		f.Close()
	}()

	return nil
}

// extractSSHKey reads an SSH private key from the vault.
func (c *containerOps) extractSSHKey(keyName string) ([]byte, error) {
	if c.vault == nil {
		return nil, fmt.Errorf("vault required for SSH key injection but not provided")
	}
	entryName := "SSH Keys/" + keyName
	return c.vault.GetAttachment(entryName, keyName)
}

// resolveSecretRef resolves a vault:// reference to an env var name and value.
func (c *containerOps) resolveSecretRef(ref string) (string, string, error) {
	if c.vault == nil {
		return "", "", fmt.Errorf("vault required for secret %q but not provided", ref)
	}
	val, err := vault.ResolveVaultRef(ref, c.vault)
	if err != nil {
		return "", "", err
	}
	name := strings.TrimPrefix(ref, "vault://")
	name = strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	return name, val, nil
}

// buildContainerArgs constructs the docker run arguments for an agent container.
func (c *containerOps) buildContainerArgs(containerName string, agentCfg config.AgentConfig, ctrCfg config.Container, cfg *config.Config) ([]string, error) {
	args := []string{"run", "-d", "-t", "--name", containerName}

	// Firewall: use host network + NET_ADMIN caps (container's init-firewall.sh handles rules)
	if ctrCfg.Firewall {
		args = append(args,
			"--cap-add=NET_ADMIN",
			"--cap-add=NET_RAW",
			"--network=host",
		)
	}

	// Container config mounts
	for _, mount := range container.ResolveMounts(ctrCfg.Mounts, c.homeDir) {
		args = append(args, "-v", mount)
	}

	// Agent-specific mounts — same-path principle: mount at host path inside container
	for _, mount := range agentCfg.Mounts {
		readOnly := false
		if strings.HasSuffix(mount, ":ro") {
			readOnly = true
			mount = strings.TrimSuffix(mount, ":ro")
		}

		parts := strings.SplitN(mount, ":", 2)
		hostPath := expandHome(parts[0], c.homeDir)
		if !filepath.IsAbs(hostPath) {
			hostPath = filepath.Join(c.homeDir, hostPath)
		}
		containerPath := hostPath // same-path by default
		if len(parts) > 1 {
			containerPath = expandHome(parts[1], c.homeDir)
		}

		flag := hostPath + ":" + containerPath
		if readOnly {
			flag += ":ro"
		}
		args = append(args, "-v", flag)
	}

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
	resolvedConfigDir := expandHome(claudeConfigDir, c.homeDir)
	args = append(args,
		"-v", resolvedConfigDir+":"+containerHome+"/.claude",
		"-e", "CLAUDE_CONFIG_DIR="+containerHome+"/.claude",
	)

	// SSH key injection: tmpfs mount + key as base64 env var (decoded in startup preamble)
	var sshKeyB64 string
	if agentCfg.Identity.SSH != "" {
		keyData, err := c.extractSSHKey(agentCfg.Identity.SSH)
		if err != nil {
			return nil, fmt.Errorf("extracting SSH key %q: %w", agentCfg.Identity.SSH, err)
		}
		sshKeyB64 = base64.StdEncoding.EncodeToString(keyData)
		args = append(args, "--tmpfs", "/run/agent-ssh:size=1m,mode=0700")
		args = append(args, "-e", "_AGENT_SSH_KEY_B64="+sshKeyB64)
		args = append(args, "-e", "GIT_SSH_COMMAND=ssh -i /run/agent-ssh/id_ed25519 -o StrictHostKeyChecking=no")
	}

	// Vault secrets injection: resolve vault:// refs to env vars
	for _, ref := range agentCfg.Secrets.Vault {
		envName, envVal, err := c.resolveSecretRef(ref)
		if err != nil {
			return nil, fmt.Errorf("resolving secret %q: %w", ref, err)
		}
		args = append(args, "-e", envName+"="+envVal)
	}

	// Container env vars
	for k, v := range ctrCfg.Env {
		resolved := resolveEnvValue(v, c.execFn)
		if resolved != "" {
			args = append(args, "-e", k+"="+resolved)
		}
	}

	// Agent-specific env vars (resolve $(...) shell commands)
	for k, v := range agentCfg.Env {
		resolved := resolveEnvValue(v, c.execFn)
		if resolved != "" {
			args = append(args, "-e", k+"="+resolved)
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

	// Agent containers stay alive and wait for messages via docker exec.
	var scriptParts []string

	// SSH key setup preamble
	if sshKeyB64 != "" {
		scriptParts = append(scriptParts,
			`echo "$_AGENT_SSH_KEY_B64" | base64 -d > /run/agent-ssh/id_ed25519 && chmod 600 /run/agent-ssh/id_ed25519`)
	}

	// Run startup commands except the final claude exec (which contains {{.Prompt}})
	for _, sc := range ctrCfg.StartupCommands {
		if strings.Contains(sc, "{{.Prompt}}") {
			continue // skip — agents don't use one-shot prompt
		}
		scriptParts = append(scriptParts, sc)
	}

	// Keep container alive
	scriptParts = append(scriptParts, "exec sleep infinity")
	script := strings.Join(scriptParts, " ; ")
	args = append(args, "/bin/bash", "-c", script)

	return args, nil
}
