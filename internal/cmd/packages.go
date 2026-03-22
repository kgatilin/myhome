package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var packagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "Manage system packages (brew/apt)",
}

var packagesListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show installed vs expected packages",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome packages list")
		fmt.Println("Not implemented yet")
		return nil
	},
}

var packagesSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Install missing packages",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome packages sync")
		fmt.Println("Not implemented yet")
		return nil
	},
}

var packagesDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Snapshot current packages into myhome.yml",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome packages dump")
		fmt.Println("Not implemented yet")
		return nil
	},
}

func init() {
	packagesCmd.AddCommand(packagesListCmd)
	packagesCmd.AddCommand(packagesSyncCmd)
	packagesCmd.AddCommand(packagesDumpCmd)
}
