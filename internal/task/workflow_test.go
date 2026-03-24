package task

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kgatilin/myhome/internal/config"
	"gopkg.in/yaml.v3"
)

func testWorkflow() *config.WorkflowConfig {
	return &config.WorkflowConfig{
		Params: []config.WorkflowParam{
			{Name: "ticket", Required: true},
		},
		Stages: []config.WorkflowStage{
			{Name: "start", Prompt: "/jira_ticket_start {{.ticket}}"},
			{Name: "plan", Prompt: "/jira_ticket_plan", Detect: "docs/features/*/plan.md"},
			{Name: "implement", Prompt: "/jira_ticket_implement", Detect: "docs/features/*/implementation.md"},
			{Name: "review", Prompt: "/jira_ticket_review", Detect: "docs/features/*/review.md"},
			{Name: "qa", Prompt: "/jira_ticket_qa"},
			{Name: "learn", Prompt: "/learn", Detect: "docs/features/*/learnings.md"},
			{Name: "merge", Prompt: "/jira_ticket_merge"},
		},
	}
}

func TestDetectCurrentStage(t *testing.T) {
	dir := t.TempDir()
	wf := testWorkflow()

	// No files exist — first stage with detect (plan) should be detected,
	// but "start" has no detect so it returns "start"
	got := DetectCurrentStage(wf, dir)
	if got != "start" {
		t.Errorf("empty dir: got %q, want %q", got, "start")
	}

	// Create plan file — "start" has no detect so still returns "start"
	// But start is the first stage without detect, so it's the blocking one
	// Actually — "start" has no detect, so DetectCurrentStage returns "start"
	// because we can't auto-detect its completion
}

func TestDetectCurrentStageWithDetectFiles(t *testing.T) {
	dir := t.TempDir()

	// Workflow with all stages having detect patterns
	wf := &config.WorkflowConfig{
		Stages: []config.WorkflowStage{
			{Name: "plan", Prompt: "/plan", Detect: "plan.md"},
			{Name: "implement", Prompt: "/implement", Detect: "impl.md"},
			{Name: "review", Prompt: "/review", Detect: "review.md"},
		},
	}

	// No files — should detect "plan" as current
	got := DetectCurrentStage(wf, dir)
	if got != "plan" {
		t.Errorf("empty dir: got %q, want %q", got, "plan")
	}

	// Create plan.md — should advance to "implement"
	os.WriteFile(filepath.Join(dir, "plan.md"), []byte("plan"), 0o644)
	got = DetectCurrentStage(wf, dir)
	if got != "implement" {
		t.Errorf("after plan.md: got %q, want %q", got, "implement")
	}

	// Create impl.md — should advance to "review"
	os.WriteFile(filepath.Join(dir, "impl.md"), []byte("impl"), 0o644)
	got = DetectCurrentStage(wf, dir)
	if got != "review" {
		t.Errorf("after impl.md: got %q, want %q", got, "review")
	}

	// Create review.md — all complete
	os.WriteFile(filepath.Join(dir, "review.md"), []byte("review"), 0o644)
	got = DetectCurrentStage(wf, dir)
	if got != "" {
		t.Errorf("all complete: got %q, want empty", got)
	}
}

func TestDetectCurrentStageGlobPattern(t *testing.T) {
	dir := t.TempDir()

	wf := &config.WorkflowConfig{
		Stages: []config.WorkflowStage{
			{Name: "plan", Prompt: "/plan", Detect: "docs/features/*/plan.md"},
			{Name: "implement", Prompt: "/impl", Detect: "docs/features/*/impl.md"},
		},
	}

	// No files — should detect "plan"
	got := DetectCurrentStage(wf, dir)
	if got != "plan" {
		t.Errorf("empty: got %q, want %q", got, "plan")
	}

	// Create matching file for plan
	featureDir := filepath.Join(dir, "docs", "features", "TICKET-123")
	os.MkdirAll(featureDir, 0o755)
	os.WriteFile(filepath.Join(featureDir, "plan.md"), []byte("plan"), 0o644)

	got = DetectCurrentStage(wf, dir)
	if got != "implement" {
		t.Errorf("after plan: got %q, want %q", got, "implement")
	}
}

func TestNextStage(t *testing.T) {
	wf := testWorkflow()

	tests := []struct {
		current string
		want    string
	}{
		{"start", "plan"},
		{"plan", "implement"},
		{"implement", "review"},
		{"review", "qa"},
		{"qa", "learn"},
		{"learn", "merge"},
		{"merge", ""},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		got := NextStage(wf, tt.current)
		if got != tt.want {
			t.Errorf("NextStage(%q): got %q, want %q", tt.current, got, tt.want)
		}
	}
}

