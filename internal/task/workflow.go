package task

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
)

// DetectCurrentStage determines the current stage by checking detect globs against the worktree.
// Returns the first stage whose detect pattern does NOT match (i.e. work is not yet done).
// If all stages with detect patterns match, returns the next stage without a detect pattern,
// or empty string if all stages are complete.
func DetectCurrentStage(workflow *config.WorkflowConfig, worktreePath string) string {
	for _, stage := range workflow.Stages {
		if stage.Detect == "" {
			// No detection — can't auto-detect completion, assume this is the current stage
			// unless we're past it (checked by task.Stage field)
			return stage.Name
		}
		pattern := filepath.Join(worktreePath, stage.Detect)
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			return stage.Name
		}
	}
	// All stages with detect patterns matched — workflow is complete
	return ""
}

// NextStage returns the stage after the given current stage name.
// Returns empty string if currentStage is the last stage or not found.
func NextStage(workflow *config.WorkflowConfig, currentStage string) string {
	for i, stage := range workflow.Stages {
		if stage.Name == currentStage && i+1 < len(workflow.Stages) {
			return workflow.Stages[i+1].Name
		}
	}
	return ""
}

// StageIndex returns the index of the named stage, or -1 if not found.
func StageIndex(workflow *config.WorkflowConfig, stageName string) int {
	for i, stage := range workflow.Stages {
		if stage.Name == stageName {
			return i
		}
	}
	return -1
}

// StagePrompt returns the prompt for the given stage name, or error if not found.
func StagePrompt(workflow *config.WorkflowConfig, stageName string) (string, error) {
	for _, stage := range workflow.Stages {
		if stage.Name == stageName {
			return stage.Prompt, nil
		}
	}
	return "", fmt.Errorf("stage %q not found in workflow", stageName)
}

// ResolveStagePrompt interpolates workflow params into a stage prompt template.
// Template variables use Go template syntax: {{.paramName}}.
func ResolveStagePrompt(prompt string, params map[string]string) string {
	result := prompt
	for k, v := range params {
		result = strings.ReplaceAll(result, "{{."+k+"}}", v)
	}
	return result
}

// ValidateWorkflowParams checks that all required params are provided.
func ValidateWorkflowParams(workflow *config.WorkflowConfig, params map[string]string) error {
	for _, p := range workflow.Params {
		if p.Required {
			if val, ok := params[p.Name]; !ok || val == "" {
				return fmt.Errorf("required workflow param %q not provided", p.Name)
			}
		}
	}
	return nil
}

// AdvanceStage detects the appropriate next stage for a task based on worktree state.
// It uses detect globs to find the first incomplete stage at or after the task's current stage.
// Returns the stage name to run, or error if workflow is complete.
func AdvanceStage(t *Task, workflow *config.WorkflowConfig) (string, error) {
	if t.Stage == "" {
		// No stage set yet — detect from worktree
		detected := DetectCurrentStage(workflow, t.WorktreePath)
		if detected == "" {
			return "", fmt.Errorf("all workflow stages are complete")
		}
		return detected, nil
	}

	// Current stage exists — find the next one, but verify using detection
	currentIdx := StageIndex(workflow, t.Stage)
	if currentIdx < 0 {
		return "", fmt.Errorf("current stage %q not found in workflow", t.Stage)
	}

	// Check stages from current onwards using detection
	for i := currentIdx; i < len(workflow.Stages); i++ {
		stage := workflow.Stages[i]
		if stage.Detect == "" {
			// If this is the current stage and it was already run, skip to next
			if i == currentIdx && t.StageStatus == StageStatusComplete {
				continue
			}
			return stage.Name, nil
		}
		pattern := filepath.Join(t.WorktreePath, stage.Detect)
		matches, _ := filepath.Glob(pattern)
		if len(matches) == 0 {
			return stage.Name, nil
		}
		// Stage's detect pattern matched — it's complete, check next
	}

	return "", fmt.Errorf("all workflow stages are complete")
}
