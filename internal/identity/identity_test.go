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
		"# myhome-managed-start",
		"name = Personal User",
		"email = me@personal.com",
		"[includeIf \"gitdir:~/work/\"]",
		"path = .gitconfig-work",
		"# myhome-managed-end",
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
	content := string(data)
	if !strings.Contains(content, "name = Default User") {
		t.Error("main gitconfig missing default name")
	}
	if !strings.Contains(content, "# myhome-managed-start") {
		t.Error("missing managed block start marker")
	}
	if !strings.Contains(content, "# myhome-managed-end") {
		t.Error("missing managed block end marker")
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

func TestWriteGitconfigPreservesExistingContent(t *testing.T) {
	homeDir := t.TempDir()
	path := filepath.Join(homeDir, ".gitconfig")

	// Write existing content that should be preserved
	existing := "[alias]\n  co = checkout\n  br = branch\n\n[core]\n  editor = vim\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	overrides := []Identity{
		{Dir: "~/work/", Name: "Work User", Email: "work@corp.com"},
	}
	if err := WriteGitconfig(homeDir, "Default User", "default@email.com", overrides); err != nil {
		t.Fatalf("WriteGitconfig() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Existing content preserved
	if !strings.Contains(content, "[alias]") {
		t.Error("lost existing [alias] section")
	}
	if !strings.Contains(content, "co = checkout") {
		t.Error("lost existing alias content")
	}
	if !strings.Contains(content, "editor = vim") {
		t.Error("lost existing [core] section")
	}

	// Managed block appended
	if !strings.Contains(content, "# myhome-managed-start") {
		t.Error("missing managed block")
	}
	if !strings.Contains(content, "name = Default User") {
		t.Error("missing managed identity")
	}
}

func TestWriteGitconfigUpdatesExistingBlock(t *testing.T) {
	homeDir := t.TempDir()
	path := filepath.Join(homeDir, ".gitconfig")

	// Write a file with an existing managed block
	existing := "[alias]\n  co = checkout\n\n# myhome-managed-start\n[user]\n  name = Old Name\n  email = old@email.com\n# myhome-managed-end\n\n[core]\n  editor = vim\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteGitconfig(homeDir, "New Name", "new@email.com", nil); err != nil {
		t.Fatalf("WriteGitconfig() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Old content replaced
	if strings.Contains(content, "Old Name") {
		t.Error("old managed content not replaced")
	}

	// New content present
	if !strings.Contains(content, "name = New Name") {
		t.Error("new managed content missing")
	}

	// Surrounding content preserved
	if !strings.Contains(content, "[alias]") {
		t.Error("lost content before managed block")
	}
	if !strings.Contains(content, "editor = vim") {
		t.Error("lost content after managed block")
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
