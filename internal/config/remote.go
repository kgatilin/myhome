package config

// Remote defines a remote host for SSH + tmux session management.
type Remote struct {
	Host    string `yaml:"host"`              // user@host
	Home    string `yaml:"home"`              // remote home path
	Env     string `yaml:"env"`               // environment tag
	Command string `yaml:"command,omitempty"` // command to run (default: "claude -p")
}

// Schedule defines a recurring task schedule.
type Schedule struct {
	ID        string `yaml:"id"`
	Prompt    string `yaml:"prompt"`
	Cron      string `yaml:"cron"`
	Container string `yaml:"container,omitempty"`
	Auth      string `yaml:"auth,omitempty"`
	Workdir   string `yaml:"workdir,omitempty"`
	Domain    string `yaml:"domain,omitempty"`
}
