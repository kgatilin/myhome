package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/archive"
	"github.com/kgatilin/myhome/internal/cleanup"
	"github.com/kgatilin/myhome/internal/config"
)

var cleanupApply bool

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Find garbage in workspace",
	Long:  "Reports orphan worktrees, stale branches, large untracked files, empty dirs. No deletions without --apply.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, state, homeDir, err := loadRepoContext()
		if err != nil {
			return err
		}
		env, err := cfg.ResolveEnv(state.CurrentEnv)
		if err != nil {
			return err
		}

		issues, err := cleanup.Scan(env.Repos, homeDir)
		if err != nil {
			return err
		}

		if len(issues) == 0 {
			fmt.Println("No issues found.")
			return nil
		}

		if !cleanupApply {
			fmt.Printf("Found %d issues (run with --apply to fix):\n\n", len(issues))
			for _, issue := range issues {
				fmt.Printf("  [%s] %s", issue.Type, issue.Path)
				if issue.Details != "" {
					fmt.Printf(" (%s)", issue.Details)
				}
				fmt.Println()
			}
			return nil
		}

		reader := bufio.NewReader(os.Stdin)
		return cleanup.Apply(issues, reader)
	},
}

var archiveCmd = &cobra.Command{
	Use:   "archive <path>",
	Short: "Move folder to ~/archive/ and update .gitignore",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		return archive.Move(args[0], homeDir, cfg)
	},
}

func init() {
	cleanupCmd.Flags().BoolVar(&cleanupApply, "apply", false, "Interactively confirm each cleanup action")
}
