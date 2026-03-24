package daemon

import (
	"os"
	"path/filepath"
	"testing"

	gokeepasslib "github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

func createTestVault(t *testing.T, password string) string {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.kdbx")

	db := gokeepasslib.NewDatabase(
		gokeepasslib.WithDatabaseKDBXVersion4(),
	)
	db.Credentials = gokeepasslib.NewPasswordCredentials(password)

	rootGroup := gokeepasslib.NewGroup()
	rootGroup.Name = "Root"

	entry := gokeepasslib.NewEntry()
	entry.Values = append(entry.Values, gokeepasslib.ValueData{
		Key: "Title", Value: gokeepasslib.V{Content: "test-secret"},
	})
	entry.Values = append(entry.Values, gokeepasslib.ValueData{
		Key: "Password", Value: gokeepasslib.V{Content: "secret-value", Protected: w.NewBoolWrapper(true)},
	})
	rootGroup.Entries = append(rootGroup.Entries, entry)
	db.Content.Root.Groups = []gokeepasslib.Group{rootGroup}

	if err := db.LockProtectedEntries(); err != nil {
		t.Fatalf("lock: %v", err)
	}

	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()

	if err := gokeepasslib.NewEncoder(f).Encode(db); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return dbPath
}

func TestVaultCache(t *testing.T) {
	dbPath := createTestVault(t, "testpass")

	vc := &VaultCache{}

	if vc.IsUnlocked() {
		t.Fatal("should not be unlocked initially")
	}

	_, err := vc.Get("test-secret")
	if err == nil {
		t.Fatal("Get should fail when locked")
	}

	if err := vc.Unlock(dbPath, "", "testpass"); err != nil {
		t.Fatalf("Unlock() error: %v", err)
	}

	if !vc.IsUnlocked() {
		t.Fatal("should be unlocked after Unlock()")
	}

	got, err := vc.Get("test-secret")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != "secret-value" {
		t.Errorf("Get() = %q, want %q", got, "secret-value")
	}

	if vc.Vault() == nil {
		t.Fatal("Vault() should return non-nil when unlocked")
	}

	vc.Lock()
	if vc.IsUnlocked() {
		t.Fatal("should not be unlocked after Lock()")
	}
}

func TestVaultCacheWrongPassword(t *testing.T) {
	dbPath := createTestVault(t, "testpass")

	vc := &VaultCache{}
	err := vc.Unlock(dbPath, "", "wrongpass")
	if err == nil {
		t.Fatal("Unlock() should fail with wrong password")
	}
}
