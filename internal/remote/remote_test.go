package remote

import (
	"os/exec"
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

func TestBuildClaudeCommand(t *testing.T) {
	tests := []struct {
		prompt string
		auth   string
		want   string
	}{
		{"Fix bug", "", `claude -p "Fix bug"`},
		{"Fix bug", "work", `CLAUDE_AUTH_PROFILE=work claude -p "Fix bug"`},
	}
	for _, tt := range tests {
		got := buildClaudeCommand(tt.prompt, tt.auth)
		if got != tt.want {
			t.Errorf("buildClaudeCommand(%q, %q) = %q, want %q", tt.prompt, tt.auth, got, tt.want)
		}
	}
}
