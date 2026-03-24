package task

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// LogFormatter reads NDJSON stream-json from Claude CLI and writes
// human-readable log output. Non-JSON lines are passed through as-is.
type LogFormatter struct {
	w io.Writer
}

// NewLogFormatter creates a formatter that writes to w.
func NewLogFormatter(w io.Writer) *LogFormatter {
	return &LogFormatter{w: w}
}

// Process reads from r line by line, parsing NDJSON and writing formatted output.
// It blocks until r is closed or an error occurs.
func (f *LogFormatter) Process(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Try to parse as JSON
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Not JSON — pass through (startup commands, errors, etc.)
			fmt.Fprintf(f.w, "%s\n", line)
			continue
		}

		f.formatMessage(msg)
	}
}

func (f *LogFormatter) formatMessage(msg map[string]any) {
	msgType, _ := msg["type"].(string)
	ts := time.Now().Format("15:04:05")

	switch msgType {
	case "system":
		subtype, _ := msg["subtype"].(string)
		if subtype == "init" {
			model, _ := msg["model"].(string)
			sessionID, _ := msg["session_id"].(string)
			fmt.Fprintf(f.w, "[%s] Session started (model=%s, session=%s)\n", ts, model, truncate(sessionID, 12))
		}

	case "assistant":
		message, ok := msg["message"].(map[string]any)
		if !ok {
			return
		}
		content, ok := message["content"].([]any)
		if !ok {
			return
		}
		for _, block := range content {
			b, ok := block.(map[string]any)
			if !ok {
				continue
			}
			blockType, _ := b["type"].(string)
			switch blockType {
			case "text":
				text, _ := b["text"].(string)
				if text != "" {
					// Truncate long text blocks in log
					if len(text) > 500 {
						text = text[:500] + "..."
					}
					fmt.Fprintf(f.w, "[%s] 💬 %s\n", ts, text)
				}
			case "tool_use":
				name, _ := b["name"].(string)
				input, _ := b["input"].(map[string]any)
				summary := toolSummary(name, input)
				fmt.Fprintf(f.w, "[%s] 🔧 %s %s\n", ts, name, summary)
			}
		}

	case "user":
		// Tool results — show brief summary
		message, ok := msg["message"].(map[string]any)
		if !ok {
			return
		}
		content, ok := message["content"].([]any)
		if !ok {
			return
		}
		for _, block := range content {
			b, ok := block.(map[string]any)
			if !ok {
				continue
			}
			blockType, _ := b["type"].(string)
			if blockType == "tool_result" {
				isError, _ := b["is_error"].(bool)
				if isError {
					fmt.Fprintf(f.w, "[%s] ❌ Tool error\n", ts)
				}
			}
		}

	case "result":
		subtype, _ := msg["subtype"].(string)
		result, _ := msg["result"].(string)
		costUSD, _ := msg["total_cost_usd"].(float64)
		numTurns, _ := msg["num_turns"].(float64)
		durationMs, _ := msg["duration_ms"].(float64)

		if len(result) > 500 {
			result = result[:500] + "..."
		}
		fmt.Fprintf(f.w, "[%s] ✅ %s (turns=%.0f, cost=$%.4f, duration=%.0fs)\n", ts, subtype, numTurns, costUSD, durationMs/1000)
		if result != "" {
			fmt.Fprintf(f.w, "[%s] Result: %s\n", ts, result)
		}
	}
}

// toolSummary returns a brief description of a tool call.
func toolSummary(name string, input map[string]any) string {
	switch name {
	case "Read":
		path, _ := input["file_path"].(string)
		return truncate(path, 60)
	case "Write":
		path, _ := input["file_path"].(string)
		return truncate(path, 60)
	case "Edit":
		path, _ := input["file_path"].(string)
		return truncate(path, 60)
	case "Bash":
		cmd, _ := input["command"].(string)
		return truncate(cmd, 80)
	case "Glob":
		pattern, _ := input["pattern"].(string)
		return pattern
	case "Grep":
		pattern, _ := input["pattern"].(string)
		return pattern
	default:
		return ""
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
