package packages

import (
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

// mockPlatform implements platform.Platform for testing.
type mockPlatform struct {
	os        string
	installed []string
	pkgMgr    string
}

func (m *mockPlatform) OS() string                                               { return m.os }
func (m *mockPlatform) HomeDir() string                                          { return "/home" }
func (m *mockPlatform) UserHome(username string) string                          { return "/home/" + username }
func (m *mockPlatform) CreateUser(username string) error                         { return nil }
func (m *mockPlatform) RemoveUser(username string, removeHome bool) error        { return nil }
func (m *mockPlatform) CreateGroup(group string) error                           { return nil }
func (m *mockPlatform) AddUserToGroup(username, group string) error              { return nil }
func (m *mockPlatform) SetReadOnlyACL(username, path string) error               { return nil }
func (m *mockPlatform) PackageManager() string                                   { return m.pkgMgr }
func (m *mockPlatform) InstallPackages(packages []string) error                  { return nil }
func (m *mockPlatform) InstallCaskPackages(packages []string) error              { return nil }
func (m *mockPlatform) ListInstalledPackages() ([]string, error)                 { return m.installed, nil }
func (m *mockPlatform) ServiceInstall(name string, args []string, username string, restart bool) error { return nil }
func (m *mockPlatform) ServiceStart(name string) error                           { return nil }
func (m *mockPlatform) ServiceStop(name string) error                            { return nil }
func (m *mockPlatform) ServiceStatus(name string) (bool, error)                  { return false, nil }

func TestList(t *testing.T) {
	plat := &mockPlatform{
		os:        "linux",
		installed: []string{"git", "jq", "curl"},
		pkgMgr:    "apt",
	}
	expected := config.PackageSet{
		Apt: []string{"git", "gh", "jq"},
	}

	statuses, err := List(expected, plat)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(statuses) != 3 {
		t.Fatalf("got %d statuses, want 3", len(statuses))
	}

	// Find gh — should be not installed
	for _, s := range statuses {
		if s.Name == "gh" && s.Installed {
			t.Error("gh should not be installed")
		}
		if s.Name == "git" && !s.Installed {
			t.Error("git should be installed")
		}
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"git\njq\ncurl\n", 3},
		{"", 0},
		{"single", 1},
		{"\n\ngit\n\n", 1},
	}
	for _, tt := range tests {
		got := splitLines(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitLines(%q) = %d items, want %d", tt.input, len(got), tt.want)
		}
	}
}
