package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kgatilin/myhome/internal/auth"
	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/gitignore"
	"github.com/kgatilin/myhome/internal/identity"
	"github.com/kgatilin/myhome/internal/packages"
	"github.com/kgatilin/myhome/internal/platform"
	"github.com/kgatilin/myhome/internal/repo"
	"github.com/kgatilin/myhome/internal/tools"
	"github.com/kgatilin/myhome/internal/vault"
	"github.com/kgatilin/myhome/internal/workspace"
	"github.com/spf13/cobra"
)

var initEnv string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap the workspace for an environment",
	Long: `Full machine bootstrap:
  1. Load or create myhome.yml
  2. Resolve environment and save to state file
  3. Generate ~/.gitignore
  4. Generate ~/.ssh/config
  5. Generate ~/.gitconfig identity includes
  6. Generate ~/.mise.toml and run mise install
  7. Install system packages (brew/apt)
  8. Clone repos matching the environment
  9. Check vault status and prompt to create if missing
 10. Generate VSCode workspace files`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, err := config.DefaultConfigPath()
		if err != nil {
			return fmt.Errorf("resolve config path: %w", err)
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home dir: %w", err)
		}
		statePath, err := config.DefaultStatePath()
		if err != nil {
			return fmt.Errorf("resolve state path: %w", err)
		}
		plat, err := platform.Detect()
		if err != nil {
			return fmt.Errorf("detect platform: %w", err)
		}
		return runInit(initEnv, configPath, homeDir, statePath, plat)
	},
}

func init() {
	initCmd.Flags().StringVar(&initEnv, "env", "full", "Environment to initialize (base, work, personal, full)")
}

// runInit orchestrates the full init sequence. Each step is best-effort:
// failures are printed but do not stop subsequent steps (except for config
// loading and env resolution which are fatal since all other steps depend on them).
func runInit(envName, configPath, homeDir, statePath string, plat platform.Platform) error {
	var failed bool

	// Step 1: Load or create config
	fmt.Println("[1/11] Loading config...")
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	fmt.Printf("  config loaded from %s\n", configPath)

	// Step 2: Resolve environment
	fmt.Printf("[2/11] Resolving environment %q...\n", envName)
	resolved, err := cfg.ResolveEnv(envName)
	if err != nil {
		return fmt.Errorf("resolve env: %w", err)
	}
	fmt.Printf("  resolved: %d repos, %d tools\n", len(resolved.Repos), len(resolved.Tools))

	// Step 3: Save env to state file
	fmt.Println("[3/11] Saving state...")
	if err := saveStateToPath(envName, statePath); err != nil {
		fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		failed = true
	} else {
		fmt.Printf("  current env set to %q\n", envName)
	}

	// Step 4: Generate .gitignore
	fmt.Println("[4/11] Generating .gitignore...")
	if err := gitignore.Write(cfg, homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		failed = true
	} else {
		fmt.Println("  .gitignore written")
	}

	// Step 5: Generate .ssh/config
	fmt.Println("[5/11] Generating .ssh/config...")
	if err := auth.WriteSSHConfig(cfg.Auth, homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		failed = true
	} else {
		fmt.Println("  .ssh/config written")
	}

	// Step 6: Generate .gitconfig identity
	fmt.Println("[6/11] Generating .gitconfig identity...")
	if len(cfg.Auth) > 0 {
		if err := identity.WriteGitconfig(homeDir, "", "", nil); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
			failed = true
		} else {
			fmt.Println("  .gitconfig written")
		}
	} else {
		fmt.Println("  skipped (no auth entries)")
	}

	// Step 7: Generate .mise.toml + run mise install
	fmt.Println("[7/11] Syncing dev tools (mise)...")
	if err := tools.Sync(resolved.Tools, homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		failed = true
	} else {
		fmt.Println("  tools synced")
	}

	// Step 8: Install system packages
	fmt.Println("[8/11] Syncing system packages...")
	if err := packages.Sync(resolved.Packages, plat); err != nil {
		fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		failed = true
	} else {
		fmt.Println("  packages synced")
	}

	// Step 9: Clone repos
	fmt.Println("[9/11] Cloning repos...")
	if err := repo.Sync(resolved, homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		failed = true
	} else {
		fmt.Printf("  repos synced (%d configured)\n", len(resolved.Repos))
	}

	// Step 10: Check vault status.
	fmt.Println("[10/11] Checking vault...")
	dbPath := vault.DefaultVaultPath(homeDir)
	keyFile := vault.DefaultKeyFile(homeDir)
	vaultStatus := vault.CheckStatus(dbPath, keyFile, nil)
	if vaultStatus.Exists {
		fmt.Println("  vault found")
	} else {
		fmt.Println("  vault not found — run 'myhome vault init' to create one")
	}

	// Step 11: Generate VSCode workspace files
	fmt.Println("[11/11] Generating VSCode workspace files...")
	if err := workspace.WriteAll(cfg, homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		failed = true
	} else {
		fmt.Println("  workspace files written")
	}

	if failed {
		fmt.Println("\nInit completed with errors (see above)")
	} else {
		fmt.Println("\nInit completed successfully")
	}
	return nil
}

// loadOrCreateConfig loads the config from path, or creates a minimal default if it doesn't exist.
func loadOrCreateConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}
	if !os.IsNotExist(unwrapAll(err)) {
		return nil, err
	}

	// Config doesn't exist — create a minimal default
	cfg = &config.Config{
		Envs: map[string]config.Env{
			"base": {Include: []string{"base"}},
			"full": {Include: []string{"base"}},
		},
		Tools:            map[string]map[string]string{"base": {"go": "latest"}},
		Packages:         map[string]config.PackageSet{"base": {Brew: []string{"git"}, Apt: []string{"git"}}},
		Auth:             map[string]config.AuthHost{},
		ContainerRuntime: "auto",
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	if err := cfg.Save(path); err != nil {
		return nil, fmt.Errorf("save default config: %w", err)
	}
	fmt.Printf("  created default config at %s\n", path)
	return cfg, nil
}

// saveStateToPath loads the state file, sets CurrentEnv, and saves it back.
func saveStateToPath(envName, statePath string) error {
	state, err := config.LoadState(statePath)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	state.CurrentEnv = envName
	if err := state.Save(statePath); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

// unwrapAll unwraps all layers of error wrapping to get the root cause.
func unwrapAll(err error) error {
	for {
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return err
		}
		inner := u.Unwrap()
		if inner == nil {
			return err
		}
		err = inner
	}
}
