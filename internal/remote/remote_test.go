package remote

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

func fakeExec(calls *[]string) ExecFunc {
	return func(name string, args ...string) *exec.Cmd {
		call := name
		for _, a := range args {
			call += " " + a
		}
		*calls = append(*calls, call)
		// Return a command that outputs a fake tmux session list.
		return exec.Command("echo", "test-session|2|Mon Mar 22 10:00:00 2026")
	}
}

func fakeExecEmpty() ExecFunc {
	return func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "")
	}
}

func fakeExecFail() ExecFunc {
	return func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
}

func testRemote() config.Remote {
	return config.Remote{
		Host: "user@vps.example.com",
		Home: "~/",
		Env:  "work",
	}
}

func TestRun(t *testing.T) {
	var calls []string
	session, err := Run(testRemote(), "uagent", "Fix the bug", "work", fakeExec(&calls))
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if session == "" {
		t.Error("Run() returned empty session name")
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0] == "" {
		t.Error("expected non-empty call")
	}
}

func TestRunError(t *testing.T) {
	_, err := Run(testRemote(), "uagent", "Fix bug", "", fakeExecFail())
	if err == nil {
		t.Error("Run() should fail when ssh fails")
	}
}

func TestList(t *testing.T) {
	var calls []string
	sessions, err := List(testRemote(), fakeExec(&calls))
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("List() returned %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name != "test-session" {
		t.Errorf("session name = %q, want %q", sessions[0].Name, "test-session")
	}
	if sessions[0].Windows != 2 {
		t.Errorf("session windows = %d, want 2", sessions[0].Windows)
	}
}

func TestListEmpty(t *testing.T) {
	sessions, err := List(testRemote(), fakeExecEmpty())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("List() returned %d sessions, want 0", len(sessions))
	}
}

func TestStop(t *testing.T) {
	var calls []string
	err := Stop(testRemote(), "my-session", fakeExec(&calls))
	if err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
}

func TestStopError(t *testing.T) {
	err := Stop(testRemote(), "my-session", fakeExecFail())
	if err == nil {
		t.Error("Stop() should fail when ssh fails")
	}
}

func TestSanitizeSessionName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"uagent-Fix the bug", "uagent-fix-the-bug"},
		{"repo/name with spaces", "repo-name-with-spaces"},
		{"UPPER_case.dots", "upper_case-dots"},
	}
	for _, tt := range tests {
		got := sanitizeSessionName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeSessionName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeSessionNameTruncation(t *testing.T) {
	long := "a-very-long-session-name-that-exceeds-fifty-characters-limit-test"
	got := sanitizeSessionName(long)
	if len(got) > 50 {
		t.Errorf("sanitizeSessionName() returned %d chars, want <= 50", len(got))
	}
}

func TestFirstWords(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"Fix the bug in auth", 3, "Fix-the-bug"},
		{"single", 3, "single"},
		{"", 3, ""},
	}
	for _, tt := range tests {
		got := firstWords(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("firstWords(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}

func TestInit(t *testing.T) {
	var calls []string
	execFn := fakeExec(&calls)

	err := Init(InitOpts{
		Remote:   testRemote(),
		HomeRepo: "git@github.com:user/home.git",
		VaultKey: "/home/user/.secrets/vault.key",
	}, execFn)
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Expect 5 commands: ssh-copy-id, ssh clone, ssh mkdir, scp, ssh bootstrap, ssh myhome init
	if len(calls) < 5 {
		t.Fatalf("expected at least 5 calls, got %d: %v", len(calls), calls)
	}

	// Step 1: ssh-copy-id
	if !containsStr(calls[0], "ssh-copy-id") {
		t.Errorf("call 0: expected ssh-copy-id, got %q", calls[0])
	}

	// Step 2: git clone
	if !containsStr(calls[1], "git clone") {
		t.Errorf("call 1: expected git clone, got %q", calls[1])
	}

	// Step 3: mkdir ~/.secrets
	if !containsStr(calls[2], "mkdir") {
		t.Errorf("call 2: expected mkdir, got %q", calls[2])
	}

	// Step 4: scp vault key
	if !containsStr(calls[3], "scp") {
		t.Errorf("call 3: expected scp, got %q", calls[3])
	}

	// Step 5: bootstrap.sh
	if !containsStr(calls[4], "bootstrap.sh") {
		t.Errorf("call 4: expected bootstrap.sh, got %q", calls[4])
	}

	// Step 6: myhome init
	if len(calls) > 5 && !containsStr(calls[5], "myhome init") {
		t.Errorf("call 5: expected myhome init, got %q", calls[5])
	}
}

func TestInitNoVaultKey(t *testing.T) {
	var calls []string
	execFn := fakeExec(&calls)

	err := Init(InitOpts{
		Remote:   testRemote(),
		HomeRepo: "git@github.com:user/home.git",
	}, execFn)
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Without vault key, should skip mkdir + scp steps (4 calls instead of 6)
	if len(calls) != 4 {
		t.Errorf("expected 4 calls without vault key, got %d: %v", len(calls), calls)
	}
}

func TestInitSSHCopyIDFailure(t *testing.T) {
	err := Init(InitOpts{
		Remote:   testRemote(),
		HomeRepo: "git@github.com:user/home.git",
	}, fakeExecFail())
	if err == nil {
		t.Error("Init() should fail when ssh-copy-id fails")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
}

func TestBuildClaudeCommand(t *testing.T) {
	tests := []struct {
		prompt  string
		auth    string
		command string
		want    string
	}{
		{"Fix bug", "", "", `claude -p "Fix bug"`},
		{"Fix bug", "work", "", `CLAUDE_AUTH_PROFILE=work claude -p "Fix bug"`},
		{"Fix bug", "", "claude --dangerously-skip-permissions -p", `claude --dangerously-skip-permissions -p "Fix bug"`},
		{"Fix bug", "work", "claude -p", `CLAUDE_AUTH_PROFILE=work claude -p "Fix bug"`},
	}
	for _, tt := range tests {
		got := buildClaudeCommand(tt.prompt, tt.auth, tt.command)
		if got != tt.want {
			t.Errorf("buildClaudeCommand(%q, %q, %q) = %q, want %q", tt.prompt, tt.auth, tt.command, got, tt.want)
		}
	}
}
