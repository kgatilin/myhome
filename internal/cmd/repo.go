package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories",
	Long:  "List, sync, add, remove repos. Manage worktrees via 'myhome repo <name> wt'.",
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List repos for current env",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome repo list")
		fmt.Println("Not implemented yet")
		return nil
	},
}

var repoSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Clone missing repos for current env",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome repo sync")
		fmt.Println("Not implemented yet")
		return nil
	},
}

var repoAddCmd = &cobra.Command{
	Use:   "add <path> [url]",
	Short: "Add repo to manifest",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("myhome repo add %v\n", args)
		fmt.Println("Not implemented yet")
		return nil
	},
}

var repoRmCmd = &cobra.Command{
	Use:   "rm <path>",
	Short: "Remove repo from manifest",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("myhome repo rm %s\n", args[0])
		fmt.Println("Not implemented yet")
		return nil
	},
}

var repoWtListCmd = &cobra.Command{
	Use:   "wt",
	Short: "Cross-repo worktree overview",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome repo wt list")
		fmt.Println("Not implemented yet")
		return nil
	},
}

func init() {
	repoAddCmd.Flags().String("env", "", "Environment tag for the repo")
	repoCmd.AddCommand(repoListCmd)
	repoCmd.AddCommand(repoSyncCmd)
	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoRmCmd)
	repoCmd.AddCommand(repoWtListCmd)
}
