package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/container"
	"github.com/kgatilin/myhome/internal/remote"
	"github.com/kgatilin/myhome/internal/task"
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
		store, err := defaultTaskStore()
		if err != nil {
			return err
		}
		id, err := store.NextID()
		if err != nil {
			return err
		}
		domain, _ := cmd.Flags().GetString("domain")
		t := &task.Task{
			ID:          id,
			Type:        task.TaskTypeGeneral,
			Domain:      domain,
			Description: args[0],
			Status:      task.TaskStatusOpen,
			CreatedAt:   time.Now(),
		}
		if err := store.Save(t); err != nil {
			return err
		}
		fmt.Printf("Task %d created\n", id)
		return nil
	},
}

var taskRunCmd = &cobra.Command{
	Use:   "run <repo> <branch> <prompt>",
	Short: "Create worktree + launch Claude in Docker background",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		remoteName, _ := cmd.Flags().GetString("remote")
		authProfile, _ := cmd.Flags().GetString("auth")

		// Remote execution path: SSH + tmux instead of local container.
		if remoteName != "" {
			remoteCfg, ok := cfg.Remotes[remoteName]
			if !ok {
				return fmt.Errorf("unknown remote %q", remoteName)
			}
			store, err := defaultTaskStore()
			if err != nil {
				return err
			}
			id, err := store.NextID()
			if err != nil {
				return err
			}
			session, err := remote.Run(remoteCfg, args[0], args[2], authProfile, nil)
			if err != nil {
				return err
			}
			t := &task.Task{
				ID:          id,
				Type:        task.TaskTypeRun,
				Description: args[2],
				Status:      task.TaskStatusRunning,
				CreatedAt:   time.Now(),
				Repo:        args[0],
				Branch:      args[1],
			}
			if err := store.Save(t); err != nil {
				return err
			}
			fmt.Printf("Task %d started remotely on %s (session: %s)\n", id, remoteName, session)
			fmt.Printf("Attach with: myhome remote attach %s %s\n", remoteName, session)
			return nil
		}

		store, err := defaultTaskStore()
		if err != nil {
			return err
		}

		runtime, err := container.DetectRuntime(cfg.ContainerRuntime)
		if err != nil {
			return err
		}

		homeDir, _ := os.UserHomeDir()

		// Resolve repo path from config
		repoName := args[0]
		var projectDir string
		for _, r := range cfg.Repos {
			base := filepath.Base(r.Path)
			if base == repoName || r.Path == repoName {
				projectDir = filepath.Join(homeDir, r.Path)
				break
			}
		}
		if projectDir == "" {
			return fmt.Errorf("unknown repo: %s", repoName)
		}

		containerName, _ := cmd.Flags().GetString("container")

		runner := task.NewRunner(store, exec.Command, runtime)
		t, err := runner.Run(task.RunOpts{
			Repo:        repoName,
			Branch:      args[1],
			Description: args[2],
			Container:   containerName,
			AuthProfile: authProfile,
			ProjectDir:  projectDir,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Task %d started (container: %s)\n", t.ID, t.ContainerID)
		fmt.Printf("Log: %s\n", t.LogFile)
		return nil
	},
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := defaultTaskStore()
		if err != nil {
			return err
		}
		domain, _ := cmd.Flags().GetString("domain")
		status, _ := cmd.Flags().GetString("status")
		tasks, err := store.List(task.ListFilter{
			Status: task.TaskStatus(status),
			Domain: domain,
		})
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			fmt.Println("No tasks")
			return nil
		}
		fmt.Printf("%-4s %-8s %-10s %-10s %s\n", "ID", "TYPE", "STATUS", "DOMAIN", "DESCRIPTION")
		for _, t := range tasks {
			fmt.Printf("%-4d %-8s %-10s %-10s %s\n", t.ID, t.Type, t.Status, t.Domain, t.Description)
		}
		return nil
	},
}

var taskLogCmd = &cobra.Command{
	Use:   "log <id>",
	Short: "Stream Claude output for a run task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := defaultTaskStore()
		if err != nil {
			return err
		}
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid task ID: %s", args[0])
		}
		t, err := store.Load(id)
		if err != nil {
			return err
		}
		if t.LogFile == "" {
			return fmt.Errorf("task %d has no log file", id)
		}
		follow, _ := cmd.Flags().GetBool("follow")
		return task.TailLog(t.LogFile, follow)
	},
}

var taskDoneCmd = &cobra.Command{
	Use:   "done <id>",
	Short: "Mark task complete",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := defaultTaskStore()
		if err != nil {
			return err
		}
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid task ID: %s", args[0])
		}
		if err := store.MarkDone(id); err != nil {
			return err
		}
		fmt.Printf("Task %d marked done\n", id)
		return nil
	},
}

var taskStopCmd = &cobra.Command{
	Use:   "stop <id>",
	Short: "Stop running container for a run task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		store, err := defaultTaskStore()
		if err != nil {
			return err
		}
		runtime, err := container.DetectRuntime(cfg.ContainerRuntime)
		if err != nil {
			return err
		}
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid task ID: %s", args[0])
		}
		runner := task.NewRunner(store, exec.Command, runtime)
		if err := runner.Stop(id); err != nil {
			return err
		}
		fmt.Printf("Task %d stopped\n", id)
		return nil
	},
}

var taskRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Remove task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := defaultTaskStore()
		if err != nil {
			return err
		}
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid task ID: %s", args[0])
		}
		if err := store.Remove(id); err != nil {
			return err
		}
		fmt.Printf("Task %d removed\n", id)
		return nil
	},
}

func init() {
	taskAddCmd.Flags().String("domain", "", "Domain tag (work, dev, life)")
	taskRunCmd.Flags().String("auth", "", "Claude auth profile")
	taskRunCmd.Flags().String("container", "claude-code", "Container to use")
	taskRunCmd.Flags().String("remote", "", "Run on remote host instead of locally")
	taskListCmd.Flags().String("domain", "", "Filter by domain")
	taskListCmd.Flags().String("status", "", "Filter by status (open, running, done)")
	taskLogCmd.Flags().BoolP("follow", "f", false, "Follow log output")

	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskRunCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskLogCmd)
	taskCmd.AddCommand(taskDoneCmd)
	taskCmd.AddCommand(taskStopCmd)
	taskCmd.AddCommand(taskRmCmd)
}

func defaultTaskStore() (*task.Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return task.NewStore(filepath.Join(homeDir, "tasks"))
}
