package cmd

import (
	"fmt"
	"os"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/selfupdate"
	"github.com/spf13/cobra"
)

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update myhome binary from source",
	Long:  "Pull latest changes from the myhome source repo, rebuild, and replace the current binary.",
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		var repoPaths []string
		cfgPath, err := config.DefaultConfigPath()
		if err == nil {
			if cfg, loadErr := config.Load(cfgPath); loadErr == nil {
				for _, r := range cfg.Repos {
					repoPaths = append(repoPaths, r.Path)
				}
			}
		}

		sourceDir, err := selfupdate.FindSourceDir(homeDir, repoPaths)
		if err != nil {
			return err
		}
		fmt.Printf("Source repo: %s\n", sourceDir)

		return selfupdate.Run(sourceDir)
	},
}
