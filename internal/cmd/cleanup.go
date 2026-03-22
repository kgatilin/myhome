package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cleanupApply bool

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Find garbage in workspace",
	Long:  "Reports orphan worktrees, stale branches, large untracked files, empty dirs. No deletions without --apply.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cleanupApply {
			fmt.Println("myhome cleanup --apply (interactive)")
		} else {
			fmt.Println("myhome cleanup (report only)")
		}
		fmt.Println("Not implemented yet")
		return nil
	},
}

var archiveCmd = &cobra.Command{
	Use:   "archive <path>",
	Short: "Move folder to ~/archive/ and update .gitignore",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("myhome archive %s\n", args[0])
		fmt.Println("Not implemented yet")
		return nil
	},
}

func init() {
	cleanupCmd.Flags().BoolVar(&cleanupApply, "apply", false, "Interactively confirm each cleanup action")
}
