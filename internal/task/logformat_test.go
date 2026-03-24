package task

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogFormatter_Process(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		excludes []string
	}{
		{
			name:     "non-json passthrough",
			input:    "some plain text\n",
			contains: []string{"some plain text"},
		},
		{
			name:     "empty lines skipped",
			input:    "\n\n\n",
			contains: []string{},
		},
		{
			name:     "system init message",
			input:    `{"type":"system","subtype":"init","model":"opus","session_id":"abc123456789xyz"}` + "\n",
			contains: []string{"Session started", "model=opus", "abc123456789"},
		},
		{
			name: "assistant text message",
			input: `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world"}]}}` + "\n",
			contains: []string{"Hello world"},
		},
		{
			name: "assistant tool use",
			input: `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls -la"}}]}}` + "\n",
			contains: []string{"Bash", "ls -la"},
		},
		{
			name: "result message",
			input: `{"type":"result","subtype":"success","result":"done","total_cost_usd":0.05,"num_turns":3,"duration_ms":12000}` + "\n",
			contains: []string{"success", "turns=3", "$0.0500", "12s"},
		},
		{
			name: "tool error",
			input: `{"type":"user","message":{"content":[{"type":"tool_result","is_error":true}]}}` + "\n",
			contains: []string{"Tool error"},
		},
		{
			name:     "unknown type ignored",
			input:    `{"type":"unknown","data":"foo"}` + "\n",
			contains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			f := NewLogFormatter(&buf)
			f.Process(strings.NewReader(tt.input))
			output := buf.String()

			for _, s := range tt.contains {
				if !strings.Contains(output, s) {
					t.Errorf("output %q does not contain %q", output, s)
				}
			}
			for _, s := range tt.excludes {
				if strings.Contains(output, s) {
					t.Errorf("output %q should not contain %q", output, s)
				}
			}
		})
	}
}

func TestToolSummary(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		input  map[string]any
		expect string
	}{
		{"read", "Read", map[string]any{"file_path": "/foo/bar.go"}, "/foo/bar.go"},
		{"bash", "Bash", map[string]any{"command": "go test ./..."}, "go test ./..."},
		{"grep", "Grep", map[string]any{"pattern": "TODO"}, "TODO"},
		{"unknown", "CustomTool", map[string]any{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toolSummary(tt.tool, tt.input)
			if got != tt.expect {
				t.Errorf("toolSummary(%q) = %q, want %q", tt.tool, got, tt.expect)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("this is a long string", 10); got != "this is a ..." {
		t.Errorf("truncate long = %q", got)
	}
	if got := truncate("line1\nline2", 20); got != "line1 line2" {
		t.Errorf("truncate newline = %q", got)
	}
}
