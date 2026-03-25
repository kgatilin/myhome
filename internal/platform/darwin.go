package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// darwinUserOps handles user and group management on macOS.
type darwinUserOps struct {
	cmdRunner
	homeBase string
}

func (d *darwinUserOps) CreateUser(username string) error {
	home := filepath.Join(d.homeBase, username)
	if err := d.run("sysadminctl", "-addUser", username, "-home", home, "-shell", "/bin/zsh"); err != nil {
		return fmt.Errorf("create user %s: %w", username, err)
	}
	return nil
}

func (d *darwinUserOps) RemoveUser(username string, removeHome bool) error {
	args := []string{"-deleteUser", username}
	if removeHome {
		args = append(args, "-secure")
	}
	if err := d.run("sysadminctl", args...); err != nil {
		return fmt.Errorf("remove user %s: %w", username, err)
	}
	return nil
}

func (d *darwinUserOps) CreateGroup(group string) error {
	if err := d.run("dseditgroup", "-o", "create", group); err != nil {
		return fmt.Errorf("create group %s: %w", group, err)
	}
	return nil
}

func (d *darwinUserOps) AddUserToGroup(username, group string) error {
	if err := d.run("dseditgroup", "-o", "edit", "-a", username, "-t", "user", group); err != nil {
		return fmt.Errorf("add %s to group %s: %w", username, group, err)
	}
	return nil
}

func (d *darwinUserOps) SetReadOnlyACL(username, path string) error {
	acl := fmt.Sprintf("user:%s allow list,search,readattr,readextattr,readsecurity,read", username)
	if err := d.run("chmod", "+a", acl, path); err != nil {
		return fmt.Errorf("set ACL on %s for %s: %w", path, username, err)
	}
	return nil
}

// darwinPackageOps handles package management on macOS via Homebrew.
type darwinPackageOps struct {
	cmdRunner
}

func (d *darwinPackageOps) PackageManager() string { return "brew" }

func (d *darwinPackageOps) InstallPackages(packages []string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"install"}, packages...)
	if err := d.run("brew", args...); err != nil {
		return fmt.Errorf("brew install: %w", err)
	}
	return nil
}

func (d *darwinPackageOps) InstallCaskPackages(packages []string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"install", "--cask"}, packages...)
	if err := d.run("brew", args...); err != nil {
		return fmt.Errorf("brew install --cask: %w", err)
	}
	return nil
}

func (d *darwinPackageOps) ListInstalledPackages() ([]string, error) {
	out, err := d.output("brew", "list", "--formula", "-1")
	if err != nil {
		return nil, fmt.Errorf("brew list: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}

// darwinServiceOps handles launchd service management on macOS.
type darwinServiceOps struct {
	cmdRunner
	homeBase string
}

func (d *darwinServiceOps) ServiceInstall(name, command, username string, restart bool) error {
	plist := generateLaunchdPlist(name, command, username, restart)
	userHome := filepath.Join(d.homeBase, username)
	path := filepath.Join(userHome, "Library", "LaunchAgents", fmt.Sprintf("com.myhome.%s.plist", name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write plist %s: %w", path, err)
	}
	return nil
}

func (d *darwinServiceOps) ServiceStart(name string) error {
	if err := d.run("launchctl", "load", "-w", launchdPlistPath(name)); err != nil {
		return fmt.Errorf("start service %s: %w", name, err)
	}
	return nil
}

func (d *darwinServiceOps) ServiceStop(name string) error {
	if err := d.run("launchctl", "unload", launchdPlistPath(name)); err != nil {
		return fmt.Errorf("stop service %s: %w", name, err)
	}
	return nil
}

func (d *darwinServiceOps) ServiceStatus(name string) (bool, error) {
	out, err := d.output("launchctl", "list")
	if err != nil {
		return false, fmt.Errorf("launchctl list: %w", err)
	}
	label := fmt.Sprintf("com.myhome.%s", name)
	return strings.Contains(string(out), label), nil
}

// Darwin implements Platform for macOS using composed sub-types.
type Darwin struct {
	darwinUserOps
	darwinPackageOps
	darwinServiceOps
}

func newDarwin() *Darwin {
	return &Darwin{
		darwinUserOps:    darwinUserOps{homeBase: "/Users"},
		darwinPackageOps: darwinPackageOps{},
		darwinServiceOps: darwinServiceOps{homeBase: "/Users"},
	}
}

func (d *Darwin) OS() string      { return "darwin" }
func (d *Darwin) HomeDir() string { return "/Users" }
func (d *Darwin) UserHome(username string) string {
	return filepath.Join("/Users", username)
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
	homeDir, _ := os.UserHomeDir()
	binPath := fmt.Sprintf("%s/.local/bin:%s/go/bin:/usr/local/bin:/usr/bin:/bin", homeDir, homeDir)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.myhome.%s</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>%s</string>
    </dict>
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
</plist>`, name, binPath, command, username, keepAlive, name, name)
}
