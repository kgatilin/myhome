package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/container"
	"github.com/kgatilin/myhome/internal/remote"
	"github.com/kgatilin/myhome/internal/task"
	"github.com/kgatilin/myhome/internal/vault"
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

		// Parse workflow params (--param key=value)
		paramFlags, _ := cmd.Flags().GetStringSlice("param")
		if len(paramFlags) > 0 {
			t.WorkflowParams = parseParams(paramFlags)
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

			if repoName == "" || branch == "" {
				return fmt.Errorf("inline mode requires --repo and --branch")
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

			// Parse workflow params (--param key=value)
			paramFlags, _ := cmd.Flags().GetStringSlice("param")
			if len(paramFlags) > 0 {
				t.WorkflowParams = parseParams(paramFlags)
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
		var repoConfig *config.Repo
		for i, r := range cfg.Repos {
			base := filepath.Base(r.Path)
			if base == t.Repo || r.Path == t.Repo {
				projectDir = filepath.Join(homeDir, r.Path)
				repoContainer = r.Container
				repoConfig = &cfg.Repos[i]
				break
			}
			// Fallback: check if repo matches any path segment (e.g. "uagent" matches "work/uagent/code")
			for _, seg := range strings.Split(r.Path, "/") {
				if seg == t.Repo {
					projectDir = filepath.Join(homeDir, r.Path)
					repoContainer = r.Container
					repoConfig = &cfg.Repos[i]
					break
				}
			}
			if projectDir != "" {
				break
			}
		}
		if projectDir == "" {
			return fmt.Errorf("unknown repo: %s", t.Repo)
		}

		// Container priority: --container flag > repo config > default
		containerName, _ := cmd.Flags().GetString("container")
		if containerName == "" {
			containerName = repoContainer
		}

		// Look up container definition from config
		ctrConfig, ok := cfg.Containers[containerName]
		if !ok {
			return fmt.Errorf("unknown container %q in myhome.yml", containerName)
		}

		// Determine if notifications are enabled (default true on macOS)
		notifyEnabled := goruntime.GOOS == "darwin"
		if cfg.Tasks.Notifications.Enabled != nil {
			notifyEnabled = *cfg.Tasks.Notifications.Enabled
		}

		// Workflow-aware prompt resolution: if repo has a workflow and no explicit --prompt,
		// use the workflow stage prompt instead.
		if repoConfig.Workflow != nil && t.Description == "" {
			wf := repoConfig.Workflow
			if err := task.ValidateWorkflowParams(wf, t.WorkflowParams); err != nil {
				return err
			}
			// Determine stage: use task's current stage or detect from worktree
			stageName := t.Stage
			if stageName == "" {
				stageName = wf.Stages[0].Name
			}
			prompt, err := task.StagePrompt(wf, stageName)
			if err != nil {
				return err
			}
			t.Description = task.ResolveStagePrompt(prompt, t.WorkflowParams)
			t.Stage = stageName
			t.StageStatus = task.StageStatusRunning
		}

		if t.Description == "" {
			return fmt.Errorf("task %d has no prompt (use --prompt or configure a workflow for this repo)", t.ID)
		}

		// Enrich prompt: run pre_run hooks and append task_suffix
		t.Description = task.EnrichPrompt(t.Description, repoConfig, cfg.Tasks.TaskSuffix, exec.Command)

		// Open vault if any env vars use vault:// references
		var kdbxVault *vault.KDBXVault
		if hasVaultRefs(ctrConfig.Env) {
			dbPath := vault.DefaultVaultPath(homeDir)
			keyFile := vault.DefaultKeyFile(homeDir)
			password, promptErr := promptPassword("Enter vault master password: ")
			if promptErr != nil {
				return promptErr
			}
			kdbxVault, err = vault.OpenKDBX(dbPath, keyFile, password)
			if err != nil {
				return fmt.Errorf("open vault: %w", err)
			}
		}

		// Collect SSH hosts from auth config for known_hosts generation
		var sshHosts []string
		for host := range cfg.Auth {
			sshHosts = append(sshHosts, host)
		}

		runner := task.NewRunner(store, exec.Command, runtime)
		if err := runner.RunTask(t, task.RunOpts{
			ContainerName:   containerName,
			ContainerConfig: ctrConfig,
			AuthProfile:     authProfile,
			ClaudeConfig:    &cfg.Claude,
			ProjectDir:      projectDir,
			HomeDir:         homeDir,
			Notify:          notifyEnabled,
			Vault:           kdbxVault,
			SSHHosts:        sshHosts,
		}); err != nil {
			return err
		}
		fmt.Printf("Task %d started (container: %s)\n", t.ID, t.ContainerID)
		fmt.Printf("Log: %s\n", t.LogFile)

		// --follow: stream logs and block until container exits
		follow, _ := cmd.Flags().GetBool("follow")
		if follow {
			_ = task.TailLog(t.LogFile, true, true)
			exitCode, err := runner.WaitForContainer(t.ContainerID)
			if err != nil {
				return err
			}
			// Reload task to get final status
			t, _ = store.Load(t.ID)
			fmt.Printf("\nTask %d: %s\n", t.ID, t.Status)
			if exitCode != 0 {
				os.Exit(exitCode)
			}
		}
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
		fmt.Printf("%-4s %-8s %-10s %-10s %-12s %s\n", "ID", "TYPE", "STATUS", "DOMAIN", "STAGE", "DESCRIPTION")
		for _, t := range tasks {
			stage := t.Stage
			if stage == "" {
				stage = "-"
			}
			desc := t.Description
			// Truncate long descriptions (enriched prompts)
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			// Replace newlines for single-line display
			desc = strings.ReplaceAll(desc, "\n", " ")
			fmt.Printf("%-4d %-8s %-10s %-10s %-12s %s\n", t.ID, t.Type, t.Status, t.Domain, stage, desc)
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
		raw, _ := cmd.Flags().GetBool("raw")
		format := !raw
		if !follow {
			return task.TailLog(t.LogFile, false, format)
		}

		// Follow mode: stream logs, then block until container exits
		if err := task.TailLog(t.LogFile, true, format); err != nil {
			// tail -f exits when the log stream ends (container stops) — not an error
		}

		if t.ContainerID == "" || t.Status != task.TaskStatusRunning {
			return nil
		}

		// Wait for task store to reflect final status (updated by completion watcher)
		t, err = store.Load(id)
		if err != nil {
			return err
		}
		fmt.Printf("\nTask %d: %s\n", t.ID, t.Status)
		if t.ExitCode != nil && *t.ExitCode != 0 {
			os.Exit(*t.ExitCode)
		}
		return nil
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

var taskNextCmd = &cobra.Command{
	Use:   "next <id>",
	Short: "Advance task to the next workflow stage and launch container",
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
		if t.Repo == "" {
			return fmt.Errorf("task %d has no repo set", t.ID)
		}

		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		homeDir, _ := os.UserHomeDir()

		// Resolve repo config
		repoConfig := resolveRepoConfig(cfg, t.Repo)
		if repoConfig == nil {
			return fmt.Errorf("unknown repo: %s", t.Repo)
		}
		if repoConfig.Workflow == nil {
			return fmt.Errorf("repo %s has no workflow configured", t.Repo)
		}

		wf := repoConfig.Workflow

		// Ensure worktree path is set (needed for detection)
		if t.WorktreePath == "" {
			projectDir := filepath.Join(homeDir, repoConfig.Path)
			sanitized := strings.ReplaceAll(t.Branch, "/", "--")
			t.WorktreePath = filepath.Join(projectDir, ".worktrees", sanitized)
		}

		// Mark current stage complete before advancing
		if t.Stage != "" {
			t.StageStatus = task.StageStatusComplete
		}

		// Detect and advance to next stage
		nextStage, err := task.AdvanceStage(t, wf)
		if err != nil {
			return err
		}

		// Resolve the stage prompt
		prompt, err := task.StagePrompt(wf, nextStage)
		if err != nil {
			return err
		}
		resolved := task.ResolveStagePrompt(prompt, t.WorkflowParams)

		// Update task for the new stage
		t.Description = resolved
		t.Stage = nextStage
		t.StageStatus = task.StageStatusPending
		t.Status = task.TaskStatusOpen

		if err := store.Save(t); err != nil {
			return err
		}

		fmt.Printf("Task %d → stage %q\n", t.ID, nextStage)
		fmt.Printf("Prompt: %s\n", resolved)

		// Auto-launch: run the stage in a container
		authProfile, _ := cmd.Flags().GetString("auth")
		if authProfile == "" {
			authProfile = t.AuthProfile
		}

		runtime, err := container.DetectRuntime(cfg.ContainerRuntime)
		if err != nil {
			return err
		}

		projectDir := filepath.Join(homeDir, repoConfig.Path)
		containerName, _ := cmd.Flags().GetString("container")
		if containerName == "" {
			containerName = repoConfig.Container
		}
		ctrConfig, ok := cfg.Containers[containerName]
		if !ok {
			return fmt.Errorf("unknown container %q in myhome.yml", containerName)
		}

		notifyEnabled := goruntime.GOOS == "darwin"
		if cfg.Tasks.Notifications.Enabled != nil {
			notifyEnabled = *cfg.Tasks.Notifications.Enabled
		}

		// Enrich prompt
		t.Description = task.EnrichPrompt(t.Description, repoConfig, cfg.Tasks.TaskSuffix, exec.Command)

		// Open vault if needed
		var kdbxVault *vault.KDBXVault
		if hasVaultRefs(ctrConfig.Env) {
			dbPath := vault.DefaultVaultPath(homeDir)
			keyFile := vault.DefaultKeyFile(homeDir)
			password, promptErr := promptPassword("Enter vault master password: ")
			if promptErr != nil {
				return promptErr
			}
			kdbxVault, err = vault.OpenKDBX(dbPath, keyFile, password)
			if err != nil {
				return fmt.Errorf("open vault: %w", err)
			}
		}

		var sshHosts []string
		for host := range cfg.Auth {
			sshHosts = append(sshHosts, host)
		}

		runner := task.NewRunner(store, exec.Command, runtime)
		if err := runner.RunTask(t, task.RunOpts{
			ContainerName:   containerName,
			ContainerConfig: ctrConfig,
			AuthProfile:     authProfile,
			ClaudeConfig:    &cfg.Claude,
			ProjectDir:      projectDir,
			HomeDir:         homeDir,
			Notify:          notifyEnabled,
			Vault:           kdbxVault,
			SSHHosts:        sshHosts,
		}); err != nil {
			return err
		}

		fmt.Printf("Task %d started (container: %s, stage: %s)\n", t.ID, t.ContainerID, nextStage)
		fmt.Printf("Log: %s\n", t.LogFile)

		follow, _ := cmd.Flags().GetBool("follow")
		if follow {
			_ = task.TailLog(t.LogFile, true, true)
			exitCode, err := runner.WaitForContainer(t.ContainerID)
			if err != nil {
				return err
			}
			t, _ = store.Load(t.ID)
			fmt.Printf("\nTask %d: %s\n", t.ID, t.Status)
			if exitCode != 0 {
				os.Exit(exitCode)
			}
		}
		return nil
	},
}

var taskStageCmd = &cobra.Command{
	Use:   "stage <id>",
	Short: "Show current workflow stage and what's next",
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
		if t.Repo == "" {
			return fmt.Errorf("task %d has no repo set", t.ID)
		}

		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		repoConfig := resolveRepoConfig(cfg, t.Repo)
		if repoConfig == nil {
			return fmt.Errorf("unknown repo: %s", t.Repo)
		}
		if repoConfig.Workflow == nil {
			fmt.Printf("Task %d: repo %s has no workflow\n", t.ID, t.Repo)
			return nil
		}

		wf := repoConfig.Workflow

		// Detect actual stage from worktree if path exists
		detectedStage := ""
		if t.WorktreePath != "" {
			detectedStage = task.DetectCurrentStage(wf, t.WorktreePath)
		}

		fmt.Printf("Task %d — %s (branch: %s)\n", t.ID, t.Repo, t.Branch)
		fmt.Printf("Workflow stages:\n")
		for i, stage := range wf.Stages {
			marker := "  "
			if stage.Name == t.Stage {
				marker = "▸ "
			} else if stage.Name == detectedStage && t.Stage == "" {
				marker = "▸ "
			}

			status := ""
			if stage.Name == t.Stage {
				status = fmt.Sprintf(" [%s]", t.StageStatus)
			} else if detectedStage != "" && task.StageIndex(wf, stage.Name) < task.StageIndex(wf, detectedStage) {
				status = " [complete]"
			} else if i < task.StageIndex(wf, t.Stage) && t.Stage != "" {
				status = " [complete]"
			}

			detect := ""
			if stage.Detect != "" {
				detect = fmt.Sprintf(" (detect: %s)", stage.Detect)
			}
			fmt.Printf("  %s%d. %s%s%s\n", marker, i+1, stage.Name, status, detect)
		}

		if t.Stage != "" {
			next := task.NextStage(wf, t.Stage)
			if next != "" {
				fmt.Printf("\nNext: myhome task next %d\n", t.ID)
			} else {
				fmt.Printf("\nWorkflow complete — run: myhome task done %d\n", t.ID)
			}
		} else if detectedStage != "" {
			fmt.Printf("\nStart: myhome task run %d\n", t.ID)
		}

		return nil
	},
}

func init() {
	taskAddCmd.Flags().String("domain", "", "Domain tag (work, dev, life)")
	taskAddCmd.Flags().String("repo", "", "Repository name (makes task runnable)")
	taskAddCmd.Flags().String("branch", "", "Branch name for worktree")
	taskAddCmd.Flags().StringSlice("param", nil, "Workflow parameter (key=value, repeatable)")
	taskRunCmd.Flags().String("auth", "", "Claude auth profile")
	taskRunCmd.Flags().String("container", "", "Container to use (default: from repo config)")
	taskRunCmd.Flags().String("remote", "", "Run on remote host instead of locally")
	taskRunCmd.Flags().String("repo", "", "Repository name (inline mode)")
	taskRunCmd.Flags().String("branch", "", "Branch name (inline mode)")
	taskRunCmd.Flags().String("prompt", "", "Prompt for Claude (inline mode, bypasses workflow)")
	taskRunCmd.Flags().String("domain", "", "Domain tag (inline mode)")
	taskRunCmd.Flags().StringSlice("param", nil, "Workflow parameter (key=value, repeatable)")
	taskRunCmd.Flags().BoolP("follow", "f", false, "Stream logs and block until task completes")
	taskNextCmd.Flags().String("auth", "", "Claude auth profile")
	taskNextCmd.Flags().String("container", "", "Container to use (default: from repo config)")
	taskNextCmd.Flags().BoolP("follow", "f", false, "Stream logs and block until task completes")
	taskDoneCmd.Flags().Bool("merge", false, "Merge locally via Worktrunk before removing worktree (default: push only)")
	taskListCmd.Flags().String("domain", "", "Filter by domain")
	taskListCmd.Flags().String("status", "", "Filter by status (open, running, done)")
	taskLogCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	taskLogCmd.Flags().Bool("raw", false, "Show raw NDJSON instead of formatted output")

	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskRunCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskLogCmd)
	taskCmd.AddCommand(taskDoneCmd)
	taskCmd.AddCommand(taskStopCmd)
	taskCmd.AddCommand(taskRmCmd)
	taskCmd.AddCommand(taskNextCmd)
	taskCmd.AddCommand(taskStageCmd)
}

func defaultTaskStore() (*task.Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return task.NewStore(filepath.Join(homeDir, "tasks"))
}

// hasVaultRefs returns true if any env values use vault:// references.
func hasVaultRefs(env map[string]string) bool {
	for _, v := range env {
		if strings.HasPrefix(v, "vault://") {
			return true
		}
	}
	return false
}

// parseParams converts ["key=value", ...] flags into a map.
func parseParams(flags []string) map[string]string {
	params := make(map[string]string, len(flags))
	for _, f := range flags {
		k, v, ok := strings.Cut(f, "=")
		if ok {
			params[k] = v
		}
	}
	return params
}

// resolveRepoConfig finds the Repo config matching a repo name.
func resolveRepoConfig(cfg *config.Config, repoName string) *config.Repo {
	for i, r := range cfg.Repos {
		base := filepath.Base(r.Path)
		if base == repoName || r.Path == repoName {
			return &cfg.Repos[i]
		}
		for _, seg := range strings.Split(r.Path, "/") {
			if seg == repoName {
				return &cfg.Repos[i]
			}
		}
	}
	return nil
}
