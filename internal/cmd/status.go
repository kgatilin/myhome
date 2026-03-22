package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show workspace overview",
	Long:  "Current env, dirty repos, active worktrees, disk usage.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome status")
		fmt.Println("Not implemented yet")
		return nil
	},
}
