package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// linuxUserOps handles user and group management on Linux.
type linuxUserOps struct {
	cmdRunner
	homeBase string
}

func (l *linuxUserOps) CreateUser(username string) error {
	if err := l.run("useradd", "-m", "-s", "/bin/bash", username); err != nil {
		return fmt.Errorf("create user %s: %w", username, err)
	}
	return nil
}

func (l *linuxUserOps) RemoveUser(username string, removeHome bool) error {
	args := []string{username}
	if removeHome {
		args = append([]string{"-r"}, args...)
	}
	if err := l.run("userdel", args...); err != nil {
		return fmt.Errorf("remove user %s: %w", username, err)
	}
	return nil
}

func (l *linuxUserOps) CreateGroup(group string) error {
	if err := l.run("groupadd", group); err != nil {
		return fmt.Errorf("create group %s: %w", group, err)
	}
	return nil
}

func (l *linuxUserOps) AddUserToGroup(username, group string) error {
	if err := l.run("usermod", "-aG", group, username); err != nil {
		return fmt.Errorf("add %s to group %s: %w", username, group, err)
	}
	return nil
}

func (l *linuxUserOps) SetReadOnlyACL(username, path string) error {
	acl := fmt.Sprintf("u:%s:rX", username)
	if err := l.run("setfacl", "-R", "-m", acl, path); err != nil {
		return fmt.Errorf("set ACL on %s for %s: %w", path, username, err)
	}
	return nil
}

// linuxPackageOps handles package management on Linux via apt.
type linuxPackageOps struct {
	cmdRunner
}

func (l *linuxPackageOps) PackageManager() string { return "apt" }

func (l *linuxPackageOps) InstallPackages(packages []string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"install", "-y"}, packages...)
	if err := l.run("apt-get", args...); err != nil {
		return fmt.Errorf("apt-get install: %w", err)
	}
	return nil
}

func (l *linuxPackageOps) InstallCaskPackages(packages []string) error {
	// No cask equivalent on Linux — no-op.
	return nil
}

func (l *linuxPackageOps) ListInstalledPackages() ([]string, error) {
	out, err := l.output("dpkg-query", "-W", "-f", "${Package}\n")
	if err != nil {
		return nil, fmt.Errorf("dpkg-query: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}

// linuxServiceOps handles systemd service management on Linux.
type linuxServiceOps struct {
	cmdRunner
}

func (l *linuxServiceOps) ServiceInstall(name, command, username string, restart bool) error {
	unit := generateSystemdUnit(name, command, username, restart)
	path := filepath.Join("/etc/systemd/system", fmt.Sprintf("myhome-%s.service", name))
	if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write unit %s: %w", path, err)
	}
	if err := l.run("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	if err := l.run("systemctl", "enable", fmt.Sprintf("myhome-%s.service", name)); err != nil {
		return fmt.Errorf("systemctl enable %s: %w", name, err)
	}
	return nil
}

func (l *linuxServiceOps) ServiceStart(name string) error {
	if err := l.run("systemctl", "start", fmt.Sprintf("myhome-%s.service", name)); err != nil {
		return fmt.Errorf("start service %s: %w", name, err)
	}
	return nil
}

func (l *linuxServiceOps) ServiceStop(name string) error {
	if err := l.run("systemctl", "stop", fmt.Sprintf("myhome-%s.service", name)); err != nil {
		return fmt.Errorf("stop service %s: %w", name, err)
	}
	return nil
}

func (l *linuxServiceOps) ServiceStatus(name string) (bool, error) {
	err := exec.Command("systemctl", "is-active", "--quiet", fmt.Sprintf("myhome-%s.service", name)).Run()
	if err != nil {
		return false, nil
	}
	return true, nil
}

// Linux implements Platform for Linux systems using composed sub-types.
type Linux struct {
	linuxUserOps
	linuxPackageOps
	linuxServiceOps
}

func newLinux() *Linux {
	return &Linux{
		linuxUserOps:    linuxUserOps{homeBase: "/home"},
		linuxPackageOps: linuxPackageOps{},
		linuxServiceOps: linuxServiceOps{},
	}
}

func (l *Linux) OS() string      { return "linux" }
func (l *Linux) HomeDir() string { return "/home" }
func (l *Linux) UserHome(username string) string {
	return filepath.Join("/home", username)
}

func generateSystemdUnit(name, command, username string, restart bool) string {
	restartPolicy := "no"
	if restart {
		restartPolicy = "always"
	}
	homeDir, _ := os.UserHomeDir()
	binPath := fmt.Sprintf("%s/.local/bin:%s/.local/share/mise/shims:/usr/local/bin:/usr/bin:/bin", homeDir, homeDir)
	return fmt.Sprintf(`[Unit]
Description=myhome %s service
After=network.target

[Service]
Type=simple
User=%s
Environment=PATH=%s
Environment=HOME=%s
ExecStart=/bin/sh -c '%s'
Restart=%s

[Install]
WantedBy=multi-user.target
`, name, username, binPath, homeDir, command, restartPolicy)
}
