package task

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
)

// EnrichPrompt augments a task description by running pre_run hooks (pattern-matched
// shell commands that fetch external context like issue bodies) and appending the
// global task_suffix (workflow instructions like "commit, test, push").
func EnrichPrompt(description string, repo *config.Repo, taskSuffix string, execFn ExecFunc) string {
	var extra []string

	// Run pre_run hooks: if description matches a pattern, run the command and capture output
	if repo != nil {
		for _, hook := range repo.PreRun {
			re, err := regexp.Compile(hook.Match)
			if err != nil {
				continue
			}
			matches := re.FindStringSubmatch(description)
			if matches == nil {
				continue
			}
			// Substitute capture groups ($1, $2, ...) into the command
			cmd := hook.Run
			for i, m := range matches {
				if i == 0 {
					continue
				}
				cmd = strings.ReplaceAll(cmd, fmt.Sprintf("$%d", i), m)
			}
			// Execute on host, capture output
			out := runShellCommand(cmd, execFn)
			if out != "" {
				extra = append(extra, out)
			}
		}
	}

	// Append task_suffix (workflow instructions)
	if taskSuffix != "" {
		extra = append(extra, taskSuffix)
	}

	if len(extra) == 0 {
		return description
	}
	return description + "\n\n" + strings.Join(extra, "\n\n")
}

// runShellCommand runs a shell command and returns its stdout, or empty string on error.
func runShellCommand(cmd string, execFn ExecFunc) string {
	c := execFn("sh", "-c", cmd)
	var stdout bytes.Buffer
	c.Stdout = &stdout
	if err := c.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}
