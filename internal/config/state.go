package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// State represents the myhome runtime state file (~/.myhome-state.yml).
type State struct {
	CurrentEnv   string               `yaml:"current_env"`
	LastSync     map[string]time.Time `yaml:"last_sync,omitempty"`
	Users        []string             `yaml:"users,omitempty"`
	BuildCommits map[string]string    `yaml:"build_commits,omitempty"`
}

// GetBuildCommit returns the last built commit for a repo path.
func (s *State) GetBuildCommit(repoPath string) string {
	if s.BuildCommits == nil {
		return ""
	}
	return s.BuildCommits[repoPath]
}

// SetBuildCommit records the commit hash that was last built for a repo.
func (s *State) SetBuildCommit(repoPath, commit string) {
	if s.BuildCommits == nil {
		s.BuildCommits = make(map[string]string)
	}
	s.BuildCommits[repoPath] = commit
}

// DefaultStatePath returns ~/.myhome-state.yml.
func DefaultStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".myhome-state.yml"), nil
}

// LoadState reads the state file. Returns empty state if the file doesn't exist.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{LastSync: make(map[string]time.Time)}, nil
		}
		return nil, fmt.Errorf("read state %s: %w", path, err)
	}
	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	if state.LastSync == nil {
		state.LastSync = make(map[string]time.Time)
	}
	return &state, nil
}

// Save writes the state file.
func (s *State) Save(path string) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write state %s: %w", path, err)
	}
	return nil
}

// SetSynced records the current time for a sync operation.
func (s *State) SetSynced(key string) {
	s.LastSync[key] = time.Now()
}
