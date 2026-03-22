package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ListFilter controls which tasks are returned by List.
type ListFilter struct {
	Status TaskStatus
	Domain string
}

// Store manages task persistence as YAML files on disk.
type Store struct {
	tasksDir string
}

// NewStore creates a Store rooted at the given directory.
// It ensures the active/, done/, and logs/ subdirectories exist.
func NewStore(tasksDir string) (*Store, error) {
	for _, sub := range []string{"active", "done", "logs"} {
		if err := os.MkdirAll(filepath.Join(tasksDir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("creating %s directory: %w", sub, err)
		}
	}
	return &Store{tasksDir: tasksDir}, nil
}

// NextID scans active and done directories and returns max(id)+1.
func (s *Store) NextID() (int, error) {
	maxID := 0
	for _, sub := range []string{"active", "done"} {
		dir := filepath.Join(s.tasksDir, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return 0, fmt.Errorf("reading %s directory: %w", sub, err)
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".yml") {
				continue
			}
			idStr := strings.TrimSuffix(name, ".yml")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				continue
			}
			if id > maxID {
				maxID = id
			}
		}
	}
	return maxID + 1, nil
}

// Save writes a task to the appropriate directory based on its status.
func (s *Store) Save(task *Task) error {
	data, err := yaml.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshaling task %d: %w", task.ID, err)
	}

	dir := s.dirForStatus(task.Status)
	path := filepath.Join(dir, fmt.Sprintf("%d.yml", task.ID))

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing task %d: %w", task.ID, err)
	}
	return nil
}

// Load reads a task by ID, checking active/ first then done/.
func (s *Store) Load(id int) (*Task, error) {
	filename := fmt.Sprintf("%d.yml", id)

	for _, sub := range []string{"active", "done"} {
		path := filepath.Join(s.tasksDir, sub, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading task %d from %s: %w", id, sub, err)
		}
		var t Task
		if err := yaml.Unmarshal(data, &t); err != nil {
			return nil, fmt.Errorf("parsing task %d: %w", id, err)
		}
		return &t, nil
	}
	return nil, fmt.Errorf("task %d not found", id)
}

// List returns tasks matching the given filter.
func (s *Store) List(filter ListFilter) ([]*Task, error) {
	var tasks []*Task

	dirs := []string{"active", "done"}
	for _, sub := range dirs {
		dir := filepath.Join(s.tasksDir, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("reading %s directory: %w", sub, err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".yml") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return nil, fmt.Errorf("reading task file %s: %w", e.Name(), err)
			}
			var t Task
			if err := yaml.Unmarshal(data, &t); err != nil {
				return nil, fmt.Errorf("parsing task file %s: %w", e.Name(), err)
			}
			if filter.Status != "" && t.Status != filter.Status {
				continue
			}
			if filter.Domain != "" && t.Domain != filter.Domain {
				continue
			}
			tasks = append(tasks, &t)
		}
	}
	return tasks, nil
}

// MarkDone transitions a task to done status and moves it from active/ to done/.
func (s *Store) MarkDone(id int) error {
	task, err := s.Load(id)
	if err != nil {
		return fmt.Errorf("loading task for mark done: %w", err)
	}

	// Remove from current location
	oldPath := filepath.Join(s.tasksDir, "active", fmt.Sprintf("%d.yml", id))

	now := time.Now()
	task.Status = TaskStatusDone
	task.DoneAt = &now

	if err := s.Save(task); err != nil {
		return fmt.Errorf("saving done task: %w", err)
	}

	// Remove old file if it existed in active (ignore error if already in done)
	os.Remove(oldPath)
	return nil
}

// Remove deletes a task file from both active/ and done/.
func (s *Store) Remove(id int) error {
	filename := fmt.Sprintf("%d.yml", id)
	removed := false
	for _, sub := range []string{"active", "done"} {
		path := filepath.Join(s.tasksDir, sub, filename)
		err := os.Remove(path)
		if err == nil {
			removed = true
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("removing task %d from %s: %w", id, sub, err)
		}
	}
	if !removed {
		return fmt.Errorf("task %d not found", id)
	}
	return nil
}

// LogDir returns the path to the logs directory.
func (s *Store) LogDir() string {
	return filepath.Join(s.tasksDir, "logs")
}

// dirForStatus returns the directory path for the given task status.
func (s *Store) dirForStatus(status TaskStatus) string {
	if status == TaskStatusDone {
		return filepath.Join(s.tasksDir, "done")
	}
	return filepath.Join(s.tasksDir, "active")
}
