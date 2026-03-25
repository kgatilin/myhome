package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/platform"
	"github.com/kgatilin/myhome/internal/repo"
	"github.com/kgatilin/myhome/internal/selfupdate"
	"github.com/kgatilin/myhome/internal/service"
	"github.com/kgatilin/myhome/internal/tools"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Run full sync pipeline (self-update, tools, repos, services)",
	Long:  "Run the full myhome sync pipeline: pull config, rebuild binary, install tools, clone/build repos, start services.\nUse flags to run only specific steps.",
	RunE: func(cmd *cobra.Command, args []string) error {
		doSelf, _ := cmd.Flags().GetBool("self")
		doTools, _ := cmd.Flags().GetBool("tools")
		doRepos, _ := cmd.Flags().GetBool("repos")
		doServices, _ := cmd.Flags().GetBool("services")

		// Default: run all steps if no flags specified
		if !doSelf && !doTools && !doRepos && !doServices {
			doSelf = true
			doTools = true
			doRepos = true
			doServices = true
		}

		// Step 0: pull home repo to get latest config
		fmt.Println("==> Pulling home repo")
		if err := pullHomeRepo(); err != nil {
			fmt.Printf("Warning: git pull failed: %v (continuing)\n", err)
		}

		if doSelf {
			fmt.Println("==> Self-update")
			if err := runSelfUpdate(); err != nil {
				fmt.Printf("Warning: self-update failed: %v (continuing)\n", err)
			} else {
				// Re-exec the new binary with remaining steps
				// so tools/repos/services run with the updated code
				newBin := selfupdate.InstallPath()
				var remainingArgs []string
				remainingArgs = append(remainingArgs, "sync")
				if doTools {
					remainingArgs = append(remainingArgs, "--tools")
				}
				if doRepos {
					remainingArgs = append(remainingArgs, "--repos")
				}
				if doServices {
					remainingArgs = append(remainingArgs, "--services")
				}
				fmt.Printf("Re-executing %s %v\n", newBin, remainingArgs)
				err := syscall.Exec(newBin, append([]string{"myhome"}, remainingArgs...), os.Environ())
				// syscall.Exec replaces the process — if we get here, it failed
				fmt.Printf("Warning: re-exec failed: %v (continuing with current binary)\n", err)
			}
		}

		if doTools {
			fmt.Println("==> Tools sync")
			if err := runToolsSync(); err != nil {
				fmt.Printf("Warning: tools sync failed: %v (continuing)\n", err)
			}
		}

		if doRepos {
			fmt.Println("==> Repo sync")
			if err := runRepoSync(); err != nil {
				fmt.Printf("Warning: repo sync failed: %v (continuing)\n", err)
			}
		}

		if doServices {
			fmt.Println("==> Starting services")
			if err := runServiceStart(); err != nil {
				fmt.Printf("Warning: service start failed: %v (continuing)\n", err)
			}
		}

		fmt.Println("Sync complete")
		return nil
	},
}

func runSelfUpdate() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	var repoPaths []string
	cfgPath, err := config.DefaultConfigPath()
	if err == nil {
		if cfg, loadErr := config.Load(cfgPath); loadErr == nil {
			for _, r := range cfg.Repos {
				repoPaths = append(repoPaths, r.Path)
			}
		}
	}

	sourceDir, err := selfupdate.FindSourceDir(homeDir, repoPaths)
	if err != nil {
		return err
	}
	fmt.Printf("Source repo: %s\n", sourceDir)
	return selfupdate.Run(sourceDir)
}

func runToolsSync() error {
	cfg, state, homeDir, err := loadRepoContext()
	if err != nil {
		return err
	}
	env, err := cfg.ResolveEnv(state.CurrentEnv)
	if err != nil {
		return err
	}
	return tools.Sync(env.Tools, homeDir)
}

func runRepoSync() error {
	cfg, state, homeDir, err := loadRepoContext()
	if err != nil {
		return err
	}
	env, err := cfg.ResolveEnv(state.CurrentEnv)
	if err != nil {
		return err
	}
	return repo.Sync(env, homeDir)
}

func runServiceStart() error {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	plat, _ := platform.Detect()
	return service.StartAll(cfg.Services, plat, service.WithConfig(cfg))
}

func pullHomeRepo() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "-C", homeDir, "pull", "--ff-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func init() {
	syncCmd.Flags().Bool("self", false, "Only run self-update step")
	syncCmd.Flags().Bool("tools", false, "Only run tools sync step")
	syncCmd.Flags().Bool("repos", false, "Only run repo sync step")
	syncCmd.Flags().Bool("services", false, "Only run service start step")
}
