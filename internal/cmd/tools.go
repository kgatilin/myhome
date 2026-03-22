package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Manage dev runtimes (via mise)",
}

var toolsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show installed vs expected runtimes",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome tools list")
		fmt.Println("Not implemented yet")
		return nil
	},
}

var toolsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Install/update runtimes for current env",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome tools sync")
		fmt.Println("Not implemented yet")
		return nil
	},
}

func init() {
	toolsCmd.AddCommand(toolsListCmd)
	toolsCmd.AddCommand(toolsSyncCmd)
}
