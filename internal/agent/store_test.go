package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	state := &State{
		Name:      "test-agent",
		Status:    StatusRunning,
		Container: "claude-personal",
		CreatedAt: time.Now().Truncate(time.Second),
		ContainerID: "abc123def456",
		NumTurns:  5,
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load("test-agent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Name != "test-agent" {
		t.Errorf("Name = %q, want %q", loaded.Name, "test-agent")
	}
	if loaded.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", loaded.Status, StatusRunning)
	}
	if loaded.ContainerID != "abc123def456" {
		t.Errorf("ContainerID = %q, want %q", loaded.ContainerID, "abc123def456")
	}
	if loaded.NumTurns != 5 {
		t.Errorf("NumTurns = %d, want 5", loaded.NumTurns)
	}
}

func TestStoreList(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	agents := []string{"alpha", "beta", "gamma"}
	for _, name := range agents {
		state := &State{
			Name:      name,
			Status:    StatusRunning,
			Container: "test",
			CreatedAt: time.Now(),
		}
		if err := store.Save(state); err != nil {
			t.Fatalf("Save %s: %v", name, err)
		}
	}

	states, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(states) != 3 {
		t.Errorf("List returned %d states, want 3", len(states))
	}
}

func TestStoreRemove(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	state := &State{
		Name:      "to-remove",
		Status:    StatusStopped,
		Container: "test",
		CreatedAt: time.Now(),
	}
	if err := store.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Remove("to-remove"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	_, err = store.Load("to-remove")
	if err == nil {
		t.Error("Load after Remove should return error")
	}
}

func TestStoreLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, err = store.Load("nonexistent")
	if err == nil {
		t.Error("Load nonexistent should return error")
	}
}

func TestNewStoreCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")

	store, err := NewStore(agentsDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Check logs subdirectory was created
	logDir := store.LogDir()
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("LogDir %s was not created", logDir)
	}
}
