package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot get home dir: %v", err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/.secrets/vault.key", filepath.Join(home, ".secrets/vault.key")},
		{"~", home},
		{"~/", home + "/"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
		{"~other", "~other"}, // not current user's home
	}

	for _, tt := range tests {
		got := ExpandTilde(tt.input)
		if got != tt.want {
			t.Errorf("ExpandTilde(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
