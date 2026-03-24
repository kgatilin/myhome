package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

func TestList(t *testing.T) {
	homeDir := t.TempDir()

	// Create a cloned repo (with .git dir)
	clonedPath := filepath.Join(homeDir, "dev/tools/myhome")
	os.MkdirAll(filepath.Join(clonedPath, ".git"), 0o755)

	env := &config.ResolvedEnv{
		Repos: []config.Repo{
			{Path: "dev/tools/myhome", URL: "git@github.com:user/myhome.git", Env: "base"},
			{Path: "work/uagent", URL: "git@gitlab.com:org/uagent.git", Env: "work"},
		},
	}

	statuses, err := List(env, homeDir)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2", len(statuses))
	}
	if !statuses[0].Cloned {
		t.Error("myhome should be cloned")
	}
	if statuses[1].Cloned {
		t.Error("uagent should not be cloned")
	}
}

func TestSync(t *testing.T) {
	homeDir := t.TempDir()

	// Create a bare repo to clone from
	bareRepo := filepath.Join(t.TempDir(), "origin.git")
	if err := exec.Command("git", "init", "--bare", bareRepo).Run(); err != nil {
		t.Fatalf("create bare repo: %v", err)
	}

	env := &config.ResolvedEnv{
		Repos: []config.Repo{
			{Path: "test/repo", URL: bareRepo, Env: "base"},
		},
	}

	if err := Sync(env, homeDir); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	// Verify it was cloned
	if !isGitRepo(filepath.Join(homeDir, "test/repo")) {
		t.Error("repo should be cloned after sync")
	}

	// Sync again — should be a no-op
	if err := Sync(env, homeDir); err != nil {
		t.Fatalf("Sync() second call error: %v", err)
	}
}

func TestSyncContinuesOnError(t *testing.T) {
	homeDir := t.TempDir()

	// Create a bare repo that can actually be cloned
	bareRepo := filepath.Join(t.TempDir(), "good.git")
	if err := exec.Command("git", "init", "--bare", bareRepo).Run(); err != nil {
		t.Fatalf("create bare repo: %v", err)
	}

	env := &config.ResolvedEnv{
		Repos: []config.Repo{
			{Path: "bad/repo", URL: "/nonexistent/path.git", Env: "base"},
			{Path: "good/repo", URL: bareRepo, Env: "base"},
		},
	}

	err := Sync(env, homeDir)
	if err == nil {
		t.Fatal("Sync() should return error for failed clone")
	}
	if !strings.Contains(err.Error(), "bad/repo") {
		t.Errorf("error should mention bad/repo, got: %v", err)
	}
	// Good repo should still be cloned despite the earlier failure
	if !isGitRepo(filepath.Join(homeDir, "good/repo")) {
		t.Error("good/repo should be cloned even when bad/repo fails")
	}
}

func TestAdd(t *testing.T) {
	cfg := &config.Config{}
	if err := Add(cfg, "dev/new-repo", "git@github.com:user/new.git", "personal", "/home/user"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Fatalf("got %d repos, want 1", len(cfg.Repos))
	}
	if cfg.Repos[0].Path != "dev/new-repo" {
		t.Errorf("path = %q, want dev/new-repo", cfg.Repos[0].Path)
	}
	if cfg.Repos[0].Env != "personal" {
		t.Errorf("env = %q, want personal", cfg.Repos[0].Env)
	}

	// Duplicate should fail
	err := Add(cfg, "dev/new-repo", "git@github.com:user/new.git", "personal", "/home/user")
	if err == nil {
		t.Error("expected error on duplicate add")
	}
}

func TestAddDefaultEnv(t *testing.T) {
	cfg := &config.Config{}
	Add(cfg, "dev/test", "git@github.com:user/test.git", "", "/home/user")
	if cfg.Repos[0].Env != "base" {
		t.Errorf("default env = %q, want base", cfg.Repos[0].Env)
	}
}

func TestRm(t *testing.T) {
	cfg := &config.Config{
		Repos: []config.Repo{
			{Path: "dev/tools/myhome", URL: "git@github.com:user/myhome.git", Env: "base"},
			{Path: "work/uagent", URL: "git@gitlab.com:org/uagent.git", Env: "work"},
		},
	}

	if err := Rm(cfg, "work/uagent"); err != nil {
		t.Fatalf("Rm() error: %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Fatalf("got %d repos, want 1", len(cfg.Repos))
	}
	if cfg.Repos[0].Path != "dev/tools/myhome" {
		t.Errorf("remaining repo = %q, want dev/tools/myhome", cfg.Repos[0].Path)
	}

	// Remove nonexistent
	if err := Rm(cfg, "nonexistent"); err == nil {
		t.Error("expected error on nonexistent rm")
	}
}

func TestFindByName(t *testing.T) {
	repos := []config.Repo{
		{Path: "dev/tools/myhome", URL: "url1", Env: "base"},
		{Path: "work/uagent", URL: "url2", Env: "work"},
	}

	// Find by basename
	r, err := FindByName(repos, "uagent")
	if err != nil {
		t.Fatalf("FindByName(uagent) error: %v", err)
	}
	if r.Path != "work/uagent" {
		t.Errorf("found %q, want work/uagent", r.Path)
	}

	// Find by full path
	r, err = FindByName(repos, "dev/tools/myhome")
	if err != nil {
		t.Fatalf("FindByName(full path) error: %v", err)
	}
	if r.Path != "dev/tools/myhome" {
		t.Errorf("found %q, want dev/tools/myhome", r.Path)
	}

	// Not found
	_, err = FindByName(repos, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent repo")
	}
}

func TestFindByNameAmbiguous(t *testing.T) {
	repos := []config.Repo{
		{Path: "dev/tools/mylib", Env: "base"},
		{Path: "work/mylib", Env: "work"},
	}
	_, err := FindByName(repos, "mylib")
	if err == nil {
		t.Error("expected ambiguous error")
	}
}

func TestAddDetectURL(t *testing.T) {
	// Create a real git repo with a remote
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "myrepo")
	cmds := [][]string{
		{"git", "init", repoPath},
		{"git", "-C", repoPath, "remote", "add", "origin", "git@github.com:user/detected.git"},
	}
	for _, c := range cmds {
		if err := exec.Command(c[0], c[1:]...).Run(); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	cfg := &config.Config{}
	if err := Add(cfg, "myrepo", "", "base", dir); err != nil {
		t.Fatalf("Add() with auto-detect error: %v", err)
	}
	if cfg.Repos[0].URL != "git@github.com:user/detected.git" {
		t.Errorf("detected URL = %q, want git@github.com:user/detected.git", cfg.Repos[0].URL)
	}
}
