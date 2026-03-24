package config

// ClaudeConfig holds Claude-specific settings.
type ClaudeConfig struct {
	ConfigDir    string                 `yaml:"config_dir"`
	AuthProfiles map[string]AuthProfile `yaml:"auth_profiles"`
}

// AuthProfile defines auth credentials for a Claude profile.
type AuthProfile struct {
	AuthFile string            `yaml:"auth_file"`
	Env      map[string]string `yaml:"env,omitempty"`
}
