package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/packages"
	"github.com/kgatilin/myhome/internal/platform"
)

var packagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "Manage system packages (brew/apt)",
}

var packagesListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show installed vs expected packages",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, state, _, err := loadRepoContext()
		if err != nil {
			return err
		}
		env, err := cfg.ResolveEnv(state.CurrentEnv)
		if err != nil {
			return err
		}
		plat, err := platform.Detect()
		if err != nil {
			return err
		}
		statuses, err := packages.List(env.Packages, plat)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			status := "missing"
			if s.Installed {
				status = "installed"
			}
			fmt.Printf("%-25s [%s]\n", s.Name, status)
		}
		return nil
	},
}

var packagesSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Install missing packages",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, state, _, err := loadRepoContext()
		if err != nil {
			return err
		}
		env, err := cfg.ResolveEnv(state.CurrentEnv)
		if err != nil {
			return err
		}
		plat, err := platform.Detect()
		if err != nil {
			return err
		}
		if err := packages.Sync(env.Packages, plat); err != nil {
			return err
		}
		fmt.Println("Packages synced successfully")
		return nil
	},
}

var packagesDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Snapshot current packages into myhome.yml",
	RunE: func(cmd *cobra.Command, args []string) error {
		return packages.DumpToWriter(os.Stdout)
	},
}

func init() {
	packagesCmd.AddCommand(packagesListCmd)
	packagesCmd.AddCommand(packagesSyncCmd)
	packagesCmd.AddCommand(packagesDumpCmd)
}
