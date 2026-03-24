package agent

import (
	"os"
	"path/filepath"
	"testing"

	gokeepasslib "github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"

	"github.com/kgatilin/myhome/internal/vault"
)

// createTestVaultWithSSHKey creates a test vault with an SSH key stored as a binary attachment.
// The key is stored at "SSH Keys/<keyName>" with attachment name <keyName>.
func createTestVaultWithSSHKey(t *testing.T, keyName, keyData string) *vault.KDBXVault {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.kdbx")

	db := gokeepasslib.NewDatabase(
		gokeepasslib.WithDatabaseKDBXVersion4(),
	)
	db.Credentials = gokeepasslib.NewPasswordCredentials("testpass")

	rootGroup := gokeepasslib.NewGroup()
	rootGroup.Name = "Root"

	sshGroup := gokeepasslib.NewGroup()
	sshGroup.Name = "SSH Keys"

	entry := gokeepasslib.NewEntry()
	entry.Values = append(entry.Values, gokeepasslib.ValueData{
		Key:   "Title",
		Value: gokeepasslib.V{Content: keyName},
	})

	// Add binary attachment for the SSH key
	binary := db.AddBinary([]byte(keyData))
	entry.Binaries = append(entry.Binaries, binary.CreateReference(keyName))

	sshGroup.Entries = append(sshGroup.Entries, entry)
	rootGroup.Groups = append(rootGroup.Groups, sshGroup)
	db.Content.Root.Groups = []gokeepasslib.Group{rootGroup}

	if err := db.LockProtectedEntries(); err != nil {
		t.Fatalf("lock protected entries: %v", err)
	}

	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("create vault file: %v", err)
	}
	defer f.Close()

	if err := gokeepasslib.NewEncoder(f).Encode(db); err != nil {
		t.Fatalf("encode vault: %v", err)
	}

	v, err := vault.OpenKDBX(dbPath, "", "testpass")
	if err != nil {
		t.Fatalf("OpenKDBX: %v", err)
	}
	return v
}

// createTestVaultWithSecrets creates a test vault with secret entries (password fields).
func createTestVaultWithSecrets(t *testing.T, secrets map[string]string) *vault.KDBXVault {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.kdbx")

	db := gokeepasslib.NewDatabase(
		gokeepasslib.WithDatabaseKDBXVersion4(),
	)
	db.Credentials = gokeepasslib.NewPasswordCredentials("testpass")

	rootGroup := gokeepasslib.NewGroup()
	rootGroup.Name = "Root"

	for name, password := range secrets {
		entry := gokeepasslib.NewEntry()
		entry.Values = append(entry.Values,
			gokeepasslib.ValueData{
				Key:   "Title",
				Value: gokeepasslib.V{Content: name},
			},
			gokeepasslib.ValueData{
				Key:   "Password",
				Value: gokeepasslib.V{Content: password, Protected: w.NewBoolWrapper(true)},
			},
		)
		rootGroup.Entries = append(rootGroup.Entries, entry)
	}

	db.Content.Root.Groups = []gokeepasslib.Group{rootGroup}

	if err := db.LockProtectedEntries(); err != nil {
		t.Fatalf("lock protected entries: %v", err)
	}

	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("create vault file: %v", err)
	}
	defer f.Close()

	if err := gokeepasslib.NewEncoder(f).Encode(db); err != nil {
		t.Fatalf("encode vault: %v", err)
	}

	v, err := vault.OpenKDBX(dbPath, "", "testpass")
	if err != nil {
		t.Fatalf("OpenKDBX: %v", err)
	}
	return v
}
