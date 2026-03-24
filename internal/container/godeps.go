package container

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GoDep represents a single Go dependency entry.
type GoDep struct {
	// Source dependencies use git clone + go build (source:github.com/user/repo cmd/tool).
	IsSource bool
	// For source deps: the repo URL (e.g. github.com/kgatilin/archlint).
	RepoURL string
	// For source deps: the cmd path within the repo (e.g. cmd/archlint).
	CmdPath string
	// For regular deps: the full go install target (e.g. golang.org/x/tools/cmd/goimports@latest).
	InstallTarget string
}

// ParseGoDepsFile reads a dependencies_go.txt file and returns parsed entries.
// Format:
//   - source:github.com/user/repo cmd/tool  → git clone + go build
//   - github.com/user/tool@latest           → go install
//   - empty lines and # comments are skipped
func ParseGoDepsFile(path string) ([]GoDep, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var deps []GoDep
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		dep, err := parseGoDep(line)
		if err != nil {
			return nil, fmt.Errorf("parsing %q: %w", line, err)
		}
		deps = append(deps, dep)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return deps, nil
}

func parseGoDep(line string) (GoDep, error) {
	if rest, ok := strings.CutPrefix(line, "source:"); ok {
		parts := strings.SplitN(rest, " ", 2)
		if len(parts) != 2 {
			return GoDep{}, fmt.Errorf("source entry must have format 'source:REPO CMD_PATH', got %q", line)
		}
		return GoDep{
			IsSource: true,
			RepoURL:  strings.TrimSpace(parts[0]),
			CmdPath:  strings.TrimSpace(parts[1]),
		}, nil
	}
	return GoDep{InstallTarget: line}, nil
}

// BinaryName returns the expected binary name for a dependency.
func (d GoDep) BinaryName() string {
	if d.IsSource {
		return filepath.Base(d.CmdPath)
	}
	// For go install targets like "github.com/foo/bar/cmd/baz@latest",
	// strip @version and take basename.
	target := d.InstallTarget
	if idx := strings.LastIndex(target, "@"); idx > 0 {
		target = target[:idx]
	}
	return filepath.Base(target)
}

// GenerateDockerfile creates a Dockerfile that extends baseImage and installs Go deps.
// Each source dep is cloned and built; regular deps use go install.
// Returns empty string if there are no deps.
func GenerateDockerfile(baseImage string, deps []GoDep) string {
	if len(deps) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "FROM %s\n", baseImage)

	for _, dep := range deps {
		if dep.IsSource {
			// Clone repo, build binary, clean up — all in one RUN to minimize layers.
			binName := dep.BinaryName()
			fmt.Fprintf(&b, "RUN git clone --depth 1 https://%s /tmp/%s-build && "+
				"cd /tmp/%s-build && "+
				"go build -o \"$(go env GOPATH)/bin/%s\" ./%s && "+
				"rm -rf /tmp/%s-build\n",
				dep.RepoURL, binName, binName, binName, dep.CmdPath, binName)
		} else {
			fmt.Fprintf(&b, "RUN go install %s\n", dep.InstallTarget)
		}
	}

	return b.String()
}
