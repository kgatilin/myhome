package task

import "time"

// TaskType distinguishes between general tasks and dev run tasks.
type TaskType string

const (
	TaskTypeGeneral TaskType = "general"
	TaskTypeRun     TaskType = "run"
)

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	TaskStatusOpen    TaskStatus = "open"
	TaskStatusRunning TaskStatus = "running"
	TaskStatusDone    TaskStatus = "done"
)

// Task represents a tracked unit of work.
type Task struct {
	ID          int        `yaml:"id"`
	Type        TaskType   `yaml:"type"`
	Domain      string     `yaml:"domain,omitempty"`
	Description string     `yaml:"description"`
	Status      TaskStatus `yaml:"status"`
	CreatedAt   time.Time  `yaml:"created_at"`
	DoneAt      *time.Time `yaml:"done_at,omitempty"`
	// Run task fields
	Repo         string `yaml:"repo,omitempty"`
	Branch       string `yaml:"branch,omitempty"`
	ContainerID  string `yaml:"container_id,omitempty"`
	Container    string `yaml:"container,omitempty"`
	AuthProfile  string `yaml:"auth_profile,omitempty"`
	WorktreePath string `yaml:"worktree_path,omitempty"`
	LogFile      string `yaml:"log_file,omitempty"`
}
