package platform

import (
	"runtime"
	"testing"
)

func TestDetect(t *testing.T) {
	p, err := Detect()
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if p.OS() != runtime.GOOS {
		t.Errorf("OS() = %q, want %q", p.OS(), runtime.GOOS)
	}
}

func TestDarwinPaths(t *testing.T) {
	d := &Darwin{}
	if d.HomeDir() != "/Users" {
		t.Errorf("HomeDir() = %q, want /Users", d.HomeDir())
	}
	if d.UserHome("agent") != "/Users/agent" {
		t.Errorf("UserHome(agent) = %q, want /Users/agent", d.UserHome("agent"))
	}
	if d.PackageManager() != "brew" {
		t.Errorf("PackageManager() = %q, want brew", d.PackageManager())
	}
}

func TestLinuxPaths(t *testing.T) {
	l := &Linux{}
	if l.HomeDir() != "/home" {
		t.Errorf("HomeDir() = %q, want /home", l.HomeDir())
	}
	if l.UserHome("agent") != "/home/agent" {
		t.Errorf("UserHome(agent) = %q, want /home/agent", l.UserHome("agent"))
	}
	if l.PackageManager() != "apt" {
		t.Errorf("PackageManager() = %q, want apt", l.PackageManager())
	}
}

func TestLinuxInstallCaskIsNoop(t *testing.T) {
	l := &Linux{}
	if err := l.InstallCaskPackages([]string{"something"}); err != nil {
		t.Errorf("InstallCaskPackages() on Linux should be no-op, got error: %v", err)
	}
}

func TestDarwinInstallEmptyPackages(t *testing.T) {
	d := &Darwin{}
	if err := d.InstallPackages(nil); err != nil {
		t.Errorf("InstallPackages(nil) should be no-op, got error: %v", err)
	}
	if err := d.InstallCaskPackages(nil); err != nil {
		t.Errorf("InstallCaskPackages(nil) should be no-op, got error: %v", err)
	}
}

func TestLinuxInstallEmptyPackages(t *testing.T) {
	l := &Linux{}
	if err := l.InstallPackages(nil); err != nil {
		t.Errorf("InstallPackages(nil) should be no-op, got error: %v", err)
	}
}

func TestGenerateSystemdUnit(t *testing.T) {
	unit := generateSystemdUnit("myagent", "claude --config-dir ~/.claude", "agent", true)
	tests := []struct {
		name     string
		contains string
	}{
		{"description", "myhome myagent service"},
		{"user", "User=agent"},
		{"exec", "ExecStart=/bin/sh -c 'claude --config-dir ~/.claude'"},
		{"restart", "Restart=always"},
		{"install", "WantedBy=multi-user.target"},
	}
	for _, tt := range tests {
		if !containsStr(unit, tt.contains) {
			t.Errorf("systemd unit missing %s: %q not in output", tt.name, tt.contains)
		}
	}
}

func TestGenerateLaunchdPlist(t *testing.T) {
	plist := generateLaunchdPlist("myagent", "claude --config-dir ~/.claude", "agent", true)
	tests := []struct {
		name     string
		contains string
	}{
		{"label", "com.myhome.myagent"},
		{"command", "claude --config-dir ~/.claude"},
		{"user", "<string>agent</string>"},
		{"keepalive", "<true/>"},
		{"run at load", "<key>RunAtLoad</key>"},
	}
	for _, tt := range tests {
		if !containsStr(plist, tt.contains) {
			t.Errorf("launchd plist missing %s: %q not in output", tt.name, tt.contains)
		}
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
