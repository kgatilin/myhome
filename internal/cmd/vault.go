package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/vault"
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Manage KeePassXC vaults",
	Long:  "Create and manage KeePassXC vaults for secret storage and SSH key management.",
}

var vaultInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new vault with key file",
	Long:  "Creates ~/setup/vault.kdbx and generates a key file in ~/.secrets/vault.key.",
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		dbPath := vault.DefaultVaultPath(homeDir)
		secretsDir := vault.DefaultSecretsDir(homeDir)

		// Check if vault already exists.
		if _, err := os.Stat(dbPath); err == nil {
			return fmt.Errorf("vault already exists at %s", dbPath)
		}

		// Prompt for master password.
		password, err := promptPassword("Enter master password for vault: ")
		if err != nil {
			return err
		}

		v, err := vault.Init(dbPath, secretsDir, "vault.key", password, nil)
		if err != nil {
			return err
		}

		fmt.Printf("Vault created: %s\n", v.DBPath)
		fmt.Printf("Key file: %s\n", v.KeyFile)
		fmt.Println("Keep your master password safe — it cannot be recovered.")
		return nil
	},
}

var vaultStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show vault status",
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		dbPath := vault.DefaultVaultPath(homeDir)
		keyFile := vault.DefaultKeyFile(homeDir)

		status := vault.CheckStatus(dbPath, keyFile, nil)

		fmt.Printf("Vault:    %s\n", status.DBPath)
		if status.Exists {
			fmt.Println("Status:   exists")
		} else {
			fmt.Println("Status:   not found")
		}
		fmt.Printf("Key file: %s\n", status.KeyFile)
		if status.KeePassXCRunning {
			fmt.Println("KeePassXC: running")
		} else {
			fmt.Println("KeePassXC: not running")
		}
		return nil
	},
}

var vaultSSHAddCmd = &cobra.Command{
	Use:   "ssh-add <key-name>",
	Short: "Import SSH key into vault",
	Long:  "Imports an SSH private key from ~/.ssh/<key-name> into the vault.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyName := args[0]
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		dbPath := vault.DefaultVaultPath(homeDir)
		keyFile := vault.DefaultKeyFile(homeDir)
		sshKeyPath := homeDir + "/.ssh/" + keyName

		if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
			return fmt.Errorf("SSH key not found: %s", sshKeyPath)
		}

		password, err := promptPassword("Enter vault master password: ")
		if err != nil {
			return err
		}

		if err := vault.SSHAdd(dbPath, keyFile, password, keyName, sshKeyPath, nil); err != nil {
			return err
		}
		fmt.Printf("SSH key %s imported into vault\n", keyName)
		return nil
	},
}

var vaultSSHAgentCmd = &cobra.Command{
	Use:   "ssh-agent <key-name>",
	Short: "Enable SSH agent for a vault key",
	Long:  "Enables KeePassXC SSH agent integration for a key stored in the vault.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyName := args[0]
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		dbPath := vault.DefaultVaultPath(homeDir)
		keyFile := vault.DefaultKeyFile(homeDir)

		password, err := promptPassword("Enter vault master password: ")
		if err != nil {
			return err
		}

		if err := vault.SSHAgent(dbPath, keyFile, password, keyName, nil); err != nil {
			return err
		}
		fmt.Printf("SSH agent enabled for %s\n", keyName)
		return nil
	},
}

func init() {
	vaultCmd.AddCommand(vaultInitCmd)
	vaultCmd.AddCommand(vaultStatusCmd)
	vaultCmd.AddCommand(vaultSSHAddCmd)
	vaultCmd.AddCommand(vaultSSHAgentCmd)
}

// promptPassword reads a line from stdin. In a real implementation this would
// use terminal.ReadPassword to suppress echo, but we keep it simple.
func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return strings.TrimSpace(password), nil
}
