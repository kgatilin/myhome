package identity

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	blockStart = "# myhome-managed-start"
	blockEnd   = "# myhome-managed-end"
)

// Identity maps a directory pattern to a git user/email.
type Identity struct {
	Dir   string // directory pattern (e.g., "~/work/")
	Name  string
	Email string
}

// GenerateManagedBlock produces the myhome-managed block content (without delimiters).
func GenerateManagedBlock(defaultName, defaultEmail string, overrides []Identity) string {
	var b strings.Builder
	b.WriteString("[user]\n")
	fmt.Fprintf(&b, "  name = %s\n", defaultName)
	fmt.Fprintf(&b, "  email = %s\n", defaultEmail)

	for _, id := range overrides {
		b.WriteString("\n")
		dir := id.Dir
		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
		fmt.Fprintf(&b, "[includeIf \"gitdir:%s\"]\n", dir)
		fmt.Fprintf(&b, "  path = %s\n", gitconfigPath(id.Dir))
	}

	return b.String()
}

// GenerateGitconfig produces .gitconfig content with includeIf directives
// wrapped in myhome-managed delimiters.
func GenerateGitconfig(defaultName, defaultEmail string, overrides []Identity) string {
	var b strings.Builder
	b.WriteString(blockStart)
	b.WriteString("\n")
	b.WriteString(GenerateManagedBlock(defaultName, defaultEmail, overrides))
	b.WriteString(blockEnd)
	b.WriteString("\n")
	return b.String()
}

// GenerateIdentityFile produces a git config fragment for a single identity override.
func GenerateIdentityFile(id Identity) string {
	var b strings.Builder
	b.WriteString("[user]\n")
	fmt.Fprintf(&b, "  name = %s\n", id.Name)
	fmt.Fprintf(&b, "  email = %s\n", id.Email)
	return b.String()
}

// WriteGitconfig writes identity override files and updates only the myhome-managed
// block in .gitconfig, preserving all other content.
func WriteGitconfig(homeDir, defaultName, defaultEmail string, overrides []Identity) error {
	// Write identity override files first
	for _, id := range overrides {
		content := GenerateIdentityFile(id)
		path := filepath.Join(homeDir, gitconfigPath(id.Dir))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write identity file %s: %w", path, err)
		}
	}

	// Generate the managed block
	managedBlock := GenerateGitconfig(defaultName, defaultEmail, overrides)

	// Read existing .gitconfig if it exists
	path := filepath.Join(homeDir, ".gitconfig")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read .gitconfig: %w", err)
	}

	var result string
	if len(existing) == 0 {
		// No existing file — write just the managed block
		result = managedBlock
	} else {
		result = replaceBlock(string(existing), managedBlock)
	}

	if err := os.WriteFile(path, []byte(result), 0o644); err != nil {
		return fmt.Errorf("write .gitconfig: %w", err)
	}
	return nil
}

// replaceBlock replaces the myhome-managed block in content, or appends it if not found.
func replaceBlock(content, block string) string {
	startIdx := strings.Index(content, blockStart)
	endIdx := strings.Index(content, blockEnd)

	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		// No existing block — append
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if len(content) > 0 && !strings.HasSuffix(content, "\n\n") {
			content += "\n"
		}
		return content + block
	}

	// Replace existing block (include the trailing newline after blockEnd)
	afterEnd := endIdx + len(blockEnd)
	if afterEnd < len(content) && content[afterEnd] == '\n' {
		afterEnd++
	}

	return content[:startIdx] + block + content[afterEnd:]
}

// gitconfigPath returns the path for an identity override config file.
// e.g., "~/work/" → ".gitconfig-work"
func gitconfigPath(dir string) string {
	// Strip ~/ prefix and trailing /
	name := strings.TrimPrefix(dir, "~/")
	name = strings.TrimSuffix(name, "/")
	name = strings.ReplaceAll(name, "/", "-")
	return fmt.Sprintf(".gitconfig-%s", name)
}
