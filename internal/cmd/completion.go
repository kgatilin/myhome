package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for myhome.

To load completions:

  Zsh:
    # Add to ~/.zshrc:
    eval "$(myhome completion zsh)"
    # Or generate to fpath:
    myhome completion zsh > "${fpath[1]}/_myhome"

  Bash:
    source <(myhome completion bash)

  Fish:
    myhome completion fish | source`,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

// repoNameCompletionFunc provides dynamic tab-completion for repo names from myhome.yml.
func repoNameCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	seen := make(map[string]bool)
	for _, r := range cfg.Repos {
		base := filepath.Base(r.Path)
		if seen[base] {
			// Name conflict: use full path for both.
			names = append(names, r.Path)
		} else {
			names = append(names, base)
			seen[base] = true
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// remoteNameCompletionFunc provides dynamic tab-completion for remote names.
func remoteNameCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for k := range cfg.Remotes {
		names = append(names, k)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// envNameCompletionFunc provides dynamic tab-completion for environment names.
func envNameCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for k := range cfg.Envs {
		names = append(names, k)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// containerNameCompletionFunc provides dynamic tab-completion for container names.
func containerNameCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for k := range cfg.Containers {
		names = append(names, k)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
