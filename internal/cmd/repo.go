package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/gitignore"
	"github.com/kgatilin/myhome/internal/repo"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories",
	Long:  "List, sync, add, remove repos. Manage worktrees via 'myhome repo <name> wt'.",
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List repos for current env",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, state, homeDir, err := loadRepoContext()
		if err != nil {
			return err
		}
		env, err := cfg.ResolveEnv(state.CurrentEnv)
		if err != nil {
			return err
		}
		statuses, err := repo.List(env, homeDir)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			status := "not cloned"
			if s.Cloned {
				status = "ok"
				if s.Dirty {
					status = "dirty"
				}
			}
			fmt.Printf("%-40s [%s] (%s)\n", s.Path, status, s.Env)
		}
		return nil
	},
}

var repoSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Clone missing repos for current env",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, state, homeDir, err := loadRepoContext()
		if err != nil {
			return err
		}
		env, err := cfg.ResolveEnv(state.CurrentEnv)
		if err != nil {
			return err
		}
		return repo.Sync(env, homeDir)
	},
}

var repoAddCmd = &cobra.Command{
	Use:   "add <path> [url]",
	Short: "Add repo to manifest",
	Args:  cobra.RangeArgs(1, 2),
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
		path := args[0]
		var url string
		if len(args) > 1 {
			url = args[1]
		}
		env, _ := cmd.Flags().GetString("env")
		if err := repo.Add(cfg, path, url, env, homeDir); err != nil {
			return err
		}
		if err := cfg.Save(cfgPath); err != nil {
			return err
		}
		if err := gitignore.Write(cfg, homeDir); err != nil {
			return err
		}
		fmt.Printf("Added repo %s\n", path)
		return nil
	},
}

var repoRmCmd = &cobra.Command{
	Use:   "rm <path>",
	Short: "Remove repo from manifest",
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
		if err := repo.Rm(cfg, args[0]); err != nil {
			return err
		}
		if err := cfg.Save(cfgPath); err != nil {
			return err
		}
		if err := gitignore.Write(cfg, homeDir); err != nil {
			return err
		}
		fmt.Printf("Removed repo %s\n", args[0])
		return nil
	},
}

var repoWtListCmd = &cobra.Command{
	Use:   "wt",
	Short: "Cross-repo worktree overview",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("myhome repo wt list")
		fmt.Println("Not implemented yet")
		return nil
	},
}

func init() {
	repoAddCmd.Flags().String("env", "", "Environment tag for the repo")
	repoCmd.AddCommand(repoListCmd)
	repoCmd.AddCommand(repoSyncCmd)
	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoRmCmd)
	repoCmd.AddCommand(repoWtListCmd)
}

func loadRepoContext() (*config.Config, *config.State, string, error) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil, nil, "", err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, "", err
	}
	statePath, err := config.DefaultStatePath()
	if err != nil {
		return nil, nil, "", err
	}
	state, err := config.LoadState(statePath)
	if err != nil {
		return nil, nil, "", err
	}
	if state.CurrentEnv == "" {
		return nil, nil, "", fmt.Errorf("no environment set — run 'myhome init --env <env>' first")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, "", err
	}
	return cfg, state, homeDir, nil
}
