package config

// Config represents the full myhome.yml configuration.
type Config struct {
	Envs             map[string]Env            `yaml:"envs"`
	Repos            []Repo                    `yaml:"repos"`
	Tools            map[string]map[string]string `yaml:"tools"`
	Packages         map[string]PackageSet     `yaml:"packages"`
	Auth             map[string]AuthHost       `yaml:"auth"`
	AgentTemplates   map[string]AgentTemplate  `yaml:"agent_templates"`
	Users            map[string]User           `yaml:"users"`
	ContainerRuntime string                    `yaml:"container_runtime"`
	Containers       map[string]Container      `yaml:"containers"`
	Claude           ClaudeConfig              `yaml:"claude"`
	Tasks            TasksConfig               `yaml:"tasks"`
	Remotes          map[string]Remote         `yaml:"remotes,omitempty"`
	Schedules        []Schedule                `yaml:"schedules,omitempty"`
}

// Env defines an environment profile with included environment tags.
type Env struct {
	Include []string `yaml:"include"`
}

// Repo defines a git repository to manage.
type Repo struct {
	Path      string          `yaml:"path"`
	URL       string          `yaml:"url"`
	Env       string          `yaml:"env"`
	Container string          `yaml:"container,omitempty"`
	Worktrees *WorktreeConfig `yaml:"worktrees,omitempty"`
}

// WorktreeConfig configures worktree behavior for a repo.
type WorktreeConfig struct {
	Dir           string `yaml:"dir"`
	DefaultBranch string `yaml:"default_branch"`
}

// PackageSet groups system packages by package manager.
type PackageSet struct {
	Brew     []string `yaml:"brew,omitempty"`
	BrewCask []string `yaml:"brew_cask,omitempty"`
	Apt      []string `yaml:"apt,omitempty"`
}

// AuthHost configures SSH auth for a host.
type AuthHost struct {
	Key string `yaml:"key"`
}

// AgentTemplate defines a template for creating agent users.
type AgentTemplate struct {
	TemplateRepo string        `yaml:"template_repo"`
	Service      ServiceConfig `yaml:"service"`
}

// ServiceConfig defines how an agent service runs.
type ServiceConfig struct {
	Command string `yaml:"command"`
	Restart string `yaml:"restart"`
}

// User defines an agent user.
type User struct {
	Env      string `yaml:"env"`
	Template string `yaml:"template"`
}

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

// ClaudeConfig holds Claude-specific settings.
type ClaudeConfig struct {
	ConfigDir    string                  `yaml:"config_dir"`
	AuthProfiles map[string]AuthProfile `yaml:"auth_profiles"`
}

// AuthProfile defines auth credentials for a Claude profile.
type AuthProfile struct {
	AuthFile string            `yaml:"auth_file"`
	Env      map[string]string `yaml:"env,omitempty"`
}

// TasksConfig holds task-related settings.
type TasksConfig struct {
	Dir           string              `yaml:"dir,omitempty"` // defaults to ~/tasks
	Notifications NotificationsConfig `yaml:"notifications,omitempty"`
}

// NotificationsConfig controls desktop notifications for task completion.
type NotificationsConfig struct {
	Enabled *bool `yaml:"enabled,omitempty"` // defaults to true on macOS
}

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

// ResolvedEnv holds the merged repos/tools/packages for a resolved environment.
type ResolvedEnv struct {
	Name     string
	Repos    []Repo
	Tools    map[string]string
	Packages PackageSet
}
