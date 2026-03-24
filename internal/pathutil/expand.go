package pathutil

import (
	"os"
	"strings"
)

// ExpandTilde replaces a leading ~ with the current user's home directory.
// Returns the path unchanged if it doesn't start with ~ or if home dir lookup fails.
func ExpandTilde(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return home + path[1:]
		}
	}
	return path
}
