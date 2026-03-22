package archive

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/gitignore"
)

// Move moves a path to ~/archive/ and regenerates .gitignore.
func Move(path, homeDir string, cfg *config.Config) error {
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(homeDir, path)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	archiveDir := filepath.Join(homeDir, "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}

	dest := filepath.Join(archiveDir, filepath.Base(absPath))
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("destination already exists: %s", dest)
	}

	if err := os.Rename(absPath, dest); err != nil {
		return fmt.Errorf("move %s to %s: %w", absPath, dest, err)
	}

	// Regenerate .gitignore (archive/ is already in static rules)
	if err := gitignore.Write(cfg, homeDir); err != nil {
		return fmt.Errorf("regenerate .gitignore: %w", err)
	}

	fmt.Printf("Archived %s → %s\n", path, dest)
	return nil
}
