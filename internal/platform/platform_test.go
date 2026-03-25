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

func TestGenerateSystemdUnit_singleArg(t *testing.T) {
	unit := generateSystemdUnit("myagent", []string{"claude --config-dir ~/.claude"}, "agent", true)
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

func TestGenerateSystemdUnit_multiArg(t *testing.T) {
	args := []string{"docker", "run", "--rm", "-e", "GIT_AUTHOR_NAME=Konstantin Gatilin", "myimage"}
	unit := generateSystemdUnit("myagent", args, "agent", true)
	tests := []struct {
		name     string
		contains string
	}{
		{"no sh -c", "ExecStart=docker"},
		{"quoted env", `"GIT_AUTHOR_NAME=Konstantin Gatilin"`},
		{"image", "myimage"},
	}
	for _, tt := range tests {
		if !containsStr(unit, tt.contains) {
			t.Errorf("systemd unit missing %s: %q not in output:\n%s", tt.name, tt.contains, unit)
		}
	}
	// Must NOT contain /bin/sh -c
	if containsStr(unit, "/bin/sh -c") {
		t.Error("multi-arg systemd unit should not use /bin/sh -c wrapper")
	}
}

func TestGenerateLaunchdPlist_singleArg(t *testing.T) {
	plist := generateLaunchdPlist("myagent", []string{"claude --config-dir ~/.claude"}, "agent", true)
	tests := []struct {
		name     string
		contains string
	}{
		{"label", "com.myhome.myagent"},
		{"command", "claude --config-dir ~/.claude"},
		{"sh wrapper", "<string>/bin/sh</string>"},
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

func TestGenerateLaunchdPlist_multiArg(t *testing.T) {
	args := []string{"docker", "run", "--rm", "-e", "GIT_AUTHOR_NAME=Konstantin Gatilin", "myimage"}
	plist := generateLaunchdPlist("myagent", args, "agent", true)
	tests := []struct {
		name     string
		contains string
	}{
		{"docker arg", "<string>docker</string>"},
		{"run arg", "<string>run</string>"},
		{"env arg", "<string>GIT_AUTHOR_NAME=Konstantin Gatilin</string>"},
		{"image arg", "<string>myimage</string>"},
	}
	for _, tt := range tests {
		if !containsStr(plist, tt.contains) {
			t.Errorf("launchd plist missing %s: %q not in output:\n%s", tt.name, tt.contains, plist)
		}
	}
	// Must NOT contain /bin/sh
	if containsStr(plist, "<string>/bin/sh</string>") {
		t.Error("multi-arg launchd plist should not use /bin/sh wrapper")
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
