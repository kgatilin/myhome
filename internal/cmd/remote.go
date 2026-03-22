package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/remote"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage remote SSH + tmux sessions",
	Long:  "Run Claude on remote hosts via SSH and tmux sessions.",
}

var remoteRunCmd = &cobra.Command{
	Use:               "run <host> <repo> <prompt>",
	Short:             "Run Claude on remote host in a tmux session",
	Args:              cobra.ExactArgs(3),
	ValidArgsFunction: remoteNameCompletionFunc,
	RunE: func(cmd *cobra.Command, args []string) error {
		hostName := args[0]
		repo := args[1]
		prompt := args[2]
		authProfile, _ := cmd.Flags().GetString("auth")

		remoteCfg, err := loadRemote(hostName)
		if err != nil {
			return err
		}

		session, err := remote.Run(remoteCfg, repo, prompt, authProfile, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Session started: %s on %s\n", session, hostName)
		fmt.Printf("Attach with: myhome remote attach %s %s\n", hostName, session)
		return nil
	},
}

var remoteListCmd = &cobra.Command{
	Use:               "list <host>",
	Short:             "List tmux sessions on remote host",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: remoteNameCompletionFunc,
	RunE: func(cmd *cobra.Command, args []string) error {
		hostName := args[0]

		remoteCfg, err := loadRemote(hostName)
		if err != nil {
			return err
		}

		sessions, err := remote.List(remoteCfg, nil)
		if err != nil {
			return err
		}
		if len(sessions) == 0 {
			fmt.Printf("No tmux sessions on %s\n", hostName)
			return nil
		}

		fmt.Printf("%-30s %-8s %s\n", "SESSION", "WINDOWS", "CREATED")
		for _, s := range sessions {
			fmt.Printf("%-30s %-8d %s\n", s.Name, s.Windows, s.Created)
		}
		return nil
	},
}

var remoteAttachCmd = &cobra.Command{
	Use:   "attach <host> <session>",
	Short: "Attach to a remote tmux session",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		hostName := args[0]
		session := args[1]

		remoteCfg, err := loadRemote(hostName)
		if err != nil {
			return err
		}

		return remote.Attach(remoteCfg, session, nil)
	},
}

var remoteStopCmd = &cobra.Command{
	Use:   "stop <host> <session>",
	Short: "Stop a remote tmux session",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		hostName := args[0]
		session := args[1]

		remoteCfg, err := loadRemote(hostName)
		if err != nil {
			return err
		}

		if err := remote.Stop(remoteCfg, session, nil); err != nil {
			return err
		}
		fmt.Printf("Session %s stopped on %s\n", session, hostName)
		return nil
	},
}

func init() {
	remoteRunCmd.Flags().String("auth", "", "Claude auth profile")

	remoteCmd.AddCommand(remoteRunCmd)
	remoteCmd.AddCommand(remoteListCmd)
	remoteCmd.AddCommand(remoteAttachCmd)
	remoteCmd.AddCommand(remoteStopCmd)
}

// loadRemote loads a remote config by name from myhome.yml.
func loadRemote(name string) (config.Remote, error) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return config.Remote{}, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return config.Remote{}, fmt.Errorf("load config: %w", err)
	}
	r, ok := cfg.Remotes[name]
	if !ok {
		available := make([]string, 0, len(cfg.Remotes))
		for k := range cfg.Remotes {
			available = append(available, k)
		}
		return config.Remote{}, fmt.Errorf("unknown remote %q (available: %v)", name, available)
	}
	return r, nil
}

