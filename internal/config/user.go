package config

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
