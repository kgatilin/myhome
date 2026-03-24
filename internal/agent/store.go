package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Store manages agent state persistence in ~/.myhome/agents/.
type Store struct {
	dir string
}

// NewStore creates a Store rooted at the given directory and ensures it exists.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating agents directory: %w", err)
	}
	// Ensure logs subdirectory exists
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		return nil, fmt.Errorf("creating agents/logs directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Save writes agent state to a YAML file.
func (s *Store) Save(state *State) error {
	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling agent %s: %w", state.Name, err)
	}
	path := s.path(state.Name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing agent %s state: %w", state.Name, err)
	}
	return nil
}

// Load reads agent state by name.
func (s *Store) Load(name string) (*State, error) {
	data, err := os.ReadFile(s.path(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("agent %q not found", name)
		}
		return nil, fmt.Errorf("reading agent %s state: %w", name, err)
	}
	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing agent %s state: %w", name, err)
	}
	return &state, nil
}

// List returns all persisted agent states.
func (s *Store) List() ([]*State, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("reading agents directory: %w", err)
	}
	var states []*State
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var state State
		if err := yaml.Unmarshal(data, &state); err != nil {
			continue
		}
		states = append(states, &state)
	}
	return states, nil
}

// Remove deletes an agent's state file.
func (s *Store) Remove(name string) error {
	path := s.path(name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("agent %q not found", name)
		}
		return fmt.Errorf("removing agent %s state: %w", name, err)
	}
	return nil
}

// LogDir returns the path to the agent logs directory.
func (s *Store) LogDir() string {
	return filepath.Join(s.dir, "logs")
}

func (s *Store) path(name string) string {
	return filepath.Join(s.dir, name+".yml")
}
