package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/schedule"
)

var taskScheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage scheduled tasks",
	Long:  "Create and manage cron-based recurring tasks with template variables.",
}

var taskScheduleAddCmd = &cobra.Command{
	Use:   "add <prompt>",
	Short: "Create a scheduled task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt := args[0]
		cronExpr, _ := cmd.Flags().GetString("cron")
		containerName, _ := cmd.Flags().GetString("container")
		authProfile, _ := cmd.Flags().GetString("auth")
		workdir, _ := cmd.Flags().GetString("workdir")
		domain, _ := cmd.Flags().GetString("domain")
		schedID, _ := cmd.Flags().GetString("id")

		if cronExpr == "" {
			return fmt.Errorf("--cron is required")
		}
		if schedID == "" {
			return fmt.Errorf("--id is required")
		}

		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		sched := config.Schedule{
			ID:        schedID,
			Prompt:    prompt,
			Cron:      cronExpr,
			Container: containerName,
			Auth:      authProfile,
			Workdir:   workdir,
			Domain:    domain,
		}

		// Add to config.
		cfg.Schedules = append(cfg.Schedules, sched)
		if err := cfg.Save(cfgPath); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		myhomeBin, _ := os.Executable()
		if myhomeBin == "" {
			myhomeBin = "myhome"
		}

		if err := schedule.Install(sched, myhomeBin, homeDir, nil); err != nil {
			return err
		}

		fmt.Printf("Schedule %q created: %s\n", schedID, cronExpr)
		return nil
	},
}

var taskScheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scheduled tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		infos := schedule.List(cfg.Schedules, homeDir)
		if len(infos) == 0 {
			fmt.Println("No scheduled tasks")
			return nil
		}

		fmt.Printf("%-15s %-20s %-15s %-10s %s\n", "ID", "CRON", "CONTAINER", "INSTALLED", "PROMPT")
		for _, info := range infos {
			installed := "no"
			if info.Installed {
				installed = "yes"
			}
			prompt := info.Prompt
			if len(prompt) > 40 {
				prompt = prompt[:37] + "..."
			}
			fmt.Printf("%-15s %-20s %-15s %-10s %s\n", info.ID, info.Cron, info.Container, installed, prompt)
		}
		return nil
	},
}

var taskScheduleRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Remove a scheduled task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		schedID := args[0]

		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		// Remove from OS scheduler.
		if err := schedule.Remove(schedID, homeDir, nil); err != nil {
			return err
		}

		// Remove from config.
		var newSchedules []config.Schedule
		found := false
		for _, s := range cfg.Schedules {
			if s.ID == schedID {
				found = true
				continue
			}
			newSchedules = append(newSchedules, s)
		}
		if !found {
			return fmt.Errorf("schedule %q not found in config", schedID)
		}
		cfg.Schedules = newSchedules
		if err := cfg.Save(cfgPath); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Schedule %q removed\n", schedID)
		return nil
	},
}

func init() {
	taskScheduleAddCmd.Flags().String("cron", "", "Cron expression (e.g., '0 18 * * 1-5')")
	taskScheduleAddCmd.Flags().String("container", "claude-code", "Container to use")
	taskScheduleAddCmd.Flags().String("auth", "", "Claude auth profile")
	taskScheduleAddCmd.Flags().String("workdir", "", "Working directory")
	taskScheduleAddCmd.Flags().String("domain", "", "Domain tag")
	taskScheduleAddCmd.Flags().String("id", "", "Schedule identifier")

	taskScheduleCmd.AddCommand(taskScheduleAddCmd)
	taskScheduleCmd.AddCommand(taskScheduleListCmd)
	taskScheduleCmd.AddCommand(taskScheduleRmCmd)

	taskCmd.AddCommand(taskScheduleCmd)
}
