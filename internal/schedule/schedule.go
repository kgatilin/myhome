package schedule

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kgatilin/myhome/internal/config"
)

// ExecFunc creates an *exec.Cmd for the given command and arguments.
type ExecFunc func(name string, args ...string) *exec.Cmd

// DefaultExec uses os/exec.Command.
func DefaultExec(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// ScheduleInfo holds display information about a scheduled task.
type ScheduleInfo struct {
	ID        string
	Prompt    string
	Cron      string
	Container string
	Workdir   string
	Installed bool
}

// ResolveTemplateVars replaces template variables in a string.
// Supported: {date}, {year}, {month}, {week}, {day}, {domain}
func ResolveTemplateVars(s string, now time.Time, domain string) string {
	_, week := now.ISOWeek()
	replacements := map[string]string{
		"{date}":   now.Format("2006-01-02"),
		"{year}":   strconv.Itoa(now.Year()),
		"{month}":  fmt.Sprintf("%02d", now.Month()),
		"{week}":   fmt.Sprintf("%02d", week),
		"{day}":    fmt.Sprintf("%02d", now.Day()),
		"{domain}": domain,
	}
	for k, v := range replacements {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}

// Install creates a launchd plist (macOS) or cron entry (Linux) for a schedule.
func Install(sched config.Schedule, myhomeBin, homeDir string, execFn ExecFunc) error {
	if execFn == nil {
		execFn = DefaultExec
	}

	if runtime.GOOS == "darwin" {
		return installLaunchd(sched, myhomeBin, homeDir)
	}
	return installCron(sched, myhomeBin, homeDir, execFn)
}

// Remove removes a scheduled task (launchd plist or cron entry).
func Remove(schedID, homeDir string, execFn ExecFunc) error {
	if execFn == nil {
		execFn = DefaultExec
	}

	if runtime.GOOS == "darwin" {
		return removeLaunchd(schedID, homeDir)
	}
	return removeCron(schedID, execFn)
}

// List returns info about all configured schedules with install status.
func List(schedules []config.Schedule, homeDir string) []ScheduleInfo {
	var infos []ScheduleInfo
	for _, s := range schedules {
		info := ScheduleInfo{
			ID:        s.ID,
			Prompt:    s.Prompt,
			Cron:      s.Cron,
			Container: s.Container,
			Workdir:   s.Workdir,
			Installed: isInstalled(s.ID, homeDir),
		}
		infos = append(infos, info)
	}
	return infos
}

// buildCommand constructs the myhome command that the scheduler will run.
func buildCommand(sched config.Schedule, myhomeBin string) string {
	parts := []string{myhomeBin, "task", "run"}

	// The prompt will have template vars resolved at execution time.
	if sched.Container != "" {
		parts = append(parts, "--container", sched.Container)
	}
	if sched.Auth != "" {
		parts = append(parts, "--auth", sched.Auth)
	}

	return strings.Join(parts, " ")
}

// --- macOS: launchd ---

func plistPath(schedID, homeDir string) string {
	return filepath.Join(homeDir, "Library", "LaunchAgents", "com.myhome.schedule."+schedID+".plist")
}

func installLaunchd(sched config.Schedule, myhomeBin, homeDir string) error {
	path := plistPath(sched.ID, homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	minute, hour, dayOfMonth, month, dayOfWeek, err := parseCron(sched.Cron)
	if err != nil {
		return fmt.Errorf("parse cron expression: %w", err)
	}

	// Build the script that resolves template vars at runtime.
	resolvedPrompt := fmt.Sprintf(`$(echo '%s' | sed "s/{date}/$(date +%%Y-%%m-%%d)/g; s/{year}/$(date +%%Y)/g; s/{week}/$(date +%%V)/g; s/{month}/$(date +%%m)/g; s/{day}/$(date +%%d)/g; s/{domain}/%s/g")`, sched.Prompt, sched.Domain)
	cmdLine := buildCommand(sched, myhomeBin) + " " + resolvedPrompt

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.myhome.schedule.%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>/bin/sh</string>
		<string>-c</string>
		<string>%s</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
%s	</dict>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
	<key>WorkingDirectory</key>
	<string>%s</string>
</dict>
</plist>
`,
		sched.ID,
		cmdLine,
		buildCalendarInterval(minute, hour, dayOfMonth, month, dayOfWeek),
		filepath.Join(homeDir, "tasks", "logs", "schedule-"+sched.ID+".log"),
		filepath.Join(homeDir, "tasks", "logs", "schedule-"+sched.ID+".err"),
		resolveWorkdir(sched.Workdir, homeDir),
	)

	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	return nil
}

func removeLaunchd(schedID, homeDir string) error {
	path := plistPath(schedID, homeDir)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	return nil
}

func buildCalendarInterval(minute, hour, dayOfMonth, month, dayOfWeek string) string {
	var parts []string
	if minute != "*" {
		parts = append(parts, fmt.Sprintf("\t\t<key>Minute</key>\n\t\t<integer>%s</integer>", minute))
	}
	if hour != "*" {
		parts = append(parts, fmt.Sprintf("\t\t<key>Hour</key>\n\t\t<integer>%s</integer>", hour))
	}
	if dayOfMonth != "*" {
		parts = append(parts, fmt.Sprintf("\t\t<key>Day</key>\n\t\t<integer>%s</integer>", dayOfMonth))
	}
	if month != "*" {
		parts = append(parts, fmt.Sprintf("\t\t<key>Month</key>\n\t\t<integer>%s</integer>", month))
	}
	if dayOfWeek != "*" {
		parts = append(parts, fmt.Sprintf("\t\t<key>Weekday</key>\n\t\t<integer>%s</integer>", dayOfWeek))
	}
	return strings.Join(parts, "\n") + "\n"
}

// --- Linux: cron ---

func installCron(sched config.Schedule, myhomeBin, homeDir string, execFn ExecFunc) error {
	resolvedPrompt := fmt.Sprintf(`$$(echo '%s' | sed "s/{date}/$$(date +%%Y-%%m-%%d)/g; s/{year}/$$(date +%%Y)/g; s/{week}/$$(date +%%V)/g; s/{month}/$$(date +%%m)/g; s/{day}/$$(date +%%d)/g; s/{domain}/%s/g")`, sched.Prompt, sched.Domain)
	cmdLine := buildCommand(sched, myhomeBin) + " " + resolvedPrompt

	cronLine := fmt.Sprintf("%s cd %s && %s # myhome-schedule:%s",
		sched.Cron,
		resolveWorkdir(sched.Workdir, homeDir),
		cmdLine,
		sched.ID,
	)

	// Read existing crontab, append new entry.
	cmd := execFn("crontab", "-l")
	var existing strings.Builder
	cmd.Stdout = &existing
	cmd.Run() // ignore error (empty crontab)

	lines := strings.TrimSpace(existing.String())
	if lines != "" {
		lines += "\n"
	}
	lines += cronLine + "\n"

	// Write new crontab.
	cmd = execFn("crontab", "-")
	cmd.Stdin = strings.NewReader(lines)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install cron entry: %w", err)
	}
	return nil
}

func removeCron(schedID string, execFn ExecFunc) error {
	cmd := execFn("crontab", "-l")
	var existing strings.Builder
	cmd.Stdout = &existing
	if err := cmd.Run(); err != nil {
		return nil // no crontab
	}

	marker := "# myhome-schedule:" + schedID
	var newLines []string
	for _, line := range strings.Split(existing.String(), "\n") {
		if !strings.Contains(line, marker) {
			newLines = append(newLines, line)
		}
	}

	cmd = execFn("crontab", "-")
	cmd.Stdin = strings.NewReader(strings.Join(newLines, "\n"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("update crontab: %w", err)
	}
	return nil
}

// --- Helpers ---

// parseCron splits a cron expression into its 5 fields.
func parseCron(expr string) (minute, hour, dayOfMonth, month, dayOfWeek string, err error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return "", "", "", "", "", fmt.Errorf("cron expression must have 5 fields, got %d: %q", len(fields), expr)
	}
	return fields[0], fields[1], fields[2], fields[3], fields[4], nil
}

func isInstalled(schedID, homeDir string) bool {
	if runtime.GOOS == "darwin" {
		_, err := os.Stat(plistPath(schedID, homeDir))
		return err == nil
	}
	// On Linux, check crontab for the marker.
	cmd := exec.Command("crontab", "-l")
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false
	}
	return strings.Contains(out.String(), "# myhome-schedule:"+schedID)
}

func resolveWorkdir(workdir, homeDir string) string {
	if workdir == "" {
		return homeDir
	}
	if strings.HasPrefix(workdir, "~/") {
		return filepath.Join(homeDir, workdir[2:])
	}
	return workdir
}
