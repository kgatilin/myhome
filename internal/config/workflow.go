package config

// WorkflowConfig defines a multi-stage pipeline for a repo.
// Each stage runs a prompt/skill in a container and stops for human review.
type WorkflowConfig struct {
	Params []WorkflowParam `yaml:"params,omitempty"`
	Stages []WorkflowStage `yaml:"stages"`
}

// WorkflowParam defines a named parameter that can be interpolated into stage prompts.
type WorkflowParam struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required,omitempty"`
}

// WorkflowStage defines a single stage in a workflow pipeline.
type WorkflowStage struct {
	Name   string `yaml:"name"`             // stage identifier (e.g. "plan", "implement")
	Prompt string `yaml:"prompt"`           // prompt template with {{.paramName}} variables
	Detect string `yaml:"detect,omitempty"` // glob pattern to detect stage completion in worktree
}
