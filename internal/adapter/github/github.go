// Package github implements a polling adapter that watches GitHub repos for
// issues labeled "agent-ready" and posts them as tasks to the deskd message bus.
package github

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/kgatilin/myhome/internal/config"
)

// Issue represents a GitHub issue returned by gh CLI.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
}

// Poller polls GitHub repos for issues and posts them to the deskd bus.
type Poller struct {
	cfg   *config.GitHubAdapterConfig
	bus   *BusClient
	store *StateStore
}

// NewPoller creates a new GitHub issue poller.
func NewPoller(cfg *config.GitHubAdapterConfig, bus *BusClient, store *StateStore) *Poller {
	return &Poller{cfg: cfg, bus: bus, store: store}
}

// Run starts the polling loop. Blocks until an error occurs.
func (p *Poller) Run() error {
	if err := p.bus.Connect(); err != nil {
		return fmt.Errorf("bus connect: %w", err)
	}
	defer p.bus.Close()

	log.Printf("github adapter started, polling %d repos every %s", len(p.cfg.Repos), p.cfg.PollInterval)

	// Poll once immediately, then on interval.
	if err := p.poll(); err != nil {
		log.Printf("poll error: %v", err)
	}

	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := p.poll(); err != nil {
			log.Printf("poll error: %v", err)
		}
	}
	return nil
}

// poll checks all configured repos for new agent-ready issues.
func (p *Poller) poll() error {
	state, err := p.store.Load()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	for _, repo := range p.cfg.Repos {
		issues, err := p.listIssues(repo)
		if err != nil {
			log.Printf("list issues for %s: %v", repo, err)
			continue
		}

		for _, issue := range issues {
			key := IssueKey(repo, issue.Number)
			if _, seen := state.PostedIssues[key]; seen {
				continue
			}

			if err := p.postIssue(repo, issue); err != nil {
				log.Printf("post issue %s: %v", key, err)
				continue
			}

			if err := p.markInProgress(repo, issue.Number); err != nil {
				log.Printf("mark in-progress %s: %v", key, err)
			}

			state.PostedIssues[key] = PostedIssue{
				PostedAt: time.Now(),
				Title:    issue.Title,
			}
			log.Printf("posted %s: %s", key, issue.Title)
		}
	}

	return p.store.Save(state)
}

// listIssues fetches open issues with the configured label from a repo using gh CLI.
func (p *Poller) listIssues(repo string) ([]Issue, error) {
	out, err := exec.Command("gh", "issue", "list",
		"--repo", repo,
		"--label", p.cfg.Label,
		"--state", "open",
		"--json", "number,title,body,url",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue list: %w", err)
	}

	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parse gh output: %w", err)
	}
	return issues, nil
}

// postIssue sends an issue to the deskd bus.
func (p *Poller) postIssue(repo string, issue Issue) error {
	payload := fmt.Sprintf("Issue #%d: %s\n\n%s", issue.Number, issue.Title, issue.Body)
	msg := BusMessage{
		Type:    "message",
		ID:      fmt.Sprintf("github-%s-%d-%d", repo, issue.Number, time.Now().UnixMilli()),
		Source:  fmt.Sprintf("github:%s", repo),
		Target:  p.cfg.DefaultTarget,
		Payload: payload,
		Metadata: map[string]any{
			"priority":     5,
			"issue_number": issue.Number,
			"issue_url":    issue.URL,
			"repo":         repo,
		},
	}
	return p.bus.Publish(msg)
}

// markInProgress adds the "in-progress" label to an issue via gh CLI.
func (p *Poller) markInProgress(repo string, number int) error {
	return exec.Command("gh", "issue", "edit",
		"--repo", repo,
		fmt.Sprintf("%d", number),
		"--add-label", "in-progress",
	).Run()
}
