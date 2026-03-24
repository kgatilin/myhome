package container

import (
	"slices"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

func TestDetectRuntime_Preferred(t *testing.T) {
	_, err := DetectRuntime("nonexistent-runtime-xyz")
	if err == nil {
		t.Error("expected error for nonexistent preferred runtime")
	}
}

func TestDetectRuntime_Auto(t *testing.T) {
	result, err := DetectRuntime("auto")
	if err != nil {
		if result != "" {
			t.Errorf("expected empty result on error, got %q", result)
		}
		return
	}
	if result == "" {
		t.Error("expected non-empty runtime path")
	}
}

func TestResolveMounts(t *testing.T) {
	homeDir := "/home/testuser"

	tests := []struct {
		name   string
		mounts []string
		want   []string
	}{
		{
			name:   "tilde with read-only",
			mounts: []string{"~/.ssh:ro"},
			want:   []string{"/home/testuser/.ssh:/home/testuser/.ssh:ro"},
		},
		{
			name:   "tilde without suffix",
			mounts: []string{"~/.gitconfig"},
			want:   []string{"/home/testuser/.gitconfig:/home/testuser/.gitconfig"},
		},
		{
			name:   "absolute path",
			mounts: []string{"/tmp/data"},
			want:   []string{"/tmp/data:/tmp/data"},
		},
		{
			name:   "multiple mounts",
			mounts: []string{"~/.ssh:ro", "~/.gitconfig:ro", "~/.cursor"},
			want: []string{
				"/home/testuser/.ssh:/home/testuser/.ssh:ro",
				"/home/testuser/.gitconfig:/home/testuser/.gitconfig:ro",
				"/home/testuser/.cursor:/home/testuser/.cursor",
			},
		},
		{
			name:   "empty mounts",
			mounts: nil,
			want:   nil,
		},
		{
			name:   "bare tilde",
			mounts: []string{"~"},
			want:   []string{"/home/testuser:/home/testuser"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveMounts(tt.mounts, homeDir)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d mounts, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("mount[%d]: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestResolveAuth(t *testing.T) {
	homeDir := "/home/testuser"

	tests := []struct {
		name            string
		authFile        string
		authEnv         map[string]string
		claudeConfigDir string
		wantMounts      int
		wantEnvs        int
		wantMountSub    string
	}{
		{
			name:            "simple auth file",
			authFile:        "~/.claude.json",
			claudeConfigDir: "~/.claude",
			wantMounts:      2,
			wantEnvs:        0,
			wantMountSub:    "/home/testuser/.claude.json",
		},
		{
			name:     "auth with env vars",
			authFile: "~/.claude-vertex.json",
			authEnv: map[string]string{
				"CLAUDE_CODE_USE_VERTEX":      "1",
				"ANTHROPIC_VERTEX_PROJECT_ID": "my-project",
			},
			claudeConfigDir: "~/.claude",
			wantMounts:      2,
			wantEnvs:        2,
			wantMountSub:    "/home/testuser/.claude-vertex.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounts, envVars := ResolveAuth(tt.authFile, tt.authEnv, tt.claudeConfigDir, homeDir)
			if len(mounts) != tt.wantMounts {
				t.Errorf("mounts: got %d, want %d: %v", len(mounts), tt.wantMounts, mounts)
			}
			if len(envVars) != tt.wantEnvs {
				t.Errorf("envVars: got %d, want %d: %v", len(envVars), tt.wantEnvs, envVars)
			}

			foundSub := false
			for _, m := range mounts {
				if contains(m, tt.wantMountSub) {
					foundSub = true
					break
				}
			}
			if !foundSub {
				t.Errorf("expected mount containing %q, got %v", tt.wantMountSub, mounts)
			}
		})
	}
}

func TestResolveAuth_ConfigDirMount(t *testing.T) {
	homeDir := "/home/testuser"
	mounts, _ := ResolveAuth("~/.claude.json", nil, "~/.claude", homeDir)

	found := false
	for _, m := range mounts {
		if m == "/home/testuser/.claude:/home/testuser/.claude" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected config dir mount, got %v", mounts)
	}
}

func TestBuildArgs(t *testing.T) {
	homeDir := "/home/testuser"
	ctr := config.Container{
		Dockerfile: "containers/claude-code/official",
		Image:      "claude-code-local:official",
	}

	args := BuildArgs("claude-code", ctr, homeDir)

	if args[0] != "build" {
		t.Errorf("expected 'build', got %q", args[0])
	}
	if !slices.Contains(args, "-t") {
		t.Error("expected -t flag")
	}
	if !slices.Contains(args, "claude-code-local:official") {
		t.Errorf("expected image tag in args: %v", args)
	}
	fIdx := slices.Index(args, "-f")
	if fIdx < 0 || fIdx+1 >= len(args) {
		t.Fatal("expected -f flag with value")
	}
	dockerfilePath := args[fIdx+1]
	if dockerfilePath != "/home/testuser/containers/claude-code/official" {
		t.Errorf("dockerfile path: got %q, want %q", dockerfilePath, "/home/testuser/containers/claude-code/official")
	}
}

func TestRunArgs(t *testing.T) {
	homeDir := "/home/testuser"

	tests := []struct {
		name       string
		ctr        config.Container
		opts       RunOpts
		wantArgs   []string
		noWantArgs []string
	}{
		{
			name:     "basic interactive",
			ctr:      config.Container{Image: "test:latest"},
			opts:     RunOpts{},
			wantArgs: []string{"run", "--name", "-it", "--rm", "test:latest"},
		},
		{
			name:       "detached mode",
			ctr:        config.Container{Image: "test:latest"},
			opts:       RunOpts{Detach: true},
			wantArgs:   []string{"run", "-d"},
			noWantArgs: []string{"-it", "--rm"},
		},
		{
			name:     "firewall enabled",
			ctr:      config.Container{Image: "test:latest", Firewall: true},
			opts:     RunOpts{},
			wantArgs: []string{"--network", "none"},
		},
		{
			name:     "with mounts",
			ctr:      config.Container{Image: "test:latest", Mounts: []string{"~/.ssh:ro"}},
			opts:     RunOpts{},
			wantArgs: []string{"-v", "/home/testuser/.ssh:/home/testuser/.ssh:ro"},
		},
		{
			name:     "with project dir",
			ctr:      config.Container{Image: "test:latest"},
			opts:     RunOpts{ProjectDir: "/home/testuser/work/myproject"},
			wantArgs: []string{"-v", "/home/testuser/work/myproject:/home/testuser/work/myproject", "-w", "/home/testuser/work/myproject"},
		},
		{
			name:     "with startup commands",
			ctr:      config.Container{Image: "test:latest", StartupCommands: []string{"pip install -r requirements.txt", "echo ready"}},
			opts:     RunOpts{},
			wantArgs: []string{"/bin/sh", "-c", "pip install -r requirements.txt && echo ready"},
		},
		{
			name: "with auth profile",
			ctr:  config.Container{Image: "test:latest"},
			opts: RunOpts{
				AuthFile:        "~/.claude.json",
				AuthEnv:         map[string]string{"MY_VAR": "value"},
				ClaudeConfigDir: "~/.claude",
			},
			wantArgs: []string{"-e", "MY_VAR=value"},
		},
		{
			name:     "with extra args",
			ctr:      config.Container{Image: "test:latest"},
			opts:     RunOpts{ExtraArgs: []string{"--cpus", "2"}},
			wantArgs: []string{"--cpus", "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := RunArgs(tt.name, tt.ctr, homeDir, tt.opts)

			for _, want := range tt.wantArgs {
				if !slices.Contains(args, want) {
					t.Errorf("expected %q in args: %v", want, args)
				}
			}
			for _, noWant := range tt.noWantArgs {
				if slices.Contains(args, noWant) {
					t.Errorf("unexpected %q in args: %v", noWant, args)
				}
			}
		})
	}
}

func TestExpandTilde(t *testing.T) {
	homeDir := "/home/user"

	tests := []struct {
		input string
		want  string
	}{
		{"~/.ssh", "/home/user/.ssh"},
		{"~", "/home/user"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := expandTilde(tt.input, homeDir)
			if got != tt.want {
				t.Errorf("expandTilde(%q): got %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
