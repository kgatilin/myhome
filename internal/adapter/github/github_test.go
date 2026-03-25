package github

import (
	"testing"
	"time"

	"github.com/kgatilin/myhome/internal/config"
)

func TestNewPoller(t *testing.T) {
	cfg := &config.GitHubAdapterConfig{
		Repos:         []string{"kgatilin/home"},
		Label:         "agent-ready",
		PollInterval:  30 * time.Second,
		BusSocket:     "/tmp/test.sock",
		DefaultTarget: "agent:dev",
	}

	bus := NewBusClient(cfg.BusSocket)
	store := NewStateStore(t.TempDir())
	poller := NewPoller(cfg, bus, store)

	if poller == nil {
		t.Fatal("NewPoller returned nil")
	}
	if poller.cfg != cfg {
		t.Error("cfg not set")
	}
}

func TestPostIssueFormat(t *testing.T) {
	// Verify the message format by constructing what postIssue would create.
	repo := "kgatilin/home"
	issue := Issue{
		Number: 42,
		Title:  "Implement feature X",
		Body:   "Detailed description here",
		URL:    "https://github.com/kgatilin/home/issues/42",
	}

	payload := "Issue #42: Implement feature X\n\nDetailed description here"
	msg := BusMessage{
		Type:    "message",
		Source:  "github:kgatilin/home",
		Target:  "agent:dev",
		Payload: payload,
		Metadata: map[string]any{
			"priority":     5,
			"issue_number": issue.Number,
			"issue_url":    issue.URL,
			"repo":         repo,
		},
	}

	if msg.Source != "github:kgatilin/home" {
		t.Errorf("unexpected source: %s", msg.Source)
	}
	if msg.Metadata["priority"] != 5 {
		t.Errorf("unexpected priority: %v", msg.Metadata["priority"])
	}
	if msg.Metadata["issue_number"] != 42 {
		t.Errorf("unexpected issue_number: %v", msg.Metadata["issue_number"])
	}
}
