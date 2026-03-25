package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "myhome",
	Short: "Personal workspace manager",
	Long:  "Manage your home folder: repos, environments, tools, packages, auth, agent users.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(repoCmd)
	rootCmd.AddCommand(toolsCmd)
	rootCmd.AddCommand(packagesCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(containerCmd)
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(vaultCmd)
	rootCmd.AddCommand(remoteCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(adapterCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(selfUpdateCmd)
	rootCmd.AddCommand(syncCmd)
}
