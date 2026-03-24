package config

// AgentConfig defines the configuration for a persistent agent instance.
type AgentConfig struct {
	Container    string            `yaml:"container"`
	Model        string            `yaml:"model,omitempty"`
	Mounts       []string          `yaml:"mounts,omitempty"`
	SystemPrompt string            `yaml:"system_prompt,omitempty"`
	AllowedTools []string          `yaml:"allowed_tools,omitempty"`
	MaxBudgetUSD float64           `yaml:"max_budget_usd,omitempty"`
	MaxTurns     int               `yaml:"max_turns,omitempty"`
	Identity     AgentIdentity     `yaml:"identity,omitempty"`
	Secrets      AgentSecrets      `yaml:"secrets,omitempty"`
	Env          map[string]string `yaml:"env,omitempty"`
}

// AgentIdentity defines the identity metadata for an agent (git config, SSH key).
type AgentIdentity struct {
	Git AgentGitIdentity `yaml:"git,omitempty"`
	SSH string           `yaml:"ssh,omitempty"` // SSH auth key name from auth section
}

// AgentGitIdentity configures git user for an agent.
type AgentGitIdentity struct {
	Name  string `yaml:"name,omitempty"`
	Email string `yaml:"email,omitempty"`
}

// AgentSecrets defines secret references for an agent (vault entries, etc.).
type AgentSecrets struct {
	Vault []string `yaml:"vault,omitempty"` // e.g. vault://gitlab-token-work
}
