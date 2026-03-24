package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/agent"
	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/container"
	"github.com/kgatilin/myhome/internal/daemon"
	"github.com/kgatilin/myhome/internal/task"
	"github.com/kgatilin/myhome/internal/vault"
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
		modeStr, _ := cmd.Flags().GetString("mode")
		prompt, _ := cmd.Flags().GetString("prompt")
		workDir, _ := cmd.Flags().GetString("work-dir")

		mode := agent.AgentMode(modeStr)
		if mode != agent.ModeContainer && mode != agent.ModeProcess {
			return fmt.Errorf("invalid mode %q: must be 'container' or 'process'", modeStr)
		}

		// Try daemon first (container mode only)
		if mode == agent.ModeContainer {
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
		}

		// Direct mode
		cfg, agentCfg, err := loadAgentConfig(name)
		if err != nil {
			return err
		}

		opts := agent.CreateOpts{
			Mode:    mode,
			Prompt:  prompt,
			WorkDir: workDir,
		}

		if mode == agent.ModeProcess {
			// Process mode doesn't need container runtime
			store, err := defaultAgentStore()
			if err != nil {
				return err
			}
			homeDir, _ := os.UserHomeDir()
			mgr := agent.NewManager(store, exec.Command, "", homeDir)
			if err := mgr.Create(name, agentCfg, cfg, opts); err != nil {
				return err
			}
			state, _ := store.Load(name)
			fmt.Printf("Agent %s created (process: PID %d)\n", name, state.PID)
			return nil
		}

		// Container mode
		mgr, err := newAgentManager(cfg)
		if err != nil {
			return err
		}
		if err := openVaultIfNeeded(mgr, agentCfg); err != nil {
			return err
		}
		if err := mgr.Create(name, agentCfg, cfg, opts); err != nil {
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

		states, err := store.List()
		if err != nil {
			return err
		}
		if len(states) == 0 {
			fmt.Println("No agents")
			return nil
		}

		// Detect runtime for container agents (best-effort)
		homeDir, _ := os.UserHomeDir()
		var runtime string
		cfg, _, _, cfgErr := loadContainerDeps()
		if cfgErr == nil {
			runtime, _ = container.DetectRuntime(cfg.ContainerRuntime)
		}
		mgr := agent.NewManager(store, exec.Command, runtime, homeDir)

		fmt.Printf("%-15s %-10s %-12s %-15s %-8s %s\n", "NAME", "MODE", "STATUS", "ID", "TURNS", "CREATED")
		for _, s := range states {
			refreshed, _ := mgr.RefreshStatus(s.Name)
			if refreshed != nil {
				s = refreshed
			}
			mode := string(s.Mode)
			if mode == "" {
				mode = "container"
			}
			id := s.ContainerID
			if s.Mode == agent.ModeProcess {
				id = fmt.Sprintf("PID %d", s.PID)
			} else if len(id) > 12 {
				id = id[:12]
			}
			fmt.Printf("%-15s %-10s %-12s %-15s %-8d %s\n",
				s.Name, mode, s.Status, id, s.NumTurns, s.CreatedAt.Format("2006-01-02 15:04"))
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
		mgr, err := newAgentManagerForExisting(name)
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
		raw, _ := cmd.Flags().GetBool("raw")
		return task.TailLog(state.LogFile, follow, !raw)
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
		mgr, err := newAgentManagerForExisting(name)
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
		if err := openVaultIfNeeded(mgr, agentCfg); err != nil {
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
		mgr, err := newAgentManagerForExisting(name)
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

var agentChatCmd = &cobra.Command{
	Use:               "chat <name>",
	Short:             "Interactive chat REPL with an agent",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: agentNameCompletionFunc,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		homeDir, _ := os.UserHomeDir()
		socketPath := daemon.SocketPath(homeDir)
		useDaemon := daemon.IsRunning(socketPath)

		var mgr *agent.Manager
		if !useDaemon {
			cfg, _, err := loadAgentConfig(name)
			if err != nil {
				return err
			}
			m, err := newAgentManager(cfg)
			if err != nil {
				return err
			}
			mgr = m
		}

		fmt.Printf("Chat with agent %s (type 'exit' or Ctrl-D to quit)\n", name)
		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("> ")
			if !scanner.Scan() {
				fmt.Println()
				return nil
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if line == "exit" || line == "quit" {
				return nil
			}

			var response string
			if useDaemon {
				resp, err := daemon.Call(socketPath, "send", map[string]string{
					"name":    name,
					"message": line,
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					continue
				}
				if resp.Error != "" {
					fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
					continue
				}
				json.Unmarshal(resp.Result, &response)
			} else {
				r, err := mgr.Send(name, line)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					continue
				}
				response = r
			}
			fmt.Println(response)
		}
	},
}

func init() {
	agentCreateCmd.Flags().String("mode", "container", "Agent mode: 'container' or 'process'")
	agentCreateCmd.Flags().String("prompt", "", "Initial prompt for process-mode agents")
	agentCreateCmd.Flags().String("work-dir", "", "Override work directory (process mode)")

	agentLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	agentLogsCmd.Flags().Bool("raw", false, "Show raw NDJSON instead of formatted output")

	agentCmd.AddCommand(agentCreateCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentSendCmd)
	agentCmd.AddCommand(agentChatCmd)
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

// openVaultIfNeeded opens the KeePassXC vault and sets it on the agent manager
// if the agent config requires SSH key or vault:// secret injection.
func openVaultIfNeeded(mgr *agent.Manager, agentCfg config.AgentConfig) error {
	needsVault := agentCfg.Identity.SSH != "" || len(agentCfg.Secrets.Vault) > 0
	if !needsVault {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dbPath := vault.DefaultVaultPath(homeDir)
	keyFile := vault.DefaultKeyFile(homeDir)

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("vault not found at %s (required for agent SSH/secrets)", dbPath)
	}

	password, err := promptPassword("Enter vault master password: ")
	if err != nil {
		return err
	}

	v, err := vault.OpenKDBX(dbPath, keyFile, password)
	if err != nil {
		return fmt.Errorf("opening vault: %w", err)
	}

	mgr.Vault = v
	return nil
}

// newAgentManagerForExisting creates an agent.Manager appropriate for an already-created agent.
// For process-mode agents, no container runtime is needed.
func newAgentManagerForExisting(name string) (*agent.Manager, error) {
	store, err := defaultAgentStore()
	if err != nil {
		return nil, err
	}
	state, err := store.Load(name)
	if err != nil {
		return nil, err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	if state.Mode == agent.ModeProcess {
		return agent.NewManager(store, exec.Command, "", homeDir), nil
	}
	// Container mode — need runtime
	cfg, _, err := loadAgentConfig(name)
	if err != nil {
		return nil, err
	}
	runtime, err := container.DetectRuntime(cfg.ContainerRuntime)
	if err != nil {
		return nil, err
	}
	return agent.NewManager(store, exec.Command, runtime, homeDir), nil
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
