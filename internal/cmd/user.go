package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage agent users",
}

var userCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create agent user with env-scoped access",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env, _ := cmd.Flags().GetString("env")
		tmpl, _ := cmd.Flags().GetString("template")
		fmt.Printf("myhome user create %s --env %s --template %s\n", args[0], env, tmpl)
		fmt.Println("Not implemented yet")
		return nil
	},
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List managed users",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome user list")
		fmt.Println("Not implemented yet")
		return nil
	},
}

var userRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove agent user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("myhome user rm %s\n", args[0])
		fmt.Println("Not implemented yet")
		return nil
	},
}

var userShellCmd = &cobra.Command{
	Use:   "shell <name>",
	Short: "Open shell as agent user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("myhome user shell %s\n", args[0])
		fmt.Println("Not implemented yet")
		return nil
	},
}

var userSyncCmd = &cobra.Command{
	Use:   "sync <name>",
	Short: "Sync agent's home repo",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all {
			fmt.Println("myhome user sync --all")
		} else if len(args) > 0 {
			fmt.Printf("myhome user sync %s\n", args[0])
		}
		fmt.Println("Not implemented yet")
		return nil
	},
}

func init() {
	userCreateCmd.Flags().String("env", "", "Environment for the agent")
	userCreateCmd.Flags().String("template", "", "Agent template to use")
	userSyncCmd.Flags().Bool("all", false, "Sync all agent users")

	userCmd.AddCommand(userCreateCmd)
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userRmCmd)
	userCmd.AddCommand(userShellCmd)
	userCmd.AddCommand(userSyncCmd)
}
