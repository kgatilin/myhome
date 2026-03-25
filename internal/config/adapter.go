package config

import "time"

// AdaptersConfig groups all adapter configurations.
type AdaptersConfig struct {
	GitHub   *GitHubAdapterConfig   `yaml:"github,omitempty"`
	Telegram *TelegramAdapterConfig `yaml:"telegram,omitempty"`
}

// GitHubAdapterConfig configures the GitHub issue polling adapter.
type GitHubAdapterConfig struct {
	Repos         []string      `yaml:"repos"`
	Label         string        `yaml:"label"`
	PollInterval  time.Duration `yaml:"poll_interval"`
	BusSocket     string        `yaml:"bus_socket"`
	DefaultTarget string        `yaml:"default_target"`
}

// TelegramAdapterConfig configures the Telegram bot adapter.
type TelegramAdapterConfig struct {
	Token         string          `yaml:"token"`          // vault:// prefix supported
	BotUsername   string          `yaml:"bot_username"`   // e.g. "kiraautonomaos_bot"
	BusSocket     string          `yaml:"bus_socket"`
	Routes        []TelegramRoute `yaml:"routes"`
	OwnerID       int64           `yaml:"owner_id"`
	DefaultTarget string          `yaml:"default_target"`
}

// TelegramRoute maps a Telegram chat to a bus target.
type TelegramRoute struct {
	ChatID      int64  `yaml:"chat_id"`
	Target      string `yaml:"target"`
	OwnerOnly   bool   `yaml:"owner_only"`
	MentionOnly bool   `yaml:"mention_only"` // only route when bot is @mentioned
}
