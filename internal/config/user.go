package config

// AgentTemplate defines a template for creating agent users.
type AgentTemplate struct {
	TemplateRepo string        `yaml:"template_repo"`
	Service      ServiceConfig `yaml:"service"`
}

// ServiceConfig defines how an agent service runs.
type ServiceConfig struct {
	Command   string `yaml:"command"`
	Restart   string `yaml:"restart"`
	DependsOn string `yaml:"depends_on,omitempty"`
}

// ServicesConfig defines managed system services for the agent stack.
type ServicesConfig struct {
	Deskd    *ServiceConfig            `yaml:"deskd,omitempty"`
	Agents   map[string]*ServiceConfig `yaml:"agents,omitempty"`
	Adapters map[string]*ServiceConfig `yaml:"adapters,omitempty"`
}

// User defines an agent user.
type User struct {
	Env      string `yaml:"env"`
	Template string `yaml:"template"`
}
