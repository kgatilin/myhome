package cmd

import (
	"fmt"
	"os"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/workspace"
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Generate VSCode workspace files",
	Long: `Generate .code-workspace files from myhome.yml repos, grouped by domain.

Creates:
  home.code-workspace  — all repos
  work.code-workspace  — work/ domain repos
  dev.code-workspace   — dev/ domain repos
  life.code-workspace  — life/ domain repos`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		files, err := workspace.GenerateAll(cfg)
		if err != nil {
			return err
		}

		if err := workspace.WriteAll(cfg, homeDir); err != nil {
			return err
		}

		for name := range files {
			fmt.Printf("  wrote %s\n", name)
		}
		return nil
	},
}
