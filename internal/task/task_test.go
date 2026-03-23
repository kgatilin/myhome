package task

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

func TestStoreCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	_, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	for _, sub := range []string{"active", "done", "logs"} {
		path := filepath.Join(dir, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("directory %s not created: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
	}
}

func TestNextID(t *testing.T) {
	store := newTestStore(t)

	// Empty store starts at 1
	id, err := store.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != 1 {
		t.Errorf("first NextID: got %d, want 1", id)
	}

	// Save a task, next ID should be 2
	task := &Task{ID: 1, Status: TaskStatusOpen}
	if err := store.Save(task); err != nil {
		t.Fatalf("Save: %v", err)
	}
	id, err = store.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != 2 {
		t.Errorf("after save: got %d, want 2", id)
	}

	// Save a done task with higher ID
	task2 := &Task{ID: 5, Status: TaskStatusDone}
	if err := store.Save(task2); err != nil {
		t.Fatalf("Save: %v", err)
	}
	id, err = store.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != 6 {
		t.Errorf("after done task: got %d, want 6", id)
	}
}

func TestSaveAndLoad(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().Truncate(time.Second)

	task := &Task{
		ID:          1,
		Type:        TaskTypeGeneral,
		Domain:      "work",
		Description: "fix the bug",
		Status:      TaskStatusOpen,
		CreatedAt:   now,
	}

	if err := store.Save(task); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(1)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.ID != 1 {
		t.Errorf("ID: got %d, want 1", loaded.ID)
	}
	if loaded.Type != TaskTypeGeneral {
		t.Errorf("Type: got %q, want %q", loaded.Type, TaskTypeGeneral)
	}
	if loaded.Domain != "work" {
		t.Errorf("Domain: got %q, want %q", loaded.Domain, "work")
	}
	if loaded.Description != "fix the bug" {
		t.Errorf("Description: got %q, want %q", loaded.Description, "fix the bug")
	}
	if loaded.Status != TaskStatusOpen {
		t.Errorf("Status: got %q, want %q", loaded.Status, TaskStatusOpen)
	}
}

func TestLoadNotFound(t *testing.T) {
	store := newTestStore(t)
	_, err := store.Load(999)
	if err == nil {
		t.Error("expected error for missing task")
	}
}

func TestSaveRunTask(t *testing.T) {
	store := newTestStore(t)

	task := &Task{
		ID:           1,
		Type:         TaskTypeRun,
		Description:  "run tests",
		Status:       TaskStatusRunning,
		CreatedAt:    time.Now(),
		Repo:         "myrepo",
		Branch:       "feature-x",
		ContainerID:  "abc123",
		Container:    "claude-code",
		AuthProfile:  "work",
		WorktreePath: "/home/user/dev/myrepo/.worktrees/feature-x",
		LogFile:      "/tmp/tasks/logs/1.log",
	}

	if err := store.Save(task); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Should be in active/ since status is running
	path := filepath.Join(store.tasksDir, "active", "1.yml")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("task not in active dir: %v", err)
	}

	loaded, err := store.Load(1)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ContainerID != "abc123" {
		t.Errorf("ContainerID: got %q, want %q", loaded.ContainerID, "abc123")
	}
	if loaded.WorktreePath != "/home/user/dev/myrepo/.worktrees/feature-x" {
		t.Errorf("WorktreePath mismatch")
	}
}

func TestList(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	tasks := []*Task{
		{ID: 1, Type: TaskTypeGeneral, Domain: "work", Status: TaskStatusOpen, CreatedAt: now},
		{ID: 2, Type: TaskTypeGeneral, Domain: "dev", Status: TaskStatusOpen, CreatedAt: now},
		{ID: 3, Type: TaskTypeRun, Domain: "work", Status: TaskStatusRunning, CreatedAt: now},
		{ID: 4, Type: TaskTypeGeneral, Domain: "work", Status: TaskStatusDone, CreatedAt: now},
	}
	for _, task := range tasks {
		if err := store.Save(task); err != nil {
			t.Fatalf("Save task %d: %v", task.ID, err)
		}
	}

	tests := []struct {
		name   string
		filter ListFilter
		want   int
	}{
		{"no filter", ListFilter{}, 4},
		{"open only", ListFilter{Status: TaskStatusOpen}, 2},
		{"running only", ListFilter{Status: TaskStatusRunning}, 1},
		{"done only", ListFilter{Status: TaskStatusDone}, 1},
		{"domain work", ListFilter{Domain: "work"}, 3},
		{"domain dev", ListFilter{Domain: "dev"}, 1},
		{"open + work", ListFilter{Status: TaskStatusOpen, Domain: "work"}, 1},
		{"nonexistent domain", ListFilter{Domain: "life"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := store.List(tt.filter)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(result) != tt.want {
				t.Errorf("got %d tasks, want %d", len(result), tt.want)
			}
		})
	}
}

