package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/agent"
	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/container"
	"github.com/kgatilin/myhome/internal/daemon"
	"github.com/kgatilin/myhome/internal/task"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage persistent Claude agents",
	Long:  "Create, manage, and interact with persistent Claude agents running in Docker containers.",
}

var agentCreateCmd = &cobra.Command{
	Use:               "create <name>",
	Short:             "Create and start an agent from config",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: agentNameCompletionFunc,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Try daemon first
		homeDir, _ := os.UserHomeDir()
		socketPath := daemon.SocketPath(homeDir)
		if daemon.IsRunning(socketPath) {
			resp, err := daemon.Call(socketPath, "create", map[string]string{"name": name})
			if err != nil {
				return err
			}
			if resp.Error != "" {
				return fmt.Errorf("%s", resp.Error)
			}
			fmt.Printf("Agent %s created (via daemon)\n", name)
			return nil
		}

		// Direct mode (no daemon)
		cfg, agentCfg, err := loadAgentConfig(name)
		if err != nil {
			return err
		}
		mgr, err := newAgentManager(cfg)
		if err != nil {
			return err
		}
		if err := mgr.Create(name, agentCfg, cfg); err != nil {
			return err
		}
		store, _ := defaultAgentStore()
		state, _ := store.Load(name)
		cid := state.ContainerID
		if len(cid) > 12 {
			cid = cid[:12]
		}
		fmt.Printf("Agent %s created (container: %s)\n", name, cid)
		return nil
	},
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agents with status",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := defaultAgentStore()
		if err != nil {
			return err
		}

		cfg, _, _, err := loadContainerDeps()
		if err != nil {
			return err
		}

		states, err := store.List()
		if err != nil {
			return err
		}
		if len(states) == 0 {
			fmt.Println("No agents")
			return nil
		}

		// Refresh status from container runtime
		homeDir, _ := os.UserHomeDir()
		runtime, _ := container.DetectRuntime(cfg.ContainerRuntime)
		mgr := agent.NewManager(store, exec.Command, runtime, homeDir)

		fmt.Printf("%-15s %-12s %-15s %-8s %s\n", "NAME", "STATUS", "CONTAINER", "TURNS", "CREATED")
		for _, s := range states {
			refreshed, _ := mgr.RefreshStatus(s.Name)
			if refreshed != nil {
				s = refreshed
			}
			cid := s.ContainerID
			if len(cid) > 12 {
				cid = cid[:12]
			}
			fmt.Printf("%-15s %-12s %-15s %-8d %s\n",
				s.Name, s.Status, cid, s.NumTurns, s.CreatedAt.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

var agentSendCmd = &cobra.Command{
	Use:   "send <name> <message>",
	Short: "Send a message to an agent and print the response",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		message := args[1]

		// Try daemon first
		homeDir, _ := os.UserHomeDir()
		socketPath := daemon.SocketPath(homeDir)
		if daemon.IsRunning(socketPath) {
			resp, err := daemon.Call(socketPath, "send", map[string]string{
				"name":    name,
				"message": message,
			})
			if err != nil {
				return err
			}
			if resp.Error != "" {
				return fmt.Errorf("%s", resp.Error)
			}
			var result string
			json.Unmarshal(resp.Result, &result)
			fmt.Println(result)
			return nil
		}

		// Direct mode
		cfg, _, err := loadAgentConfig(name)
		if err != nil {
			return err
		}
		mgr, err := newAgentManager(cfg)
		if err != nil {
			return err
		}
		response, err := mgr.Send(name, message)
		if err != nil {
			return err
		}
		fmt.Println(response)
		return nil
	},
}

var agentLogsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "Stream agent logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		store, err := defaultAgentStore()
		if err != nil {
			return err
		}
		state, err := store.Load(name)
		if err != nil {
			return err
		}
		if state.LogFile == "" {
			return fmt.Errorf("agent %q has no log file", name)
		}
		follow, _ := cmd.Flags().GetBool("follow")
		return task.TailLog(state.LogFile, follow)
	},
}

var agentStopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Gracefully stop an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Try daemon first
		homeDir, _ := os.UserHomeDir()
		socketPath := daemon.SocketPath(homeDir)
		if daemon.IsRunning(socketPath) {
			resp, err := daemon.Call(socketPath, "stop", map[string]string{"name": name})
			if err != nil {
				return err
			}
			if resp.Error != "" {
				return fmt.Errorf("%s", resp.Error)
			}
			fmt.Printf("Agent %s stopped\n", name)
			return nil
		}

		// Direct mode
		cfg, _, err := loadAgentConfig(name)
		if err != nil {
			return err
		}
		mgr, err := newAgentManager(cfg)
		if err != nil {
			return err
		}
		if err := mgr.Stop(name); err != nil {
			return err
		}
		fmt.Printf("Agent %s stopped\n", name)
		return nil
	},
}

var agentRestartCmd = &cobra.Command{
	Use:   "restart <name>",
	Short: "Stop and restart an agent (resumes session)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg, agentCfg, err := loadAgentConfig(name)
		if err != nil {
			return err
		}
		mgr, err := newAgentManager(cfg)
		if err != nil {
			return err
		}
		if err := mgr.Restart(name, agentCfg, cfg); err != nil {
			return err
		}
		fmt.Printf("Agent %s restarted\n", name)
		return nil
	},
}

var agentRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Stop and remove an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg, _, err := loadAgentConfig(name)
		if err != nil {
			return err
		}
		mgr, err := newAgentManager(cfg)
		if err != nil {
			return err
		}
		if err := mgr.Remove(name); err != nil {
			return err
		}
		fmt.Printf("Agent %s removed\n", name)
		return nil
	},
}

func init() {
	agentLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")

	agentCmd.AddCommand(agentCreateCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentSendCmd)
	agentCmd.AddCommand(agentLogsCmd)
	agentCmd.AddCommand(agentStopCmd)
	agentCmd.AddCommand(agentRestartCmd)
	agentCmd.AddCommand(agentRmCmd)
}

// loadAgentConfig loads the full config and the named agent's config.
func loadAgentConfig(name string) (*config.Config, config.AgentConfig, error) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil, config.AgentConfig{}, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, config.AgentConfig{}, fmt.Errorf("load config: %w", err)
	}
	agentCfg, ok := cfg.Agents[name]
	if !ok {
		available := make([]string, 0, len(cfg.Agents))
		for k := range cfg.Agents {
			available = append(available, k)
		}
		return nil, config.AgentConfig{}, fmt.Errorf("unknown agent %q (available: %v)", name, available)
	}
	return cfg, agentCfg, nil
}

// newAgentManager creates an agent.Manager from the loaded config.
func newAgentManager(cfg *config.Config) (*agent.Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	runtime, err := container.DetectRuntime(cfg.ContainerRuntime)
	if err != nil {
		return nil, err
	}
	store, err := defaultAgentStore()
	if err != nil {
		return nil, err
	}
	return agent.NewManager(store, exec.Command, runtime, homeDir), nil
}

// defaultAgentStore returns a Store rooted at ~/.myhome/agents/.
func defaultAgentStore() (*agent.Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return agent.NewStore(filepath.Join(homeDir, ".myhome", "agents"))
}

// agentNameCompletionFunc provides shell completion for agent names.
func agentNameCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(cfg.Agents))
	for name := range cfg.Agents {
		names = append(names, name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
