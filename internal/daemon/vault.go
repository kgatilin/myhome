package daemon

import (
	"fmt"
	"sync"

	"github.com/kgatilin/myhome/internal/vault"
)

// VaultCache holds an unlocked vault in memory for the lifetime of the daemon.
// The master password is only needed once at daemon startup; after that, secrets
// are served from the cached vault without re-prompting.
type VaultCache struct {
	mu    sync.RWMutex
	vault vault.Reader
}

// Unlock opens the vault and caches the decrypted database in memory.
func (vc *VaultCache) Unlock(dbPath, keyFile, password string) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	v, err := vault.OpenKDBX(dbPath, keyFile, password)
	if err != nil {
		return fmt.Errorf("unlock vault: %w", err)
	}
	vc.vault = v
	return nil
}

// Get returns a secret from the cached vault.
func (vc *VaultCache) Get(entryName string) (string, error) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	if vc.vault == nil {
		return "", fmt.Errorf("vault is not unlocked")
	}
	return vc.vault.Get(entryName)
}

// Vault returns the underlying vault Reader, or nil if not unlocked.
func (vc *VaultCache) Vault() vault.Reader {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.vault
}

// IsUnlocked returns true if the vault has been unlocked.
func (vc *VaultCache) IsUnlocked() bool {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.vault != nil
}

// Lock clears the cached vault from memory.
func (vc *VaultCache) Lock() {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.vault = nil
}
