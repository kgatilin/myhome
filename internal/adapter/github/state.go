package github

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// AdapterState tracks which issues have been posted to the bus.
type AdapterState struct {
	PostedIssues map[string]PostedIssue `yaml:"posted_issues"` // key: "owner/repo#number"
}

// PostedIssue records when an issue was posted.
type PostedIssue struct {
	PostedAt time.Time `yaml:"posted_at"`
	Title    string    `yaml:"title"`
}

// StateStore persists adapter state to disk.
type StateStore struct {
	path string
}

// NewStateStore creates a state store at the given directory.
func NewStateStore(dir string) *StateStore {
	return &StateStore{path: filepath.Join(dir, "github-adapter.yml")}
}

// DefaultStateDir returns ~/.myhome/adapter-state/.
func DefaultStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".myhome", "adapter-state"), nil
}

// Load reads the state from disk. Returns empty state if file doesn't exist.
func (s *StateStore) Load() (*AdapterState, error) {
	state := &AdapterState{PostedIssues: make(map[string]PostedIssue)}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return nil, fmt.Errorf("read adapter state: %w", err)
	}

	if err := yaml.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse adapter state: %w", err)
	}
	if state.PostedIssues == nil {
		state.PostedIssues = make(map[string]PostedIssue)
	}
	return state, nil
}

// Save writes the state to disk.
func (s *StateStore) Save(state *AdapterState) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal adapter state: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("write adapter state: %w", err)
	}
	return nil
}

// IssueKey returns the state key for an issue.
func IssueKey(repo string, number int) string {
	return fmt.Sprintf("%s#%d", repo, number)
}
