package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/platform"
	"github.com/kgatilin/myhome/internal/user"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage agent users",
}

var userCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create agent user with env-scoped access",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		env, _ := cmd.Flags().GetString("env")
		tmpl, _ := cmd.Flags().GetString("template")

		cfg, homeDir, plat, err := loadUserDeps()
		if err != nil {
			return err
		}

		// Build user config from flags, falling back to config file.
		userCfg, ok := cfg.Users[name]
		if !ok {
			userCfg = config.User{}
		}
		if env != "" {
			userCfg.Env = env
		}
		if tmpl != "" {
			userCfg.Template = tmpl
		}
		if userCfg.Env == "" {
			return fmt.Errorf("--env is required (or define user in myhome.yml)")
		}

		return user.Create(name, userCfg, cfg, plat, homeDir)
	},
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List managed users",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, plat, err := loadUserDeps()
		if err != nil {
			return err
		}
		statePath, err := config.DefaultStatePath()
		if err != nil {
			return err
		}
		state, err := config.LoadState(statePath)
		if err != nil {
			return err
		}

		users, err := user.List(cfg, state, plat)
		if err != nil {
			return err
		}
		if len(users) == 0 {
			fmt.Println("No agent users registered")
			return nil
		}

		fmt.Printf("%-20s %-12s %-20s %-10s %s\n", "NAME", "ENV", "TEMPLATE", "SERVICE", "HOME")
		for _, u := range users {
			status := "stopped"
			if u.Running {
				status = "running"
			}
			fmt.Printf("%-20s %-12s %-20s %-10s %s\n", u.Name, u.Env, u.Template, status, u.HomeDir)
		}
		return nil
	},
}

var userRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove agent user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			return fmt.Errorf("removing user %s will delete their home directory; use --force to confirm", name)
		}

		_, homeDir, plat, err := loadUserDeps()
		if err != nil {
			return err
		}
		return user.Remove(name, plat, homeDir)
	},
}

var userShellCmd = &cobra.Command{
	Use:   "shell <name>",
	Short: "Open shell as agent user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		sudoPath, err := exec.LookPath("sudo")
		if err != nil {
			return fmt.Errorf("sudo not found: %w", err)
		}
		return syscall.Exec(sudoPath, []string{"sudo", "-u", name, "-i"}, os.Environ())
	},
}

var userSyncCmd = &cobra.Command{
	Use:   "sync [name]",
	Short: "Sync agent's home repo",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")

		_, _, plat, err := loadUserDeps()
		if err != nil {
			return err
		}

		if all {
			statePath, err := config.DefaultStatePath()
			if err != nil {
				return err
			}
			state, err := config.LoadState(statePath)
			if err != nil {
				return err
			}
			return user.SyncAll(state, plat)
		}

		if len(args) == 0 {
			return fmt.Errorf("specify agent name or use --all")
		}
		return user.Sync(args[0], plat)
	},
}

func init() {
	userCreateCmd.Flags().String("env", "", "Environment for the agent")
	userCreateCmd.Flags().String("template", "", "Agent template to use")
	userRmCmd.Flags().Bool("force", false, "Confirm removal of user and home directory")
	userSyncCmd.Flags().Bool("all", false, "Sync all agent users")

	userCmd.AddCommand(userCreateCmd)
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userRmCmd)
	userCmd.AddCommand(userShellCmd)
	userCmd.AddCommand(userSyncCmd)
}

// loadUserDeps loads config, home dir, and detects platform.
func loadUserDeps() (*config.Config, string, platform.Platform, error) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil, "", nil, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", nil, fmt.Errorf("load config: %w", err)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, "", nil, err
	}
	plat, err := platform.Detect()
	if err != nil {
		return nil, "", nil, err
	}
	return cfg, homeDir, plat, nil
}
