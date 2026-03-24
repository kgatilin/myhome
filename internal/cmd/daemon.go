package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/daemon"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the myhome daemon (agent supervisor)",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon in the foreground",
	Long:  "Start the myhome daemon that supervises agents, handles health checks, and serves the CLI API.",
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		socketPath := daemon.SocketPath(homeDir)
		if daemon.IsRunning(socketPath) {
			return fmt.Errorf("daemon is already running on %s", socketPath)
		}

		d, err := daemon.New(daemon.Config{
			SocketPath: socketPath,
			HomeDir:    homeDir,
			ExecFn:     exec.Command,
		})
		if err != nil {
			return err
		}

		return d.Run()
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		socketPath := daemon.SocketPath(homeDir)
		if !daemon.IsRunning(socketPath) {
			fmt.Println("Daemon is not running")
			return nil
		}

		// Remove the socket file to trigger shutdown
		if err := os.Remove(socketPath); err != nil {
			return fmt.Errorf("removing socket: %w", err)
		}
		fmt.Println("Daemon stopped")
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if the daemon is running",
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		socketPath := daemon.SocketPath(homeDir)
		if daemon.IsRunning(socketPath) {
			fmt.Printf("Daemon is running (%s)\n", socketPath)
		} else {
			fmt.Println("Daemon is not running")
		}
		return nil
	},
}

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
}
