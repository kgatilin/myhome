package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/auth"
	"github.com/kgatilin/myhome/internal/config"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage SSH keys and config",
}

var authGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate ~/.ssh/config from myhome.yml",
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
		if err := auth.WriteSSHConfig(cfg.Auth, homeDir); err != nil {
			return err
		}
		fmt.Println("Generated ~/.ssh/config")
		return nil
	},
}

var authKeysCmd = &cobra.Command{
	Use:   "keys",
	Short: "List keys and which hosts they serve",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}
		keys := auth.ListKeys(cfg.Auth)
		for _, k := range keys {
			fmt.Printf("~/.ssh/%-20s → %v\n", k.Key, k.Hosts)
		}
		return nil
	},
}

func init() {
	authCmd.AddCommand(authGenerateCmd)
	authCmd.AddCommand(authKeysCmd)
}
