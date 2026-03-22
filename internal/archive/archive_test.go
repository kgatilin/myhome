package archive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

func TestMove(t *testing.T) {
	homeDir := t.TempDir()

	// Create source dir
	srcPath := filepath.Join(homeDir, "old-project")
	os.MkdirAll(srcPath, 0o755)
	os.WriteFile(filepath.Join(srcPath, "file.txt"), []byte("data"), 0o644)

	cfg := &config.Config{}

	if err := Move("old-project", homeDir, cfg); err != nil {
		t.Fatalf("Move() error: %v", err)
	}

	// Source should not exist
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("source should not exist after move")
	}

	// Destination should exist
	destPath := filepath.Join(homeDir, "archive", "old-project")
	if _, err := os.Stat(destPath); err != nil {
		t.Error("destination should exist after move")
	}

	// File inside should still be there
	data, err := os.ReadFile(filepath.Join(destPath, "file.txt"))
	if err != nil || string(data) != "data" {
		t.Error("file contents should be preserved")
	}

	// .gitignore should be regenerated
	if _, err := os.Stat(filepath.Join(homeDir, ".gitignore")); err != nil {
		t.Error(".gitignore should be regenerated")
	}
}

func TestMoveNonexistent(t *testing.T) {
	homeDir := t.TempDir()
	cfg := &config.Config{}
	err := Move("nonexistent", homeDir, cfg)
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestMoveDuplicate(t *testing.T) {
	homeDir := t.TempDir()

	// Create source and existing archive entry
	os.MkdirAll(filepath.Join(homeDir, "project"), 0o755)
	os.MkdirAll(filepath.Join(homeDir, "archive", "project"), 0o755)

	cfg := &config.Config{}
	err := Move("project", homeDir, cfg)
	if err == nil {
		t.Error("expected error when destination already exists")
	}
}