func TestStageIndex(t *testing.T) {
	wf := testWorkflow()

	tests := []struct {
		name string
		want int
	}{
		{"start", 0},
		{"plan", 1},
		{"merge", 6},
		{"nonexistent", -1},
	}

	for _, tt := range tests {
		got := StageIndex(wf, tt.name)
		if got != tt.want {
			t.Errorf("StageIndex(%q): got %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestStagePrompt(t *testing.T) {
	wf := testWorkflow()

	prompt, err := StagePrompt(wf, "start")
	if err != nil {
		t.Fatalf("StagePrompt: %v", err)
	}
	if prompt != "/jira_ticket_start {{.ticket}}" {
		t.Errorf("got %q", prompt)
	}

	_, err = StagePrompt(wf, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent stage")
	}
}

func TestResolveStagePrompt(t *testing.T) {
	tests := []struct {
		prompt string
		params map[string]string
		want   string
	}{
		{
			"/jira_ticket_start {{.ticket}}",
			map[string]string{"ticket": "UAGENT-500"},
			"/jira_ticket_start UAGENT-500",
		},
		{
			"/plan",
			map[string]string{"ticket": "UAGENT-500"},
			"/plan",
		},
		{
			"{{.ticket}} and {{.branch}}",
			map[string]string{"ticket": "T-1", "branch": "main"},
			"T-1 and main",
		},
		{
			"no params here",
			nil,
			"no params here",
		},
	}

	for _, tt := range tests {
		got := ResolveStagePrompt(tt.prompt, tt.params)
		if got != tt.want {
			t.Errorf("ResolveStagePrompt(%q, %v): got %q, want %q", tt.prompt, tt.params, got, tt.want)
		}
	}
}

func TestValidateWorkflowParams(t *testing.T) {
	wf := testWorkflow()

	// Missing required param
	err := ValidateWorkflowParams(wf, map[string]string{})
	if err == nil {
		t.Error("expected error for missing required param")
	}

	// Empty required param
	err = ValidateWorkflowParams(wf, map[string]string{"ticket": ""})
	if err == nil {
		t.Error("expected error for empty required param")
	}

	// Valid
	err = ValidateWorkflowParams(wf, map[string]string{"ticket": "UAGENT-500"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// No required params
	wf2 := &config.WorkflowConfig{
		Params: []config.WorkflowParam{{Name: "optional"}},
		Stages: []config.WorkflowStage{{Name: "s1", Prompt: "p"}},
	}
	err = ValidateWorkflowParams(wf2, map[string]string{})
	if err != nil {
		t.Errorf("unexpected error for optional param: %v", err)
	}
}

func TestAdvanceStage(t *testing.T) {
	dir := t.TempDir()

	wf := &config.WorkflowConfig{
		Stages: []config.WorkflowStage{
			{Name: "plan", Prompt: "/plan", Detect: "plan.md"},
			{Name: "implement", Prompt: "/impl", Detect: "impl.md"},
			{Name: "review", Prompt: "/review"},
		},
	}

	// No stage set, no files — should return "plan"
	tk := &Task{WorktreePath: dir}
	stage, err := AdvanceStage(tk, wf)
	if err != nil {
		t.Fatalf("AdvanceStage: %v", err)
	}
	if stage != "plan" {
		t.Errorf("got %q, want %q", stage, "plan")
	}

	// Task at "plan" stage, plan.md exists → advance to "implement"
	os.WriteFile(filepath.Join(dir, "plan.md"), []byte("done"), 0o644)
	tk.Stage = "plan"
	tk.StageStatus = StageStatusComplete
	stage, err = AdvanceStage(tk, wf)
	if err != nil {
		t.Fatalf("AdvanceStage: %v", err)
	}
	if stage != "implement" {
		t.Errorf("got %q, want %q", stage, "implement")
	}

	// Task at "implement", impl.md exists → advance to "review"
	os.WriteFile(filepath.Join(dir, "impl.md"), []byte("done"), 0o644)
	tk.Stage = "implement"
	tk.StageStatus = StageStatusComplete
	stage, err = AdvanceStage(tk, wf)
	if err != nil {
		t.Fatalf("AdvanceStage: %v", err)
	}
	if stage != "review" {
		t.Errorf("got %q, want %q", stage, "review")
	}

	// Task at "review" (no detect), marked complete → all done
	tk.Stage = "review"
	tk.StageStatus = StageStatusComplete
	_, err = AdvanceStage(tk, wf)
	if err == nil {
		t.Error("expected error when all stages complete")
	}
}

func TestAdvanceStageDetectsSkippedStages(t *testing.T) {
	dir := t.TempDir()

	wf := &config.WorkflowConfig{
		Stages: []config.WorkflowStage{
			{Name: "plan", Prompt: "/plan", Detect: "plan.md"},
			{Name: "implement", Prompt: "/impl", Detect: "impl.md"},
			{Name: "review", Prompt: "/review", Detect: "review.md"},
		},
	}

	// All detect files exist from the start — workflow should be complete
	os.WriteFile(filepath.Join(dir, "plan.md"), []byte("done"), 0o644)
	os.WriteFile(filepath.Join(dir, "impl.md"), []byte("done"), 0o644)
	os.WriteFile(filepath.Join(dir, "review.md"), []byte("done"), 0o644)

	tk := &Task{WorktreePath: dir}
	_, err := AdvanceStage(tk, wf)
	if err == nil {
		t.Error("expected error when all stages complete")
	}
}

func TestTaskYAMLRoundTripWithWorkflow(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	original := &Task{
		ID:          42,
		Type:        TaskTypeRun,
		Domain:      "work",
		Description: "/jira_ticket_start UAGENT-500",
		Status:      TaskStatusRunning,
		CreatedAt:   now,
		Repo:        "uagent",
		Branch:      "UAGENT-500",
		Stage:       "start",
		StageStatus: StageStatusRunning,
		WorkflowParams: map[string]string{
			"ticket": "UAGENT-500",
		},
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var restored Task
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if restored.Stage != "start" {
		t.Errorf("Stage: got %q, want %q", restored.Stage, "start")
	}
	if restored.StageStatus != StageStatusRunning {
		t.Errorf("StageStatus: got %q, want %q", restored.StageStatus, StageStatusRunning)
	}
	if restored.WorkflowParams["ticket"] != "UAGENT-500" {
		t.Errorf("WorkflowParams[ticket]: got %q, want %q", restored.WorkflowParams["ticket"], "UAGENT-500")
	}
}
