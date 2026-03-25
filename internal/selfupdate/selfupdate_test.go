package selfupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindSourceDir(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, home string)
		repoPaths []string
		wantSuf   string
		wantErr   bool
	}{
		{
			name: "well-known path",
			setup: func(t *testing.T, home string) {
				t.Helper()
				os.MkdirAll(filepath.Join(home, "dev", "tools", "myhome", ".git"), 0o755)
			},
			wantSuf: "dev/tools/myhome",
		},
		{
			name: "found via repo paths",
			setup: func(t *testing.T, home string) {
				t.Helper()
				os.MkdirAll(filepath.Join(home, "src", "myhome", ".git"), 0o755)
			},
			repoPaths: []string{"src/myhome"},
			wantSuf:   "src/myhome",
		},
		{
			name:    "not found",
			setup:   func(t *testing.T, home string) {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			tt.setup(t, home)

			dir, err := FindSourceDir(home, tt.repoPaths)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(dir, tt.wantSuf) {
				t.Errorf("got %s, want suffix %s", dir, tt.wantSuf)
			}
		})
	}
}

func TestEnvWithMiseShims(t *testing.T) {
	env := envWithMiseShims()
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			if !strings.Contains(e, "mise/shims") {
				t.Errorf("PATH does not contain mise/shims: %s", e)
			}
			return
		}
	}
	t.Error("no PATH entry found in env")
}
