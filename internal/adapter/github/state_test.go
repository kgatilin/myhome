package github

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewStateStore(dir)

	state, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(state.PostedIssues) != 0 {
		t.Errorf("expected empty map, got %d entries", len(state.PostedIssues))
	}
}

func TestStateStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewStateStore(dir)

	now := time.Now().Truncate(time.Second)
	state := &AdapterState{
		PostedIssues: map[string]PostedIssue{
			"kgatilin/home#42": {
				PostedAt: now,
				Title:    "Fix the thing",
			},
			"kgatilin/myhome#7": {
				PostedAt: now,
				Title:    "Add feature",
			},
		},
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists.
	path := filepath.Join(dir, "github-adapter.yml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(loaded.PostedIssues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(loaded.PostedIssues))
	}

	issue := loaded.PostedIssues["kgatilin/home#42"]
	if issue.Title != "Fix the thing" {
		t.Errorf("expected title 'Fix the thing', got %q", issue.Title)
	}
}

func TestStateStore_CreatesDirIfNeeded(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	store := NewStateStore(dir)

	state := &AdapterState{PostedIssues: map[string]PostedIssue{}}
	if err := store.Save(state); err != nil {
		t.Fatalf("Save() should create dirs: %v", err)
	}
}

func TestIssueKey(t *testing.T) {
	tests := []struct {
		repo   string
		number int
		want   string
	}{
		{"kgatilin/home", 42, "kgatilin/home#42"},
		{"kgatilin/myhome", 1, "kgatilin/myhome#1"},
	}

	for _, tt := range tests {
		got := IssueKey(tt.repo, tt.number)
		if got != tt.want {
			t.Errorf("IssueKey(%q, %d) = %q, want %q", tt.repo, tt.number, got, tt.want)
		}
	}
}
