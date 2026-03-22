package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/tools"
)

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Manage dev runtimes (via mise)",
}

var toolsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show installed vs expected runtimes",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, state, _, err := loadRepoContext()
		if err != nil {
			return err
		}
		env, err := cfg.ResolveEnv(state.CurrentEnv)
		if err != nil {
			return err
		}
		statuses, err := tools.List(env.Tools)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			installed := s.Installed
			if installed == "" {
				installed = "not installed"
			}
			fmt.Printf("%-15s expected=%-8s installed=%s\n", s.Name, s.Expected, installed)
		}
		return nil
	},
}

var toolsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Install/update runtimes for current env",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, state, homeDir, err := loadRepoContext()
		if err != nil {
			return err
		}
		env, err := cfg.ResolveEnv(state.CurrentEnv)
		if err != nil {
			return err
		}
		if err := tools.Sync(env.Tools, homeDir); err != nil {
			return err
		}
		fmt.Println("Tools synced successfully")
		return nil
	},
}

func init() {
	toolsCmd.AddCommand(toolsListCmd)
	toolsCmd.AddCommand(toolsSyncCmd)
}
