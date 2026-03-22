package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initEnv string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap the workspace for an environment",
	Long: `Full machine bootstrap:
  1. Verify/install mise
  2. Generate ~/.mise.toml for the env
  3. Install dev runtimes (mise install)
  4. Install system packages (brew/apt)
  5. Clone repos matching the env
  6. Generate ~/.ssh/config
  7. Generate ~/.gitignore
  8. Generate ~/.gitconfig identity includes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("myhome init --env %s\n", initEnv)
		fmt.Println("Not implemented yet")
		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&initEnv, "env", "full", "Environment to initialize (base, work, personal, full)")
}
