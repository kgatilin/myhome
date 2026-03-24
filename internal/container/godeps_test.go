package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoDepsFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []GoDep
		wantErr bool
	}{
		{
			name:    "source entry",
			content: "source:github.com/kgatilin/archlint cmd/archlint\n",
			want: []GoDep{
				{IsSource: true, RepoURL: "github.com/kgatilin/archlint", CmdPath: "cmd/archlint"},
			},
		},
		{
			name:    "regular entry",
			content: "golang.org/x/tools/cmd/goimports@latest\n",
			want: []GoDep{
				{InstallTarget: "golang.org/x/tools/cmd/goimports@latest"},
			},
		},
		{
			name:    "mixed entries with comments and blanks",
			content: "# tools\nsource:github.com/kgatilin/archlint cmd/archlint\n\ngolang.org/x/tools/cmd/goimports@latest\n# end\n",
			want: []GoDep{
				{IsSource: true, RepoURL: "github.com/kgatilin/archlint", CmdPath: "cmd/archlint"},
				{InstallTarget: "golang.org/x/tools/cmd/goimports@latest"},
			},
		},
		{
			name:    "empty file",
			content: "\n\n",
			want:    nil,
		},
		{
			name:    "source entry missing cmd path",
			content: "source:github.com/kgatilin/archlint\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "dependencies_go.txt")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got, err := ParseGoDepsFile(path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d deps, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("dep[%d]: got %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseGoDepsFile_NotFound(t *testing.T) {
	_, err := ParseGoDepsFile("/nonexistent/file")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestGoDep_BinaryName(t *testing.T) {
	tests := []struct {
		dep  GoDep
		want string
	}{
		{
			dep:  GoDep{IsSource: true, RepoURL: "github.com/kgatilin/archlint", CmdPath: "cmd/archlint"},
			want: "archlint",
		},
		{
			dep:  GoDep{InstallTarget: "golang.org/x/tools/cmd/goimports@latest"},
			want: "goimports",
		},
		{
			dep:  GoDep{InstallTarget: "github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.0"},
			want: "golangci-lint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.dep.BinaryName()
			if got != tt.want {
				t.Errorf("BinaryName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateDockerfile(t *testing.T) {
	tests := []struct {
		name      string
		baseImage string
		deps      []GoDep
		wantParts []string // substrings that must appear
		wantEmpty bool
	}{
		{
			name:      "empty deps",
			baseImage: "test:latest",
			deps:      nil,
			wantEmpty: true,
		},
		{
			name:      "source dep",
			baseImage: "claude-code:official",
			deps: []GoDep{
				{IsSource: true, RepoURL: "github.com/kgatilin/archlint", CmdPath: "cmd/archlint"},
			},
			wantParts: []string{
				"FROM claude-code:official",
				"git clone --depth 1 https://github.com/kgatilin/archlint",
				"go build",
				"./cmd/archlint",
				"rm -rf",
			},
		},
		{
			name:      "regular dep",
			baseImage: "test:latest",
			deps: []GoDep{
				{InstallTarget: "golang.org/x/tools/cmd/goimports@latest"},
			},
			wantParts: []string{
				"FROM test:latest",
				"go install golang.org/x/tools/cmd/goimports@latest",
			},
		},
		{
			name:      "mixed deps",
			baseImage: "test:latest",
			deps: []GoDep{
				{IsSource: true, RepoURL: "github.com/kgatilin/archlint", CmdPath: "cmd/archlint"},
				{InstallTarget: "golang.org/x/tools/cmd/goimports@latest"},
			},
			wantParts: []string{
				"FROM test:latest",
				"git clone",
				"go install",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateDockerfile(tt.baseImage, tt.deps)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("expected empty, got %q", got)
				}
				return
			}
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("expected %q in output:\n%s", part, got)
				}
			}
		})
	}
}
