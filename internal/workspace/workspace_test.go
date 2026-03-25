package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

var testRepos = []config.Repo{
	{Path: "dev/tools/myhome", URL: "git@github.com:kgatilin/myhome.git", Env: "base"},
	{Path: "work/uagent", URL: "git@gitlab.example.com:team/project/repo.git", Env: "work"},
	{Path: "work/infra", URL: "git@gitlab.example.com:team/infra/repo.git", Env: "work"},
	{Path: "dev/tools/go-arch-lint", URL: "git@github.com:kgatilin/go-arch-lint.git", Env: "personal"},
	{Path: "life/notes", URL: "git@github.com:kgatilin/notes.git", Env: "personal"},
}

func TestGroupByDomain(t *testing.T) {
	groups := GroupByDomain(testRepos)

	tests := []struct {
		domain string
		count  int
	}{
		{"dev", 2},
		{"work", 2},
		{"life", 1},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			repos := groups[tt.domain]
			if len(repos) != tt.count {
				t.Errorf("got %d repos, want %d", len(repos), tt.count)
			}
		})
	}

	if _, ok := groups["other"]; ok {
		t.Error("unexpected 'other' domain")
	}
}

func TestGroupByDomainSorted(t *testing.T) {
	groups := GroupByDomain(testRepos)
	devRepos := groups["dev"]
	if len(devRepos) < 2 {
		t.Fatal("expected at least 2 dev repos")
	}
	if devRepos[0].Path >= devRepos[1].Path {
		t.Errorf("repos not sorted: %s >= %s", devRepos[0].Path, devRepos[1].Path)
	}
}

func TestDomainOfUnknown(t *testing.T) {
	got := domainOf("random/repo")
	if got != "other" {
		t.Errorf("got %q, want %q", got, "other")
	}
}

func TestGenerateAll(t *testing.T) {
	cfg := &config.Config{Repos: testRepos}
	files, err := GenerateAll(cfg)
	if err != nil {
		t.Fatalf("GenerateAll failed: %v", err)
	}

	expectedFiles := []string{"home.code-workspace", "work.code-workspace", "dev.code-workspace", "life.code-workspace"}
	for _, name := range expectedFiles {
		data, ok := files[name]
		if !ok {
			t.Errorf("missing file: %s", name)
			continue
		}

		var ws WorkspaceFile
		if err := json.Unmarshal(data, &ws); err != nil {
			t.Errorf("invalid JSON in %s: %v", name, err)
			continue
		}
		if len(ws.Folders) == 0 {
			t.Errorf("%s has no folders", name)
		}
		if ws.Settings == nil {
			t.Errorf("%s has no settings", name)
		}
	}

	// Root should have all repos
	var root WorkspaceFile
	json.Unmarshal(files["home.code-workspace"], &root)
	if len(root.Folders) != len(testRepos) {
		t.Errorf("root workspace: got %d folders, want %d", len(root.Folders), len(testRepos))
	}

	// Work should have 2 repos
	var work WorkspaceFile
	json.Unmarshal(files["work.code-workspace"], &work)
	if len(work.Folders) != 2 {
		t.Errorf("work workspace: got %d folders, want 2", len(work.Folders))
	}
}

func TestGenerateAllNoRepos(t *testing.T) {
	cfg := &config.Config{}
	files, err := GenerateAll(cfg)
	if err != nil {
		t.Fatalf("GenerateAll failed: %v", err)
	}
	// Should still have the root workspace (empty folders)
	if _, ok := files["home.code-workspace"]; !ok {
		t.Error("missing root workspace")
	}
	// Should not have domain workspaces
	if len(files) != 1 {
		t.Errorf("got %d files, want 1 (root only)", len(files))
	}
}

func TestWorkspaceFoldersSorted(t *testing.T) {
	cfg := &config.Config{Repos: testRepos}
	files, err := GenerateAll(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var root WorkspaceFile
	json.Unmarshal(files["home.code-workspace"], &root)

	for i := 1; i < len(root.Folders); i++ {
		if root.Folders[i-1].Path >= root.Folders[i].Path {
			t.Errorf("folders not sorted: %s >= %s", root.Folders[i-1].Path, root.Folders[i].Path)
		}
	}
}

func TestWriteAll(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Repos: testRepos}

	if err := WriteAll(cfg, dir); err != nil {
		t.Fatalf("WriteAll failed: %v", err)
	}

	expectedFiles := []string{"home.code-workspace", "work.code-workspace", "dev.code-workspace", "life.code-workspace"}
	for _, name := range expectedFiles {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		var ws WorkspaceFile
		if err := json.Unmarshal(data, &ws); err != nil {
			t.Errorf("invalid JSON in %s: %v", name, err)
		}
	}
}

func TestWorkspaceHasSettings(t *testing.T) {
	cfg := &config.Config{Repos: testRepos}
	files, err := GenerateAll(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var ws WorkspaceFile
	json.Unmarshal(files["home.code-workspace"], &ws)

	expectedKeys := []string{
		"go.useLanguageServer",
		"git.autofetch",
		"editor.formatOnSave",
	}
	for _, key := range expectedKeys {
		if _, ok := ws.Settings[key]; !ok {
			t.Errorf("missing setting: %s", key)
		}
	}
}
