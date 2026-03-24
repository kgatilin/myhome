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
	TaskStatusFailed  TaskStatus = "failed"
)

// StageStatus represents the state of a workflow stage.
type StageStatus string

const (
	StageStatusPending  StageStatus = "pending"
	StageStatusRunning  StageStatus = "running"
	StageStatusComplete StageStatus = "complete"
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
	Iterations   int    `yaml:"iterations,omitempty"`
	ExitCode     *int   `yaml:"exit_code,omitempty"`
	// Workflow fields
	Stage          string            `yaml:"stage,omitempty"`           // current workflow stage name
	StageStatus    StageStatus       `yaml:"stage_status,omitempty"`    // pending, running, complete
	WorkflowParams map[string]string `yaml:"workflow_params,omitempty"` // params for prompt interpolation
}
