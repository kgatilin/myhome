package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPath returns the default config file location: ~/setup/myhome.yml
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, "setup", "myhome.yml"), nil
}

// Load reads and parses a myhome.yml file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses myhome.yml content from bytes.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.ContainerRuntime == "" {
		cfg.ContainerRuntime = "auto"
	}
	return &cfg, nil
}

// ResolveEnv returns the merged repos, tools, and packages for a named environment.
// It follows the env's include list, merging all referenced tags.
func (c *Config) ResolveEnv(envName string) (*ResolvedEnv, error) {
	env, ok := c.Envs[envName]
	if !ok {
		return nil, fmt.Errorf("unknown env: %s", envName)
	}

	resolved := &ResolvedEnv{
		Name:  envName,
		Tools: make(map[string]string),
	}

	tagSet := make(map[string]bool, len(env.Include))
	for _, tag := range env.Include {
		tagSet[tag] = true
	}

	// Collect repos matching any included tag
	for _, repo := range c.Repos {
		if tagSet[repo.Env] {
			resolved.Repos = append(resolved.Repos, repo)
		}
	}

	// Merge tools from all included tags (later tags override earlier)
	for _, tag := range env.Include {
		if tools, ok := c.Tools[tag]; ok {
			for k, v := range tools {
				resolved.Tools[k] = v
			}
		}
	}

	// Merge packages from all included tags
	for _, tag := range env.Include {
		if pkgs, ok := c.Packages[tag]; ok {
			resolved.Packages.Brew = append(resolved.Packages.Brew, pkgs.Brew...)
			resolved.Packages.BrewCask = append(resolved.Packages.BrewCask, pkgs.BrewCask...)
			resolved.Packages.Apt = append(resolved.Packages.Apt, pkgs.Apt...)
		}
	}

	return resolved, nil
}

// Save writes the config back to a YAML file.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}
