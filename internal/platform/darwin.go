package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Darwin implements Platform for macOS.
type Darwin struct{}

func (d *Darwin) OS() string       { return "darwin" }
func (d *Darwin) HomeDir() string  { return "/Users" }

func (d *Darwin) UserHome(username string) string {
	return filepath.Join("/Users", username)
}

func (d *Darwin) CreateUser(username string) error {
	cmd := exec.Command("sysadminctl", "-addUser", username, "-home", d.UserHome(username), "-shell", "/bin/zsh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create user %s: %w", username, err)
	}
	return nil
}

func (d *Darwin) RemoveUser(username string, removeHome bool) error {
	args := []string{"-deleteUser", username}
	if removeHome {
		args = append(args, "-secure")
	}
	cmd := exec.Command("sysadminctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove user %s: %w", username, err)
	}
	return nil
}

func (d *Darwin) CreateGroup(group string) error {
	cmd := exec.Command("dseditgroup", "-o", "create", group)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create group %s: %w", group, err)
	}
	return nil
}

func (d *Darwin) AddUserToGroup(username, group string) error {
	cmd := exec.Command("dseditgroup", "-o", "edit", "-a", username, "-t", "user", group)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add %s to group %s: %w", username, group, err)
	}
	return nil
}

func (d *Darwin) SetReadOnlyACL(username, path string) error {
	acl := fmt.Sprintf("user:%s allow list,search,readattr,readextattr,readsecurity,read", username)
	cmd := exec.Command("chmod", "+a", acl, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set ACL on %s for %s: %w", path, username, err)
	}
	return nil
}

func (d *Darwin) PackageManager() string { return "brew" }

func (d *Darwin) InstallPackages(packages []string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"install"}, packages...)
	cmd := exec.Command("brew", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew install: %w", err)
	}
	return nil
}

func (d *Darwin) InstallCaskPackages(packages []string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"install", "--cask"}, packages...)
	cmd := exec.Command("brew", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew install --cask: %w", err)
	}
	return nil
}

func (d *Darwin) ListInstalledPackages() ([]string, error) {
	out, err := exec.Command("brew", "list", "--formula", "-1").Output()
	if err != nil {
		return nil, fmt.Errorf("brew list: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}

func (d *Darwin) ServiceInstall(name, command, username string, restart bool) error {
	plist := generateLaunchdPlist(name, command, username, restart)
	path := filepath.Join(d.UserHome(username), "Library", "LaunchAgents", fmt.Sprintf("com.myhome.%s.plist", name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write plist %s: %w", path, err)
	}
	return nil
}

func (d *Darwin) ServiceStart(name string) error {
	cmd := exec.Command("launchctl", "load", "-w", launchdPlistPath(name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start service %s: %w", name, err)
	}
	return nil
}

func (d *Darwin) ServiceStop(name string) error {
	cmd := exec.Command("launchctl", "unload", launchdPlistPath(name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stop service %s: %w", name, err)
	}
	return nil
}

func (d *Darwin) ServiceStatus(name string) (bool, error) {
	out, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return false, fmt.Errorf("launchctl list: %w", err)
	}
	label := fmt.Sprintf("com.myhome.%s", name)
	return strings.Contains(string(out), label), nil
}

func launchdPlistPath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", fmt.Sprintf("com.myhome.%s.plist", name))
}

func generateLaunchdPlist(name, command, username string, restart bool) string {
	keepAlive := "false"
	if restart {
		keepAlive = "true"
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.myhome.%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>/bin/sh</string>
        <string>-c</string>
        <string>%s</string>
    </array>
    <key>UserName</key>
    <string>%s</string>
    <key>KeepAlive</key>
    <%s/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/com.myhome.%s.out.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/com.myhome.%s.err.log</string>
</dict>
</plist>`, name, command, username, keepAlive, name, name)
}