func TestMarkDone(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	task := &Task{
		ID:        1,
		Type:      TaskTypeGeneral,
		Status:    TaskStatusOpen,
		CreatedAt: now,
	}
	if err := store.Save(task); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.MarkDone(1); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}

	// Should be gone from active/
	activePath := filepath.Join(store.tasksDir, "active", "1.yml")
	if _, err := os.Stat(activePath); !os.IsNotExist(err) {
		t.Error("task still in active/ after MarkDone")
	}

	// Should be in done/
	donePath := filepath.Join(store.tasksDir, "done", "1.yml")
	if _, err := os.Stat(donePath); err != nil {
		t.Error("task not in done/ after MarkDone")
	}

	loaded, err := store.Load(1)
	if err != nil {
		t.Fatalf("Load after MarkDone: %v", err)
	}
	if loaded.Status != TaskStatusDone {
		t.Errorf("status: got %q, want %q", loaded.Status, TaskStatusDone)
	}
	if loaded.DoneAt == nil {
		t.Error("DoneAt not set")
	}
}

func TestMarkDoneNotFound(t *testing.T) {
	store := newTestStore(t)
	if err := store.MarkDone(999); err == nil {
		t.Error("expected error for missing task")
	}
}

func TestRemove(t *testing.T) {
	store := newTestStore(t)

	task := &Task{ID: 1, Status: TaskStatusOpen, CreatedAt: time.Now()}
	if err := store.Save(task); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Remove(1); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	_, err := store.Load(1)
	if err == nil {
		t.Error("expected error loading removed task")
	}
}

func TestRemoveNotFound(t *testing.T) {
	store := newTestStore(t)
	if err := store.Remove(999); err == nil {
		t.Error("expected error for missing task")
	}
}

func TestLogDir(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	want := filepath.Join(dir, "logs")
	if got := store.LogDir(); got != want {
		t.Errorf("LogDir: got %q, want %q", got, want)
	}
}

func TestTaskYAMLRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	doneAt := now.Add(time.Hour).Truncate(time.Second)

	original := &Task{
		ID:           42,
		Type:         TaskTypeRun,
		Domain:       "work",
		Description:  "implement feature",
		Status:       TaskStatusDone,
		CreatedAt:    now,
		DoneAt:       &doneAt,
		Repo:         "myrepo",
		Branch:       "feat-42",
		ContainerID:  "deadbeef",
		Container:    "claude-code",
		AuthProfile:  "personal",
		WorktreePath: "/home/user/dev/myrepo/.worktrees/feat-42",
		LogFile:      "/tmp/tasks/logs/42.log",
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var restored Task
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if restored.ID != original.ID {
		t.Errorf("ID: got %d, want %d", restored.ID, original.ID)
	}
	if restored.Type != original.Type {
		t.Errorf("Type: got %q, want %q", restored.Type, original.Type)
	}
	if restored.Domain != original.Domain {
		t.Errorf("Domain mismatch")
	}
	if restored.Description != original.Description {
		t.Errorf("Description mismatch")
	}
	if restored.Status != original.Status {
		t.Errorf("Status mismatch")
	}
	if !restored.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt mismatch")
	}
	if restored.DoneAt == nil || !restored.DoneAt.Equal(*original.DoneAt) {
		t.Errorf("DoneAt mismatch")
	}
	if restored.Repo != original.Repo {
		t.Errorf("Repo mismatch")
	}
	if restored.Branch != original.Branch {
		t.Errorf("Branch mismatch")
	}
	if restored.ContainerID != original.ContainerID {
		t.Errorf("ContainerID mismatch")
	}
	if restored.Container != original.Container {
		t.Errorf("Container mismatch")
	}
	if restored.AuthProfile != original.AuthProfile {
		t.Errorf("AuthProfile mismatch")
	}
	if restored.WorktreePath != original.WorktreePath {
		t.Errorf("WorktreePath mismatch")
	}
	if restored.LogFile != original.LogFile {
		t.Errorf("LogFile mismatch")
	}
}

