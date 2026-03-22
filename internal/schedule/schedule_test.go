package schedule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kgatilin/myhome/internal/config"
)

func TestResolveTemplateVars(t *testing.T) {
	now := time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		input  string
		domain string
		want   string
	}{
		{
			input: "Blog post for {date}",
			want:  "Blog post for 2026-03-22",
		},
		{
			input: "Week {week} of {year}",
			want:  "Week 12 of 2026",
		},
		{
			input:  "{domain} report for {month}/{day}",
			domain: "work",
			want:   "work report for 03/22",
		},
		{
			input: "No vars here",
			want:  "No vars here",
		},
	}

	for _, tt := range tests {
		got := ResolveTemplateVars(tt.input, now, tt.domain)
		if got != tt.want {
			t.Errorf("ResolveTemplateVars(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseCron(t *testing.T) {
	tests := []struct {
		expr    string
		wantMin string
		wantHr  string
		wantErr bool
	}{
		{"0 18 * * 1-5", "0", "18", false},
		{"*/5 * * * *", "*/5", "*", false},
		{"bad", "", "", true},
		{"1 2 3", "", "", true},
	}

	for _, tt := range tests {
		min, hr, _, _, _, err := parseCron(tt.expr)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseCron(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			if min != tt.wantMin {
				t.Errorf("parseCron(%q) minute = %q, want %q", tt.expr, min, tt.wantMin)
			}
			if hr != tt.wantHr {
				t.Errorf("parseCron(%q) hour = %q, want %q", tt.expr, hr, tt.wantHr)
			}
		}
	}
}

func TestResolveWorkdir(t *testing.T) {
	tests := []struct {
		workdir string
		homeDir string
		want    string
	}{
		{"", "/home/user", "/home/user"},
		{"~/work/blog", "/home/user", "/home/user/work/blog"},
		{"/abs/path", "/home/user", "/abs/path"},
	}

	for _, tt := range tests {
		got := resolveWorkdir(tt.workdir, tt.homeDir)
		if got != tt.want {
			t.Errorf("resolveWorkdir(%q, %q) = %q, want %q", tt.workdir, tt.homeDir, got, tt.want)
		}
	}
}

func TestList(t *testing.T) {
	homeDir := t.TempDir()

	schedules := []config.Schedule{
		{ID: "blog", Prompt: "Write blog", Cron: "0 18 * * 1-5", Container: "claude-code", Workdir: "~/work/blog"},
		{ID: "report", Prompt: "Weekly report", Cron: "0 9 * * 1"},
	}

	infos := List(schedules, homeDir)
	if len(infos) != 2 {
		t.Fatalf("List() returned %d infos, want 2", len(infos))
	}
	if infos[0].ID != "blog" {
		t.Errorf("infos[0].ID = %q, want %q", infos[0].ID, "blog")
	}
	if infos[0].Installed {
		t.Error("infos[0].Installed should be false")
	}
}

func TestInstallLaunchd(t *testing.T) {
	homeDir := t.TempDir()

	sched := config.Schedule{
		ID:        "test-blog",
		Prompt:    "Write blog for {date}",
		Cron:      "0 18 * * 1-5",
		Container: "claude-code",
		Auth:      "work",
		Workdir:   "~/work/blog",
		Domain:    "work",
	}

	err := installLaunchd(sched, "/usr/local/bin/myhome", homeDir)
	if err != nil {
		t.Fatalf("installLaunchd() error: %v", err)
	}

	plist := plistPath("test-blog", homeDir)
	data, err := os.ReadFile(plist)
	if err != nil {
		t.Fatalf("read plist: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "com.myhome.schedule.test-blog") {
		t.Error("plist missing label")
	}
	if !strings.Contains(content, "Hour") {
		t.Error("plist missing Hour key")
	}
	if !strings.Contains(content, "18") {
		t.Error("plist missing hour value 18")
	}
	if !strings.Contains(content, "Weekday") {
		t.Error("plist missing Weekday for 1-5 cron")
	}
}

func TestRemoveLaunchd(t *testing.T) {
	homeDir := t.TempDir()

	// Create a fake plist.
	plist := plistPath("test", homeDir)
	os.MkdirAll(filepath.Dir(plist), 0o755)
	os.WriteFile(plist, []byte("fake"), 0o644)

	err := removeLaunchd("test", homeDir)
	if err != nil {
		t.Fatalf("removeLaunchd() error: %v", err)
	}

	if _, err := os.Stat(plist); !os.IsNotExist(err) {
		t.Error("plist should be removed")
	}
}

func TestRemoveLaunchdNotExist(t *testing.T) {
	homeDir := t.TempDir()
	err := removeLaunchd("nonexistent", homeDir)
	if err != nil {
		t.Fatalf("removeLaunchd() should not error for non-existent: %v", err)
	}
}

func TestBuildCommand(t *testing.T) {
	sched := config.Schedule{
		Container: "claude-code",
		Auth:      "work",
	}
	got := buildCommand(sched, "/usr/local/bin/myhome")
	if !strings.Contains(got, "task run") {
		t.Errorf("buildCommand() = %q, missing 'task run'", got)
	}
	if !strings.Contains(got, "--container claude-code") {
		t.Errorf("buildCommand() = %q, missing container flag", got)
	}
	if !strings.Contains(got, "--auth work") {
		t.Errorf("buildCommand() = %q, missing auth flag", got)
	}
}

func TestBuildCalendarInterval(t *testing.T) {
	got := buildCalendarInterval("0", "18", "*", "*", "1-5")
	if !strings.Contains(got, "Minute") {
		t.Error("missing Minute")
	}
	if !strings.Contains(got, "Hour") {
		t.Error("missing Hour")
	}
	if !strings.Contains(got, "Weekday") {
		t.Error("missing Weekday")
	}
	if strings.Contains(got, "Day") && strings.Contains(got, "<key>Day</key>") {
		t.Error("should not include Day when dayOfMonth is *")
	}
}
