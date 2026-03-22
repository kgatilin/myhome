package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks",
	Long:  "Lightweight task management: general tasks and dev run tasks (worktree + container orchestration).",
}

var taskAddCmd = &cobra.Command{
	Use:   "add <description>",
	Short: "Create a general task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain, _ := cmd.Flags().GetString("domain")
		fmt.Printf("myhome task add %q --domain %s\n", args[0], domain)
		fmt.Println("Not implemented yet")
		return nil
	},
}

var taskRunCmd = &cobra.Command{
	Use:   "run <repo> <branch> <prompt>",
	Short: "Create worktree + launch Claude in Docker background",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		auth, _ := cmd.Flags().GetString("auth")
		container, _ := cmd.Flags().GetString("container")
		fmt.Printf("myhome task run %s %s %q --auth %s --container %s\n", args[0], args[1], args[2], auth, container)
		fmt.Println("Not implemented yet")
		return nil
	},
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome task list")
		fmt.Println("Not implemented yet")
		return nil
	},
}

var taskLogCmd = &cobra.Command{
	Use:   "log <id>",
	Short: "Stream Claude output for a run task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("myhome task log %s\n", args[0])
		fmt.Println("Not implemented yet")
		return nil
	},
}

var taskDoneCmd = &cobra.Command{
	Use:   "done <id>",
	Short: "Mark task complete",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("myhome task done %s\n", args[0])
		fmt.Println("Not implemented yet")
		return nil
	},
}

var taskStopCmd = &cobra.Command{
	Use:   "stop <id>",
	Short: "Stop running container for a run task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("myhome task stop %s\n", args[0])
		fmt.Println("Not implemented yet")
		return nil
	},
}

var taskRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Remove task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("myhome task rm %s\n", args[0])
		fmt.Println("Not implemented yet")
		return nil
	},
}

func init() {
	taskAddCmd.Flags().String("domain", "", "Domain tag (work, dev, life)")
	taskRunCmd.Flags().String("auth", "", "Claude auth profile")
	taskRunCmd.Flags().String("container", "claude-code", "Container to use")
	taskListCmd.Flags().String("domain", "", "Filter by domain")
	taskListCmd.Flags().String("status", "", "Filter by status (open, running, done)")

	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskRunCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskLogCmd)
	taskCmd.AddCommand(taskDoneCmd)
	taskCmd.AddCommand(taskStopCmd)
	taskCmd.AddCommand(taskRmCmd)
}
