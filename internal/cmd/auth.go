package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage SSH keys and config",
}

var authGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate ~/.ssh/config from myhome.yml",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome auth generate")
		fmt.Println("Not implemented yet")
		return nil
	},
}

var authKeysCmd = &cobra.Command{
	Use:   "keys",
	Short: "List keys and which hosts they serve",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome auth keys")
		fmt.Println("Not implemented yet")
		return nil
	},
}

func init() {
	authCmd.AddCommand(authGenerateCmd)
	authCmd.AddCommand(authKeysCmd)
}
