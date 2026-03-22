package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Linux implements Platform for Linux systems.
type Linux struct{}

func (l *Linux) OS() string       { return "linux" }
func (l *Linux) HomeDir() string  { return "/home" }

func (l *Linux) UserHome(username string) string {
	return filepath.Join("/home", username)
}

func (l *Linux) CreateUser(username string) error {
	cmd := exec.Command("useradd", "-m", "-s", "/bin/bash", username)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create user %s: %w", username, err)
	}
	return nil
}

func (l *Linux) RemoveUser(username string, removeHome bool) error {
	args := []string{username}
	if removeHome {
		args = append([]string{"-r"}, args...)
	}
	cmd := exec.Command("userdel", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove user %s: %w", username, err)
	}
	return nil
}

func (l *Linux) CreateGroup(group string) error {
	cmd := exec.Command("groupadd", group)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create group %s: %w", group, err)
	}
	return nil
}

func (l *Linux) AddUserToGroup(username, group string) error {
	cmd := exec.Command("usermod", "-aG", group, username)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add %s to group %s: %w", username, group, err)
	}
	return nil
}

func (l *Linux) SetReadOnlyACL(username, path string) error {
	acl := fmt.Sprintf("u:%s:rX", username)
	cmd := exec.Command("setfacl", "-R", "-m", acl, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set ACL on %s for %s: %w", path, username, err)
	}
	return nil
}

func (l *Linux) PackageManager() string { return "apt" }

func (l *Linux) InstallPackages(packages []string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"install", "-y"}, packages...)
	cmd := exec.Command("apt-get", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt-get install: %w", err)
	}
	return nil
}

func (l *Linux) InstallCaskPackages(packages []string) error {
	// No cask equivalent on Linux — no-op.
	return nil
}

func (l *Linux) ListInstalledPackages() ([]string, error) {
	out, err := exec.Command("dpkg-query", "-W", "-f", "${Package}\n").Output()
	if err != nil {
		return nil, fmt.Errorf("dpkg-query: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}

func (l *Linux) ServiceInstall(name, command, username string, restart bool) error {
	unit := generateSystemdUnit(name, command, username, restart)
	path := filepath.Join("/etc/systemd/system", fmt.Sprintf("myhome-%s.service", name))
	if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write unit %s: %w", path, err)
	}
	cmd := exec.Command("systemctl", "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	enableCmd := exec.Command("systemctl", "enable", fmt.Sprintf("myhome-%s.service", name))
	enableCmd.Stdout = os.Stdout
	enableCmd.Stderr = os.Stderr
	if err := enableCmd.Run(); err != nil {
		return fmt.Errorf("systemctl enable %s: %w", name, err)
	}
	return nil
}

func (l *Linux) ServiceStart(name string) error {
	cmd := exec.Command("systemctl", "start", fmt.Sprintf("myhome-%s.service", name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start service %s: %w", name, err)
	}
	return nil
}

func (l *Linux) ServiceStop(name string) error {
	cmd := exec.Command("systemctl", "stop", fmt.Sprintf("myhome-%s.service", name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stop service %s: %w", name, err)
	}
	return nil
}

func (l *Linux) ServiceStatus(name string) (bool, error) {
	err := exec.Command("systemctl", "is-active", "--quiet", fmt.Sprintf("myhome-%s.service", name)).Run()
	if err != nil {
		// is-active returns non-zero when not active — not an error
		return false, nil
	}
	return true, nil
}

func generateSystemdUnit(name, command, username string, restart bool) string {
	restartPolicy := "no"
	if restart {
		restartPolicy = "always"
	}
	return fmt.Sprintf(`[Unit]
Description=myhome %s service
After=network.target

[Service]
Type=simple
User=%s
ExecStart=/bin/sh -c '%s'
Restart=%s

[Install]
WantedBy=multi-user.target
`, name, username, command, restartPolicy)
}
