package daemon

import (
	"encoding/json"
	"fmt"

	"github.com/kgatilin/myhome/internal/agent"
	"github.com/kgatilin/myhome/internal/config"
)

type nameParam struct {
	Name string `json:"name"`
}

type sendParam struct {
	Name     string `json:"name"`
	Message  string `json:"message"`
	MaxTurns int    `json:"max_turns,omitempty"`
}

// handler processes daemon RPC requests using the manager and store.
type handler struct {
	manager agentManager
	store   agentStore
	agents  map[string]config.AgentConfig
}

func (h *handler) handleCreate(params json.RawMessage) Response {
	var p nameParam
	if err := json.Unmarshal(params, &p); err != nil {
		return Response{Error: fmt.Sprintf("invalid params: %v", err)}
	}
	agentCfg, ok := h.agents[p.Name]
	if !ok {
		return Response{Error: fmt.Sprintf("unknown agent %q in config", p.Name)}
	}
	fullCfg := &config.Config{InfraConfig: config.InfraConfig{Agents: h.agents}}
	if err := h.manager.Create(p.Name, agentCfg, fullCfg); err != nil {
		return Response{Error: err.Error()}
	}
	state, _ := h.store.Load(p.Name)
	return jsonResult(state)
}

func (h *handler) handleList() Response {
	states, err := h.store.List()
	if err != nil {
		return Response{Error: err.Error()}
	}
	for _, s := range states {
		h.manager.RefreshStatus(s.Name)
	}
	states, _ = h.store.List()
	return jsonResult(states)
}

func (h *handler) handleSend(params json.RawMessage) Response {
	var p sendParam
	if err := json.Unmarshal(params, &p); err != nil {
		return Response{Error: fmt.Sprintf("invalid params: %v", err)}
	}
	var opts *agent.SendOptions
	if p.MaxTurns > 0 {
		opts = &agent.SendOptions{MaxTurns: p.MaxTurns}
	}
	response, err := h.manager.Send(p.Name, p.Message, opts)
	if err != nil {
		return Response{Error: err.Error()}
	}
	return jsonResult(response)
}

func (h *handler) handleStop(params json.RawMessage) Response {
	var p nameParam
	if err := json.Unmarshal(params, &p); err != nil {
		return Response{Error: fmt.Sprintf("invalid params: %v", err)}
	}
	if err := h.manager.Stop(p.Name); err != nil {
		return Response{Error: err.Error()}
	}
	return jsonResult("stopped")
}

func (h *handler) handleRestart(params json.RawMessage) Response {
	var p nameParam
	if err := json.Unmarshal(params, &p); err != nil {
		return Response{Error: fmt.Sprintf("invalid params: %v", err)}
	}
	agentCfg, ok := h.agents[p.Name]
	if !ok {
		return Response{Error: fmt.Sprintf("unknown agent %q in config", p.Name)}
	}
	fullCfg := &config.Config{InfraConfig: config.InfraConfig{Agents: h.agents}}
	if err := h.manager.Restart(p.Name, agentCfg, fullCfg); err != nil {
		return Response{Error: err.Error()}
	}
	return jsonResult("restarted")
}

func (h *handler) handleRemove(params json.RawMessage) Response {
	var p nameParam
	if err := json.Unmarshal(params, &p); err != nil {
		return Response{Error: fmt.Sprintf("invalid params: %v", err)}
	}
	if err := h.manager.Remove(p.Name); err != nil {
		return Response{Error: err.Error()}
	}
	return jsonResult("removed")
}

func (h *handler) handleStatus(params json.RawMessage) Response {
	var p nameParam
	if err := json.Unmarshal(params, &p); err != nil {
		return Response{Error: fmt.Sprintf("invalid params: %v", err)}
	}
	state, err := h.manager.RefreshStatus(p.Name)
	if err != nil {
		return Response{Error: err.Error()}
	}
	return jsonResult(state)
}
