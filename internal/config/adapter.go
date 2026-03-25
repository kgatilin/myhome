package config

import "time"

// AdaptersConfig groups all adapter configurations.
type AdaptersConfig struct {
	GitHub *GitHubAdapterConfig `yaml:"github,omitempty"`
}

// GitHubAdapterConfig configures the GitHub issue polling adapter.
type GitHubAdapterConfig struct {
	Repos         []string      `yaml:"repos"`
	Label         string        `yaml:"label"`
	PollInterval  time.Duration `yaml:"poll_interval"`
	BusSocket     string        `yaml:"bus_socket"`
	DefaultTarget string        `yaml:"default_target"`
}
