package config

// Container defines a container image and its run configuration.
type Container struct {
	Dockerfile      string            `yaml:"dockerfile"`
	Image           string            `yaml:"image"`
	Firewall        bool              `yaml:"firewall"`
	GitBackup       bool              `yaml:"git_backup"`
	StartupCommands []string          `yaml:"startup_commands,omitempty"`
	Mounts          []string          `yaml:"mounts,omitempty"`
	Volumes         []string          `yaml:"volumes,omitempty"`
	Env             map[string]string `yaml:"env,omitempty"`
	HomeDir         string            `yaml:"home_dir,omitempty"` // container user home, default /home/node
}
