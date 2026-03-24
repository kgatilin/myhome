package daemon

// This file will contain message routing logic between input sources (CLI, webhooks,
// scheduled triggers) and agents. The router matches incoming messages to the
// appropriate agent based on context (repo, branch, task ID).
//
// Currently, routing is handled directly by the daemon's dispatch method.
// When cross-agent messaging is needed, this file will implement the RouteMessage
// logic that forwards messages between agents based on topic or capability matching.
