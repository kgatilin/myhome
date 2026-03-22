package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/repo"
	"github.com/kgatilin/myhome/internal/worktree"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show workspace overview",
	Long:  "Current env, dirty repos, active worktrees, disk usage.",
	RunE: func(cmd *cobra.Command, args []string) error {
		statePath, err := config.DefaultStatePath()
		if err != nil {
			return err
		}
		state, err := config.LoadState(statePath)
		if err != nil {
			return err
		}
		if state.CurrentEnv == "" {
			fmt.Println("No environment set — run 'myhome init --env <env>' first")
			return nil
		}

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
		env, err := cfg.ResolveEnv(state.CurrentEnv)
		if err != nil {
			return err
		}

		fmt.Printf("Environment: %s\n", state.CurrentEnv)

		// Repo status
		statuses, err := repo.List(env, homeDir)
		if err != nil {
			return err
		}
		var total, cloned, dirty int
		for _, s := range statuses {
			total++
			if s.Cloned {
				cloned++
			}
			if s.Dirty {
				dirty++
			}
		}
		fmt.Printf("Repos: %d total, %d cloned, %d dirty\n", total, cloned, dirty)

		// Worktree count
		wts, err := worktree.ListAll(env.Repos, homeDir)
		if err == nil && len(wts) > 0 {
			// Subtract main worktrees (one per repo)
			extra := len(wts) - cloned
			if extra < 0 {
				extra = 0
			}
			fmt.Printf("Worktrees: %d active (beyond main)\n", extra)
		}

		// Disk usage of home
		homeSize := dirSize(homeDir)
		fmt.Printf("Home dir size: %s\n", formatBytes(homeSize))

		return nil
	},
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func formatBytes(b int64) string {
	const (
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	default:
		return fmt.Sprintf("%d bytes", b)
	}
}
