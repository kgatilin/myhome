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
	Agents           map[string]AgentConfig    `yaml:"agents,omitempty"`
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
	PreRun    []PreRunHook    `yaml:"pre_run,omitempty"`
	Workflow  *WorkflowConfig `yaml:"workflow,omitempty"`
}

// PreRunHook defines a pattern-matched command that runs on the host before container launch.
// If the task description matches the pattern, the command runs and its output is appended to the prompt.
type PreRunHook struct {
	Match string `yaml:"match"` // regex pattern to match in task description
	Run   string `yaml:"run"`   // shell command to execute; capture groups available as $1, $2, etc.
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

// ResolvedEnv holds the merged repos/tools/packages for a resolved environment.
type ResolvedEnv struct {
	Name     string
	Repos    []Repo
	Tools    map[string]string
	Packages PackageSet
}
