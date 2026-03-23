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
	Short: "Create a task (use --repo/--branch to make it runnable)",
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
		repo, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")

		taskType := task.TaskTypeGeneral
		if repo != "" {
			taskType = task.TaskTypeRun
		}

		t := &task.Task{
			ID:          id,
			Type:        taskType,
			Domain:      domain,
			Description: args[0],
			Status:      task.TaskStatusOpen,
			CreatedAt:   time.Now(),
			Repo:        repo,
			Branch:      branch,
		}
		if err := store.Save(t); err != nil {
			return err
		}
		fmt.Printf("Task %d created\n", id)
		return nil
	},
}

var taskRunCmd = &cobra.Command{
	Use:   "run [id]",
	Short: "Run a task: create worktree + launch container",
	Long: `Run a task by ID, or create and run inline:

  myhome task run 5                              # Run existing task by ID
  myhome task run --repo uagent --branch UAGENT-500 --prompt "/jira-start UAGENT-500"  # Inline
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := defaultTaskStore()
		if err != nil {
			return err
		}

		var t *task.Task

		if len(args) == 1 {
			// Mode 1: Run existing task by ID (supports re-runs on same worktree)
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid task ID: %s", args[0])
			}
			t, err = store.Load(id)
			if err != nil {
				return err
			}
			// Allow overriding prompt for re-runs
			prompt, _ := cmd.Flags().GetString("prompt")
			if prompt != "" {
				t.Description = prompt
			}
			// Reset status so it can be re-run
			if t.Status == task.TaskStatusRunning {
				t.Status = task.TaskStatusOpen
			}
		} else {
			// Mode 2: Inline — create task from flags then run it
			repoName, _ := cmd.Flags().GetString("repo")
			branch, _ := cmd.Flags().GetString("branch")
			prompt, _ := cmd.Flags().GetString("prompt")
			domain, _ := cmd.Flags().GetString("domain")

			if repoName == "" || branch == "" || prompt == "" {
				return fmt.Errorf("inline mode requires --repo, --branch, and --prompt")
			}

			id, err := store.NextID()
			if err != nil {
				return err
			}
			t = &task.Task{
				ID:          id,
				Type:        task.TaskTypeRun,
				Domain:      domain,
				Description: prompt,
				Status:      task.TaskStatusOpen,
				CreatedAt:   time.Now(),
				Repo:        repoName,
				Branch:      branch,
			}
			if err := store.Save(t); err != nil {
				return err
			}
			fmt.Printf("Task %d created\n", id)
		}

		if t.Repo == "" {
			return fmt.Errorf("task %d has no repo set (add with --repo to make it runnable)", t.ID)
		}
		if t.Branch == "" {
			return fmt.Errorf("task %d has no branch set (add with --branch)", t.ID)
		}
		if t.Status == task.TaskStatusRunning {
			return fmt.Errorf("task %d is already running", t.ID)
		}

		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		authProfile, _ := cmd.Flags().GetString("auth")
		remoteName, _ := cmd.Flags().GetString("remote")

		// Remote execution path: SSH + tmux instead of local container.
		if remoteName != "" {
			remoteCfg, ok := cfg.Remotes[remoteName]
			if !ok {
				return fmt.Errorf("unknown remote %q", remoteName)
			}
			session, err := remote.Run(remoteCfg, t.Repo, t.Description, authProfile, nil)
			if err != nil {
				return err
			}
			t.Status = task.TaskStatusRunning
			t.AuthProfile = authProfile
			if err := store.Save(t); err != nil {
				return err
			}
			fmt.Printf("Task %d started remotely on %s (session: %s)\n", t.ID, remoteName, session)
			fmt.Printf("Attach with: myhome remote attach %s %s\n", remoteName, session)
			return nil
		}

		runtime, err := container.DetectRuntime(cfg.ContainerRuntime)
		if err != nil {
			return err
		}

		homeDir, _ := os.UserHomeDir()

		// Resolve repo path and container from config
		var projectDir string
		var repoContainer string
		for _, r := range cfg.Repos {
			base := filepath.Base(r.Path)
			if base == t.Repo || r.Path == t.Repo {
				projectDir = filepath.Join(homeDir, r.Path)
				repoContainer = r.Container
				break
			}
		}
		if projectDir == "" {
			return fmt.Errorf("unknown repo: %s", t.Repo)
		}

		// Container priority: --container flag > repo config > default
		containerName, _ := cmd.Flags().GetString("container")
		if containerName == "" || containerName == "claude-code" {
			if repoContainer != "" {
				containerName = repoContainer
			}
		}

		// Look up container definition from config
		ctrConfig, ok := cfg.Containers[containerName]
		if !ok {
			return fmt.Errorf("unknown container %q in myhome.yml", containerName)
		}

		runner := task.NewRunner(store, exec.Command, runtime)
		if err := runner.RunTask(t, task.RunOpts{
			ContainerName:   containerName,
			ContainerConfig: ctrConfig,
			AuthProfile:     authProfile,
			ClaudeConfig:    &cfg.Claude,
			ProjectDir:      projectDir,
			HomeDir:         homeDir,
		}); err != nil {
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
	Short: "Push branch, clean up worktree, mark task complete",
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

		// Simple tasks — just mark done
		if t.WorktreePath == "" {
			if err := store.MarkDone(id); err != nil {
				return err
			}
			fmt.Printf("Task %d marked done\n", id)
			return nil
		}

		// Run tasks — push + cleanup worktree
		merge, _ := cmd.Flags().GetBool("merge")

		cfgPath, _ := config.DefaultConfigPath()
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		runtime, err := container.DetectRuntime(cfg.ContainerRuntime)
		if err != nil {
			return err
		}

		runner := task.NewRunner(store, exec.Command, runtime)
		if err := runner.Done(id, merge); err != nil {
			return err
		}
		fmt.Printf("Task %d done: branch pushed, worktree cleaned up\n", id)
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
	taskAddCmd.Flags().String("repo", "", "Repository name (makes task runnable)")
	taskAddCmd.Flags().String("branch", "", "Branch name for worktree")
	taskRunCmd.Flags().String("auth", "", "Claude auth profile")
	taskRunCmd.Flags().String("container", "", "Container to use (default: from repo config)")
	taskRunCmd.Flags().String("remote", "", "Run on remote host instead of locally")
	taskRunCmd.Flags().String("repo", "", "Repository name (inline mode)")
	taskRunCmd.Flags().String("branch", "", "Branch name (inline mode)")
	taskRunCmd.Flags().String("prompt", "", "Prompt for Claude (inline mode)")
	taskRunCmd.Flags().String("domain", "", "Domain tag (inline mode)")
	taskDoneCmd.Flags().Bool("merge", false, "Merge locally via Worktrunk before removing worktree (default: push only)")
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
