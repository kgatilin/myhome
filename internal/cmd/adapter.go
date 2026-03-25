package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/adapter/github"
	"github.com/kgatilin/myhome/internal/config"
)

var adapterCmd = &cobra.Command{
	Use:   "adapter",
	Short: "Manage external event adapters",
}

var adapterGitHubCmd = &cobra.Command{
	Use:   "github",
	Short: "GitHub issue polling adapter",
}

var adapterGitHubStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start polling GitHub for agent-ready issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		ghCfg := cfg.Adapters.GitHub
		if ghCfg == nil {
			return fmt.Errorf("adapters.github not configured in myhome.yml")
		}

		// Apply flag overrides.
		if interval, _ := cmd.Flags().GetDuration("interval"); interval > 0 {
			ghCfg.PollInterval = interval
		}
		if socket, _ := cmd.Flags().GetString("socket"); socket != "" {
			ghCfg.BusSocket = socket
		}

		// Defaults.
		if ghCfg.PollInterval == 0 {
			ghCfg.PollInterval = 60 * time.Second
		}
		if ghCfg.Label == "" {
			ghCfg.Label = "agent-ready"
		}
		if ghCfg.BusSocket == "" {
			ghCfg.BusSocket = "/tmp/deskd.sock"
		}
		if ghCfg.DefaultTarget == "" {
			ghCfg.DefaultTarget = "agent:dev"
		}

		stateDir, err := github.DefaultStateDir()
		if err != nil {
			return err
		}

		bus := github.NewBusClient(ghCfg.BusSocket)
		store := github.NewStateStore(stateDir)
		poller := github.NewPoller(ghCfg, bus, store)

		fmt.Fprintf(os.Stderr, "Starting GitHub adapter: %d repos, interval=%s, socket=%s\n",
			len(ghCfg.Repos), ghCfg.PollInterval, ghCfg.BusSocket)

		return poller.Run()
	},
}

var adapterGitHubStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show adapter state (posted issues)",
	RunE: func(cmd *cobra.Command, args []string) error {
		stateDir, err := github.DefaultStateDir()
		if err != nil {
			return err
		}

		store := github.NewStateStore(stateDir)
		state, err := store.Load()
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}

		if len(state.PostedIssues) == 0 {
			fmt.Println("No issues posted yet")
			return nil
		}

		fmt.Printf("%-30s %-20s %s\n", "ISSUE", "POSTED", "TITLE")
		for key, issue := range state.PostedIssues {
			fmt.Printf("%-30s %-20s %s\n", key, issue.PostedAt.Format(time.RFC3339), issue.Title)
		}
		return nil
	},
}

func init() {
	adapterGitHubStartCmd.Flags().Duration("interval", 0, "Poll interval (overrides config)")
	adapterGitHubStartCmd.Flags().String("socket", "", "Bus socket path (overrides config)")

	adapterGitHubCmd.AddCommand(adapterGitHubStartCmd)
	adapterGitHubCmd.AddCommand(adapterGitHubStatusCmd)
	adapterCmd.AddCommand(adapterGitHubCmd)
}