// fakeExecFunc returns an ExecFunc that records calls and produces controlled output.
type execCall struct {
	Name string
	Args []string
}

func fakeExecSuccess(calls *[]execCall, stdout string) ExecFunc {
	return func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, execCall{Name: name, Args: args})
		// Use echo to produce stdout; -n to avoid trailing newline
		if stdout != "" {
			return exec.Command("echo", "-n", stdout)
		}
		return exec.Command("true")
	}
}

func fakeExecFailure(calls *[]execCall) ExecFunc {
	return func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, execCall{Name: name, Args: args})
		return exec.Command("false")
	}
}

func TestRunnerRunTask(t *testing.T) {
	store := newTestStore(t)
	projectDir := t.TempDir()

	// Don't pre-create worktree dir — RunTask should call git worktree add

	var calls []execCall
	callIdx := 0
	execFn := func(name string, args ...string) *exec.Cmd {
		calls = append(calls, execCall{Name: name, Args: args})
		idx := callIdx
		callIdx++
		switch idx {
		case 0: // git worktree add
			return exec.Command("true")
		case 1: // docker run -d
			return exec.Command("echo", "-n", "container-abc123")
		default: // docker logs -f
			return exec.Command("true")
		}
	}

	// Create a task first (as task add would)
	tk := &Task{
		ID:          1,
		Type:        TaskTypeRun,
		Domain:      "dev",
		Description: "test run",
		Status:      TaskStatusOpen,
		CreatedAt:   time.Now(),
		Repo:        "myrepo",
		Branch:      "feat-1",
	}
	if err := store.Save(tk); err != nil {
		t.Fatalf("Save: %v", err)
	}

	runner := NewRunner(store, execFn, "docker")
	err := runner.RunTask(tk, RunOpts{
		ContainerName: "claude-code",
		AuthProfile:   "work",
		ProjectDir:    projectDir,
	})
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}

	// Verify git worktree command
	if len(calls) < 1 {
		t.Fatal("expected at least 1 exec call")
	}
	if calls[0].Name != "git" {
		t.Errorf("first call: got %q, want git", calls[0].Name)
	}
	if calls[0].Args[0] != "worktree" || calls[0].Args[1] != "add" {
		t.Errorf("git args: got %v, want worktree add ...", calls[0].Args)
	}

	// Verify container run command
	if len(calls) < 2 {
		t.Fatal("expected at least 2 exec calls")
	}
	if calls[1].Name != "docker" {
		t.Errorf("second call: got %q, want docker", calls[1].Name)
	}
	if calls[1].Args[0] != "run" || calls[1].Args[1] != "-d" || calls[1].Args[2] != "--rm" {
		t.Errorf("docker args: got %v, want run -d --rm ...", calls[1].Args)
	}
	// Verify prompt is passed to claude
	lastArg := calls[1].Args[len(calls[1].Args)-1]
	if !strings.Contains(lastArg, "test run") {
		t.Errorf("expected prompt in last arg, got %q", lastArg)
	}

	// Verify task was updated in place
	if tk.Status != TaskStatusRunning {
		t.Errorf("Status: got %q, want %q", tk.Status, TaskStatusRunning)
	}
	if tk.ContainerID != "container-abc123" {
		t.Errorf("ContainerID: got %q, want %q", tk.ContainerID, "container-abc123")
	}
	if tk.Container != "claude-code" {
		t.Errorf("Container: got %q, want %q", tk.Container, "claude-code")
	}

	// Verify task was persisted
	loaded, err := store.Load(1)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ContainerID != "container-abc123" {
		t.Errorf("persisted ContainerID: got %q, want %q", loaded.ContainerID, "container-abc123")
	}
}

