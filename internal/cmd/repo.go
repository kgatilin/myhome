package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/gitignore"
	"github.com/kgatilin/myhome/internal/repo"
	"github.com/kgatilin/myhome/internal/worktree"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories",
	Long:  "List, sync, add, remove repos. Manage worktrees via 'myhome repo <name> wt'.",
	// Handle dynamic subcommands: myhome repo <name> wt <action> [args]
	// When no known subcommand matches, args are passed here.
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return handleDynamicRepoCommand(args)
	},
	SilenceUsage: true,
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
		cfg, state, homeDir, err := loadRepoContext()
		if err != nil {
			return err
		}
		env, err := cfg.ResolveEnv(state.CurrentEnv)
		if err != nil {
			return err
		}
		wts, err := worktree.ListAll(env.Repos, homeDir)
		if err != nil {
			return err
		}
		if len(wts) == 0 {
			fmt.Println("No worktrees found.")
			return nil
		}
		for _, wt := range wts {
			fmt.Printf("%-15s %-25s %s\n", wt.RepoName, wt.Branch, wt.Path)
		}
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

// handleDynamicRepoCommand handles: myhome repo <name> wt <action> [args]
func handleDynamicRepoCommand(args []string) error {
	if len(args) < 2 || args[1] != "wt" {
		return fmt.Errorf("unknown repo subcommand: %s\nUsage: myhome repo <name> wt <create|list|rm> [args]", args[0])
	}

	repoName := args[0]
	cfg, _, homeDir, err := loadRepoContext()
	if err != nil {
		return err
	}

	r, err := repo.FindByName(cfg.Repos, repoName)
	if err != nil {
		return err
	}

	// myhome repo <name> wt — list worktrees for this repo
	if len(args) == 2 {
		return listRepoWorktrees(r, homeDir)
	}

	action := args[2]
	switch action {
	case "list":
		return listRepoWorktrees(r, homeDir)
	case "create":
		if len(args) < 4 {
			return fmt.Errorf("usage: myhome repo %s wt create <branch>", repoName)
		}
		branch := args[3]
		wtPath, err := worktree.Create(r, branch, homeDir)
		if err != nil {
			return err
		}
		fmt.Printf("Created worktree at %s\n", wtPath)
		return nil
	case "rm":
		if len(args) < 4 {
			return fmt.Errorf("usage: myhome repo %s wt rm <branch>", repoName)
		}
		branch := args[3]
		return worktree.Remove(r, branch, homeDir)
	default:
		return fmt.Errorf("unknown wt action: %s (expected create, list, rm)", action)
	}
}

func listRepoWorktrees(r *config.Repo, homeDir string) error {
	wts, err := worktree.ListForRepo(r, homeDir)
	if err != nil {
		return err
	}
	if len(wts) == 0 {
		fmt.Println("No worktrees found.")
		return nil
	}
	for _, wt := range wts {
		fmt.Printf("%-25s %s\n", wt.Branch, wt.Path)
	}
	return nil
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
