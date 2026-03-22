package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var containerCmd = &cobra.Command{
	Use:   "container",
	Short: "Manage Docker containers",
}

var containerBuildCmd = &cobra.Command{
	Use:   "build <name>",
	Short: "Build container image",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all {
			fmt.Println("myhome container build --all")
		} else if len(args) > 0 {
			fmt.Printf("myhome container build %s\n", args[0])
		}
		fmt.Println("Not implemented yet")
		return nil
	},
}

var containerRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run container with optional auth profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		auth, _ := cmd.Flags().GetString("auth")
		fmt.Printf("myhome container run %s --auth %s\n", args[0], auth)
		fmt.Println("Not implemented yet")
		return nil
	},
}

var containerListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show defined containers with build status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome container list")
		fmt.Println("Not implemented yet")
		return nil
	},
}

var containerShellCmd = &cobra.Command{
	Use:   "shell <name>",
	Short: "Open shell in container for debugging",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("myhome container shell %s\n", args[0])
		fmt.Println("Not implemented yet")
		return nil
	},
}

func init() {
	containerBuildCmd.Flags().Bool("all", false, "Build all container images")
	containerRunCmd.Flags().String("auth", "", "Auth profile to use (from claude.auth_profiles)")

	containerCmd.AddCommand(containerBuildCmd)
	containerCmd.AddCommand(containerRunCmd)
	containerCmd.AddCommand(containerListCmd)
	containerCmd.AddCommand(containerShellCmd)
}