func TestRunnerRunTaskWorktreeFailure(t *testing.T) {
	store := newTestStore(t)
	projectDir := t.TempDir()

	var calls []execCall
	execFn := fakeExecFailure(&calls)

	tk := &Task{
		ID:     1,
		Type:   TaskTypeRun,
		Status: TaskStatusOpen,
		Repo:   "myrepo",
		Branch: "feat-1",
	}
	if err := store.Save(tk); err != nil {
		t.Fatalf("Save: %v", err)
	}

	runner := NewRunner(store, execFn, "docker")
	err := runner.RunTask(tk, RunOpts{
		ContainerName: "claude-code",
		ProjectDir:    projectDir,
	})
	if err == nil {
		t.Error("expected error when worktree creation fails")
	}
}

func TestTaskAddWithRepoCreatesRunType(t *testing.T) {
	store := newTestStore(t)

	// Task with repo should be TaskTypeRun
	tk := &Task{
		ID:          1,
		Type:        TaskTypeRun,
		Description: "fix the bug",
		Status:      TaskStatusOpen,
		CreatedAt:   time.Now(),
		Repo:        "myrepo",
		Branch:      "fix-123",
	}
	if err := store.Save(tk); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := store.Load(1)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Type != TaskTypeRun {
		t.Errorf("Type: got %q, want %q", loaded.Type, TaskTypeRun)
	}
	if loaded.Repo != "myrepo" {
		t.Errorf("Repo: got %q, want %q", loaded.Repo, "myrepo")
	}
	if loaded.Branch != "fix-123" {
		t.Errorf("Branch: got %q, want %q", loaded.Branch, "fix-123")
	}

	// Task without repo should be TaskTypeGeneral
	tk2 := &Task{
		ID:          2,
		Type:        TaskTypeGeneral,
		Description: "review roadmap",
		Status:      TaskStatusOpen,
		CreatedAt:   time.Now(),
	}
	if err := store.Save(tk2); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded2, err := store.Load(2)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded2.Type != TaskTypeGeneral {
		t.Errorf("Type: got %q, want %q", loaded2.Type, TaskTypeGeneral)
	}
}

func TestRunnerStop(t *testing.T) {
	store := newTestStore(t)

	// Create a running task
	task := &Task{
		ID:          1,
		Type:        TaskTypeRun,
		Status:      TaskStatusRunning,
		ContainerID: "abc123",
		CreatedAt:   time.Now(),
	}
	if err := store.Save(task); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var calls []execCall
	execFn := fakeExecSuccess(&calls, "")
	runner := NewRunner(store, execFn, "docker")

	if err := runner.Stop(1); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(calls))
	}
	if calls[0].Name != "docker" {
		t.Errorf("call name: got %q, want docker", calls[0].Name)
	}
	if calls[0].Args[0] != "stop" || calls[0].Args[1] != "abc123" {
		t.Errorf("stop args: got %v, want [stop abc123]", calls[0].Args)
	}
}

func TestRunnerStopNotRunTask(t *testing.T) {
	store := newTestStore(t)

	task := &Task{
		ID:        1,
		Type:      TaskTypeGeneral,
		Status:    TaskStatusOpen,
		CreatedAt: time.Now(),
	}
	if err := store.Save(task); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var calls []execCall
	execFn := fakeExecSuccess(&calls, "")
	runner := NewRunner(store, execFn, "docker")

	err := runner.Stop(1)
	if err == nil {
		t.Error("expected error stopping non-run task")
	}
}

func TestRunnerStopNoContainerID(t *testing.T) {
	store := newTestStore(t)

	task := &Task{
		ID:        1,
		Type:      TaskTypeRun,
		Status:    TaskStatusRunning,
		CreatedAt: time.Now(),
	}
	if err := store.Save(task); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var calls []execCall
	execFn := fakeExecSuccess(&calls, "")
	runner := NewRunner(store, execFn, "docker")

	err := runner.Stop(1)
	if err == nil {
		t.Error("expected error for task with no container ID")
	}
}

func TestRunnerWithCustomRuntime(t *testing.T) {
	store := newTestStore(t)

	task := &Task{
		ID:          1,
		Type:        TaskTypeRun,
		Status:      TaskStatusRunning,
		ContainerID: "abc123",
		CreatedAt:   time.Now(),
	}
	if err := store.Save(task); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var calls []execCall
	execFn := fakeExecSuccess(&calls, "")
	runner := NewRunner(store, execFn, "nerdctl")

	if err := runner.Stop(1); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if calls[0].Name != "nerdctl" {
		t.Errorf("runtime: got %q, want nerdctl", calls[0].Name)
	}
}
