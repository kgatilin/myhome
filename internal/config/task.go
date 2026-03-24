package config

// TasksConfig holds task-related settings.
type TasksConfig struct {
	Dir           string              `yaml:"dir,omitempty"` // defaults to ~/tasks
	Notifications NotificationsConfig `yaml:"notifications,omitempty"`
	TaskSuffix    string              `yaml:"task_suffix,omitempty"` // appended to every task prompt
}

// NotificationsConfig controls desktop notifications for task completion.
type NotificationsConfig struct {
	Enabled *bool `yaml:"enabled,omitempty"` // defaults to true on macOS
}
