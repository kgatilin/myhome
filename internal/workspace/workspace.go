package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
)

// WorkspaceFile represents a .code-workspace JSON structure.
type WorkspaceFile struct {
	Folders  []Folder               `json:"folders"`
	Settings map[string]any         `json:"settings"`
}

// Folder is a workspace folder entry.
type Folder struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path"`
}

// knownDomains defines the domain prefixes to group repos by.
var knownDomains = []string{"work", "dev", "life"}

// GroupByDomain groups repos into domains based on their path prefix.
func GroupByDomain(repos []config.Repo) map[string][]config.Repo {
	groups := make(map[string][]config.Repo)
	for _, r := range repos {
		domain := domainOf(r.Path)
		groups[domain] = append(groups[domain], r)
	}
	// Sort repos within each domain by path.
	for k := range groups {
		sort.Slice(groups[k], func(i, j int) bool {
			return groups[k][i].Path < groups[k][j].Path
		})
	}
	return groups
}

// domainOf extracts the domain prefix from a repo path.
// Returns the first path component if it matches a known domain, otherwise "other".
func domainOf(path string) string {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		return "other"
	}
	prefix := parts[0]
	for _, d := range knownDomains {
		if prefix == d {
			return d
		}
	}
	return "other"
}

// defaultSettings returns shared VSCode settings for workspace files.
func defaultSettings() map[string]any {
	return map[string]any{
		"go.useLanguageServer":          true,
		"terminal.integrated.defaultProfile.osx":   "zsh",
		"terminal.integrated.defaultProfile.linux":  "zsh",
		"git.autofetch":                 true,
		"git.confirmSync":               false,
		"editor.formatOnSave":           true,
		"files.trimTrailingWhitespace":  true,
	}
}

// GenerateAll generates all workspace files and returns a map of filename to content.
func GenerateAll(cfg *config.Config) (map[string][]byte, error) {
	groups := GroupByDomain(cfg.Repos)
	files := make(map[string][]byte)

	// Root workspace with all repos
	root, err := generateWorkspace(cfg.Repos)
	if err != nil {
		return nil, fmt.Errorf("generate root workspace: %w", err)
	}
	files["home.code-workspace"] = root

	// Per-domain workspaces
	for _, domain := range knownDomains {
		repos, ok := groups[domain]
		if !ok || len(repos) == 0 {
			continue
		}
		ws, err := generateWorkspace(repos)
		if err != nil {
			return nil, fmt.Errorf("generate %s workspace: %w", domain, err)
		}
		files[domain+".code-workspace"] = ws
	}

	// "other" domain if any repos don't match known domains
	if others, ok := groups["other"]; ok && len(others) > 0 {
		ws, err := generateWorkspace(others)
		if err != nil {
			return nil, fmt.Errorf("generate other workspace: %w", err)
		}
		files["other.code-workspace"] = ws
	}

	return files, nil
}

// generateWorkspace creates a .code-workspace JSON for a set of repos.
func generateWorkspace(repos []config.Repo) ([]byte, error) {
	sorted := make([]config.Repo, len(repos))
	copy(sorted, repos)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	folders := make([]Folder, 0, len(sorted))
	for _, r := range sorted {
		folders = append(folders, Folder{
			Name: filepath.Base(r.Path),
			Path: r.Path,
		})
	}

	ws := WorkspaceFile{
		Folders:  folders,
		Settings: defaultSettings(),
	}

	data, err := json.MarshalIndent(ws, "", "\t")
	if err != nil {
		return nil, fmt.Errorf("marshal workspace: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// WriteAll generates and writes all workspace files to the home directory.
func WriteAll(cfg *config.Config, homeDir string) error {
	files, err := GenerateAll(cfg)
	if err != nil {
		return err
	}

	for name, content := range files {
		path := filepath.Join(homeDir, name)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}
