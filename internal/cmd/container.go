package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/container"
)

var containerCmd = &cobra.Command{
	Use:   "container",
	Short: "Manage Docker containers",
}

var containerBuildCmd = &cobra.Command{
	Use:               "build <name>",
	Short:             "Build container image",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: containerNameCompletionFunc,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, homeDir, runtime, err := loadContainerDeps()
		if err != nil {
			return err
		}

		all, _ := cmd.Flags().GetBool("all")
		if all {
			for name, ctr := range cfg.Containers {
				fmt.Printf("Building %s...\n", name)
				if err := container.Build(runtime, name, ctr, homeDir); err != nil {
					return err
				}
			}
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("specify container name or use --all")
		}
		name := args[0]
		ctr, ok := cfg.Containers[name]
		if !ok {
			return fmt.Errorf("unknown container: %s", name)
		}
		return container.Build(runtime, name, ctr, homeDir)
	},
}

var containerRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run container with optional auth profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, homeDir, runtime, err := loadContainerDeps()
		if err != nil {
			return err
		}

		name := args[0]
		ctr, ok := cfg.Containers[name]
		if !ok {
			return fmt.Errorf("unknown container: %s", name)
		}

		opts := container.RunOpts{}
		authName, _ := cmd.Flags().GetString("auth")
		if authName != "" {
			profile, ok := cfg.Claude.AuthProfiles[authName]
			if !ok {
				return fmt.Errorf("unknown auth profile: %s", authName)
			}
			opts.AuthProfile = &profile
			opts.ClaudeConfigDir = cfg.Claude.ConfigDir
		}

		if ctr.GitBackup {
			// Git backup is project-specific; skip if no project dir set
		}

		id, err := container.Run(runtime, name, ctr, homeDir, opts)
		if err != nil {
			return err
		}
		if id != "" {
			fmt.Printf("Container started: %s\n", id)
		}
		return nil
	},
}

var containerListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show defined containers with build status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, runtime, err := loadContainerDeps()
		if err != nil {
			return err
		}

		running, err := container.List(runtime)
		if err != nil {
			// Not fatal — just can't show running status
			running = nil
		}
		runningByImage := make(map[string]string, len(running))
		for _, c := range running {
			runningByImage[c.Image] = c.Status
		}

		fmt.Printf("%-20s %-35s %-10s %s\n", "NAME", "IMAGE", "FIREWALL", "STATUS")
		for name, ctr := range cfg.Containers {
			status := "-"
			if s, ok := runningByImage[ctr.Image]; ok {
				status = s
			}
			fw := "no"
			if ctr.Firewall {
				fw = "yes"
			}
			fmt.Printf("%-20s %-35s %-10s %s\n", name, ctr.Image, fw, status)
		}
		return nil
	},
}

var containerShellCmd = &cobra.Command{
	Use:   "shell <name>",
	Short: "Open shell in container for debugging",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _, runtime, err := loadContainerDeps()
		if err != nil {
			return err
		}
		return container.Shell(runtime, args[0])
	},
}

func init() {
	containerBuildCmd.Flags().Bool("all", false, "Build all container images")
	containerRunCmd.Flags().String("auth", "", "Auth profile to use (from claude.auth_profiles)")

	containerCmd.AddCommand(containerBuildCmd)
	containerCmd.AddCommand(containerRunCmd)
	containerCmd.AddCommand(containerListCmd)
	containerCmd.AddCommand(containerShellCmd)
}

// loadContainerDeps loads config, home dir, and detects runtime.
func loadContainerDeps() (*config.Config, string, string, error) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil, "", "", err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("load config: %w", err)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, "", "", err
	}
	runtime, err := container.DetectRuntime(cfg.ContainerRuntime)
	if err != nil {
		return nil, "", "", err
	}
	return cfg, homeDir, runtime, nil
}
