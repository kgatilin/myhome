package vault

import (
	"os"
	"path/filepath"
	"testing"

	gokeepasslib "github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

// createTestVault creates a .kdbx file in a temp dir for testing.
// Returns the path to the vault file.
func createTestVault(t *testing.T, password string, entries []testEntry) string {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.kdbx")

	db := gokeepasslib.NewDatabase(
		gokeepasslib.WithDatabaseKDBXVersion4(),
	)
	db.Credentials = gokeepasslib.NewPasswordCredentials(password)

	// Build root group with entries
	rootGroup := gokeepasslib.NewGroup()
	rootGroup.Name = "Root"

	for _, e := range entries {
		if e.group == "" {
			rootGroup.Entries = append(rootGroup.Entries, makeEntry(e.title, e.password, e.fields))
		}
	}

	// Build sub-groups
	groups := map[string]*gokeepasslib.Group{}
	for _, e := range entries {
		if e.group != "" {
			g, ok := groups[e.group]
			if !ok {
				ng := gokeepasslib.NewGroup()
				ng.Name = e.group
				g = &ng
				groups[e.group] = g
			}
			g.Entries = append(g.Entries, makeEntry(e.title, e.password, e.fields))
		}
	}
	for _, g := range groups {
		rootGroup.Groups = append(rootGroup.Groups, *g)
	}

	db.Content.Root.Groups = []gokeepasslib.Group{rootGroup}

	// Lock entries before encoding
	if err := db.LockProtectedEntries(); err != nil {
		t.Fatalf("lock protected entries: %v", err)
	}

	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("create vault file: %v", err)
	}
	defer f.Close()

	enc := gokeepasslib.NewEncoder(f)
	if err := enc.Encode(db); err != nil {
		t.Fatalf("encode vault: %v", err)
	}

	return dbPath
}

type testEntry struct {
	group    string
	title    string
	password string
	fields   map[string]string
}

func makeEntry(title, password string, fields map[string]string) gokeepasslib.Entry {
	entry := gokeepasslib.NewEntry()
	entry.Values = append(entry.Values, gokeepasslib.ValueData{
		Key:   "Title",
		Value: gokeepasslib.V{Content: title},
	})
	entry.Values = append(entry.Values, gokeepasslib.ValueData{
		Key:   "Password",
		Value: gokeepasslib.V{Content: password, Protected: w.NewBoolWrapper(true)},
	})
	for k, v := range fields {
		entry.Values = append(entry.Values, gokeepasslib.ValueData{
			Key:   k,
			Value: gokeepasslib.V{Content: v},
		})
	}
	return entry
}

func TestOpenKDBX(t *testing.T) {
	dbPath := createTestVault(t, "testpass", []testEntry{
		{title: "my-token", password: "secret123"},
	})

	v, err := OpenKDBX(dbPath, "", "testpass")
	if err != nil {
		t.Fatalf("OpenKDBX() error: %v", err)
	}

	got, err := v.Get("my-token")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != "secret123" {
		t.Errorf("Get() = %q, want %q", got, "secret123")
	}
}

func TestOpenKDBXWrongPassword(t *testing.T) {
	dbPath := createTestVault(t, "testpass", []testEntry{
		{title: "token", password: "secret"},
	})

	_, err := OpenKDBX(dbPath, "", "wrongpass")
	if err == nil {
		t.Fatal("OpenKDBX() should fail with wrong password")
	}
}

func TestGetByPath(t *testing.T) {
	dbPath := createTestVault(t, "testpass", []testEntry{
		{group: "SSH Keys", title: "work-key", password: "ssh-secret"},
	})

	v, err := OpenKDBX(dbPath, "", "testpass")
	if err != nil {
		t.Fatalf("OpenKDBX() error: %v", err)
	}

	// Should find by path
	got, err := v.Get("SSH Keys/work-key")
	if err != nil {
		t.Fatalf("Get(path) error: %v", err)
	}
	if got != "ssh-secret" {
		t.Errorf("Get(path) = %q, want %q", got, "ssh-secret")
	}

	// Should also find by plain title
	got, err = v.Get("work-key")
	if err != nil {
		t.Fatalf("Get(title) error: %v", err)
	}
	if got != "ssh-secret" {
		t.Errorf("Get(title) = %q, want %q", got, "ssh-secret")
	}
}

func TestGetField(t *testing.T) {
	dbPath := createTestVault(t, "testpass", []testEntry{
		{title: "github", password: "ghp_token", fields: map[string]string{"UserName": "myuser"}},
	})

	v, err := OpenKDBX(dbPath, "", "testpass")
	if err != nil {
		t.Fatalf("OpenKDBX() error: %v", err)
	}

	got, err := v.GetField("github", "UserName")
	if err != nil {
		t.Fatalf("GetField() error: %v", err)
	}
	if got != "myuser" {
		t.Errorf("GetField() = %q, want %q", got, "myuser")
	}
}

func TestGetNotFound(t *testing.T) {
	dbPath := createTestVault(t, "testpass", []testEntry{
		{title: "exists", password: "val"},
	})

	v, err := OpenKDBX(dbPath, "", "testpass")
	if err != nil {
		t.Fatalf("OpenKDBX() error: %v", err)
	}

	_, err = v.Get("does-not-exist")
	if err == nil {
		t.Fatal("Get() should fail for non-existent entry")
	}
}

