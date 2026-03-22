package identity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateGitconfig(t *testing.T) {
	overrides := []Identity{
		{Dir: "~/work/", Name: "Work User", Email: "work@corp.com"},
	}
	content := GenerateGitconfig("Personal User", "me@personal.com", overrides)

	checks := []string{
		"name = Personal User",
		"email = me@personal.com",
		"[includeIf \"gitdir:~/work/\"]",
		"path = .gitconfig-work",
	}
	for _, c := range checks {
		if !strings.Contains(content, c) {
			t.Errorf("missing: %s", c)
		}
	}
}

func TestGenerateIdentityFile(t *testing.T) {
	id := Identity{Dir: "~/work/", Name: "Work User", Email: "work@corp.com"}
	content := GenerateIdentityFile(id)

	if !strings.Contains(content, "name = Work User") {
		t.Error("missing name")
	}
	if !strings.Contains(content, "email = work@corp.com") {
		t.Error("missing email")
	}
}

func TestWriteGitconfig(t *testing.T) {
	homeDir := t.TempDir()
	overrides := []Identity{
		{Dir: "~/work/", Name: "Work User", Email: "work@corp.com"},
	}

	if err := WriteGitconfig(homeDir, "Default User", "default@email.com", overrides); err != nil {
		t.Fatalf("WriteGitconfig() error: %v", err)
	}

	// Check main .gitconfig
	data, err := os.ReadFile(filepath.Join(homeDir, ".gitconfig"))
	if err != nil {
		t.Fatalf("read .gitconfig: %v", err)
	}
	if !strings.Contains(string(data), "name = Default User") {
		t.Error("main gitconfig missing default name")
	}

	// Check override file
	data, err = os.ReadFile(filepath.Join(homeDir, ".gitconfig-work"))
	if err != nil {
		t.Fatalf("read identity file: %v", err)
	}
	if !strings.Contains(string(data), "name = Work User") {
		t.Error("identity file missing work name")
	}
}

func TestGitconfigPath(t *testing.T) {
	tests := []struct {
		dir  string
		want string
	}{
		{"~/work/", ".gitconfig-work"},
		{"~/dev/personal/", ".gitconfig-dev-personal"},
		{"work", ".gitconfig-work"},
	}
	for _, tt := range tests {
		got := gitconfigPath(tt.dir)
		if got != tt.want {
			t.Errorf("gitconfigPath(%q) = %q, want %q", tt.dir, got, tt.want)
		}
	}
}
