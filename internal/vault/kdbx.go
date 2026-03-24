package vault

import (
	"fmt"
	"os"
	"strings"

	gokeepasslib "github.com/tobischo/gokeepasslib/v3"
)

// KDBXVault provides native read access to a .kdbx vault file using gokeepasslib.
// This eliminates the need for keepassxc-cli for reading secrets.
type KDBXVault struct {
	db *gokeepasslib.Database
}

// OpenKDBX opens and decrypts a .kdbx vault file using a master password and key file.
// The returned KDBXVault holds the decrypted database in memory.
func OpenKDBX(dbPath, keyFile, password string) (*KDBXVault, error) {
	f, err := os.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open vault: %w", err)
	}
	defer f.Close()

	var creds *gokeepasslib.DBCredentials
	if keyFile != "" {
		creds, err = gokeepasslib.NewPasswordAndKeyCredentials(password, keyFile)
		if err != nil {
			return nil, fmt.Errorf("build credentials: %w", err)
		}
	} else {
		creds = gokeepasslib.NewPasswordCredentials(password)
	}

	db := gokeepasslib.NewDatabase()
	db.Credentials = creds

	if err := gokeepasslib.NewDecoder(f).Decode(db); err != nil {
		return nil, fmt.Errorf("decode vault: %w", err)
	}

	if err := db.UnlockProtectedEntries(); err != nil {
		return nil, fmt.Errorf("unlock protected entries: %w", err)
	}

	return &KDBXVault{db: db}, nil
}

// Get returns the password field of an entry found by title.
// It searches all groups recursively. The name can be a plain title
// (e.g. "gh-readonly-pat") or a path with slashes (e.g. "SSH Keys/work-key").
func (v *KDBXVault) Get(name string) (string, error) {
	entry := v.findEntry(name)
	if entry == nil {
		return "", fmt.Errorf("entry %q not found in vault", name)
	}
	return entry.GetPassword(), nil
}

// GetField returns an arbitrary field of an entry found by title.
// Common field keys: "Title", "Password", "UserName", "URL", "Notes".
func (v *KDBXVault) GetField(entryName, fieldKey string) (string, error) {
	entry := v.findEntry(entryName)
	if entry == nil {
		return "", fmt.Errorf("entry %q not found in vault", entryName)
	}
	return entry.GetContent(fieldKey), nil
}

// List returns the titles of all entries in the vault.
func (v *KDBXVault) List() []string {
	var titles []string
	for _, group := range v.db.Content.Root.Groups {
		titles = append(titles, listGroupEntries("", &group)...)
	}
	return titles
}

// findEntry searches for an entry by title or path (group/title).
// It tries multiple matching strategies:
//  1. Plain title match (e.g. "work-key")
//  2. Full path match (e.g. "Root/SSH Keys/work-key")
//  3. Path without root group (e.g. "SSH Keys/work-key" when root is "Root")
func (v *KDBXVault) findEntry(name string) *gokeepasslib.Entry {
	for i := range v.db.Content.Root.Groups {
		root := &v.db.Content.Root.Groups[i]
		if entry := searchGroup("", root, name); entry != nil {
			return entry
		}
		// Try again with root group name stripped from the search name
		if root.Name != "" {
			if entry := searchGroup("", root, root.Name+"/"+name); entry != nil {
				return entry
			}
		}
	}
	return nil
}

// searchGroup recursively searches groups for an entry matching name.
// Name can be a plain title or a slash-separated path like "Root/SSH Keys/work-key".
func searchGroup(prefix string, group *gokeepasslib.Group, name string) *gokeepasslib.Entry {
	groupPath := prefix
	if group.Name != "" {
		if groupPath != "" {
			groupPath += "/"
		}
		groupPath += group.Name
	}

	for i := range group.Entries {
		title := group.Entries[i].GetTitle()
		// Match by plain title
		if title == name {
			return &group.Entries[i]
		}
		// Match by full path (e.g. "Root/SSH Keys/work-key")
		fullPath := groupPath + "/" + title
		if fullPath == name {
			return &group.Entries[i]
		}
	}

	for i := range group.Groups {
		if entry := searchGroup(groupPath, &group.Groups[i], name); entry != nil {
			return entry
		}
	}
	return nil
}

// listGroupEntries recursively collects entry titles with their group paths.
func listGroupEntries(prefix string, group *gokeepasslib.Group) []string {
	var titles []string
	groupPath := prefix
	if group.Name != "" {
		if groupPath != "" {
			groupPath += "/"
		}
		groupPath += group.Name
	}

	for _, entry := range group.Entries {
		title := entry.GetTitle()
		if groupPath != "" {
			titles = append(titles, groupPath+"/"+title)
		} else {
			titles = append(titles, title)
		}
	}

	for i := range group.Groups {
		titles = append(titles, listGroupEntries(groupPath, &group.Groups[i])...)
	}
	return titles
}

// GetAttachment returns the binary content of an attachment on a vault entry.
// SSH keys imported via "vault ssh-add" are stored as attachments under "SSH Keys/<name>".
func (v *KDBXVault) GetAttachment(entryName, attachmentName string) ([]byte, error) {
	entry := v.findEntry(entryName)
	if entry == nil {
		return nil, fmt.Errorf("entry %q not found in vault", entryName)
	}
	for _, binRef := range entry.Binaries {
		if binRef.Name == attachmentName {
			bin := v.db.FindBinary(binRef.Value.ID)
			if bin == nil {
				return nil, fmt.Errorf("binary ref %d for attachment %q not found in vault", binRef.Value.ID, attachmentName)
			}
			return bin.GetContentBytes()
		}
	}
	return nil, fmt.Errorf("attachment %q not found on entry %q", attachmentName, entryName)
}

// ResolveVaultRef resolves a "vault://<entry>" reference to its secret value.
// Returns the original string unchanged if it doesn't have the vault:// prefix.
func ResolveVaultRef(ref string, v *KDBXVault) (string, error) {
	if !strings.HasPrefix(ref, "vault://") {
		return ref, nil
	}
	if v == nil {
		return "", fmt.Errorf("vault:// reference %q but no vault is open", ref)
	}
	entryName := strings.TrimPrefix(ref, "vault://")
	return v.Get(entryName)
}