func TestList(t *testing.T) {
	dbPath := createTestVault(t, "testpass", []testEntry{
		{title: "token-a", password: "a"},
		{group: "SSH Keys", title: "key-b", password: "b"},
	})

	v, err := OpenKDBX(dbPath, "", "testpass")
	if err != nil {
		t.Fatalf("OpenKDBX() error: %v", err)
	}

	titles := v.List()
	if len(titles) < 2 {
		t.Fatalf("List() returned %d entries, want at least 2", len(titles))
	}

	found := map[string]bool{}
	for _, title := range titles {
		found[title] = true
	}
	if !found["Root/token-a"] {
		t.Error("List() missing Root/token-a")
	}
	if !found["Root/SSH Keys/key-b"] {
		t.Error("List() missing Root/SSH Keys/key-b")
	}
}

func TestGetAttachment(t *testing.T) {
	dbPath := createTestVaultWithAttachment(t, "testpass",
		"SSH Keys", "personal", "ssh-ed25519-key-data-here")

	v, err := OpenKDBX(dbPath, "", "testpass")
	if err != nil {
		t.Fatalf("OpenKDBX() error: %v", err)
	}

	got, err := v.GetAttachment("SSH Keys/personal", "personal")
	if err != nil {
		t.Fatalf("GetAttachment() error: %v", err)
	}
	if string(got) != "ssh-ed25519-key-data-here" {
		t.Errorf("GetAttachment() = %q, want %q", string(got), "ssh-ed25519-key-data-here")
	}
}

func TestGetAttachmentNotFound(t *testing.T) {
	dbPath := createTestVault(t, "testpass", []testEntry{
		{group: "SSH Keys", title: "mykey", password: ""},
	})

	v, err := OpenKDBX(dbPath, "", "testpass")
	if err != nil {
		t.Fatalf("OpenKDBX() error: %v", err)
	}

	_, err = v.GetAttachment("SSH Keys/mykey", "mykey")
	if err == nil {
		t.Fatal("GetAttachment() should fail when no attachment exists")
	}
}

// createTestVaultWithAttachment creates a vault with an entry that has a binary attachment.
func createTestVaultWithAttachment(t *testing.T, password, group, name, content string) string {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.kdbx")

	db := gokeepasslib.NewDatabase(
		gokeepasslib.WithDatabaseKDBXVersion4(),
	)
	db.Credentials = gokeepasslib.NewPasswordCredentials(password)

	rootGroup := gokeepasslib.NewGroup()
	rootGroup.Name = "Root"

	subGroup := gokeepasslib.NewGroup()
	subGroup.Name = group

	entry := gokeepasslib.NewEntry()
	entry.Values = append(entry.Values, gokeepasslib.ValueData{
		Key:   "Title",
		Value: gokeepasslib.V{Content: name},
	})

	// Add binary attachment
	binary := db.AddBinary([]byte(content))
	entry.Binaries = append(entry.Binaries, binary.CreateReference(name))

	subGroup.Entries = append(subGroup.Entries, entry)
	rootGroup.Groups = append(rootGroup.Groups, subGroup)
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

	return dbPath
}

func TestResolveVaultRef(t *testing.T) {
	dbPath := createTestVault(t, "testpass", []testEntry{
		{title: "my-pat", password: "ghp_abc123"},
	})

	v, err := OpenKDBX(dbPath, "", "testpass")
	if err != nil {
		t.Fatalf("OpenKDBX() error: %v", err)
	}

	tests := []struct {
		ref  string
		want string
	}{
		{"vault://my-pat", "ghp_abc123"},
		{"plain-value", "plain-value"},
	}

	for _, tt := range tests {
		got, err := ResolveVaultRef(tt.ref, v)
		if err != nil {
			t.Fatalf("ResolveVaultRef(%q) error: %v", tt.ref, err)
		}
		if got != tt.want {
			t.Errorf("ResolveVaultRef(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestResolveVaultRefNoVault(t *testing.T) {
	// Non-vault ref should pass through even with nil vault.
	got, err := ResolveVaultRef("plain-value", nil)
	if err != nil {
		t.Fatalf("ResolveVaultRef(plain) error: %v", err)
	}
	if got != "plain-value" {
		t.Errorf("ResolveVaultRef(plain) = %q, want %q", got, "plain-value")
	}

	// vault:// ref with nil vault should error.
	_, err = ResolveVaultRef("vault://something", nil)
	if err == nil {
		t.Fatal("ResolveVaultRef(vault://, nil) should fail")
	}
}

func TestOpenKDBXWithKeyFile(t *testing.T) {
	// Create a vault with password only, then try with a non-existent key file
	dbPath := createTestVault(t, "testpass", []testEntry{
		{title: "test", password: "val"},
	})

	_, err := OpenKDBX(dbPath, "/nonexistent/key.file", "testpass")
	if err == nil {
		t.Fatal("OpenKDBX() should fail with non-existent key file")
	}
}
