// Package daemon provides a long-running supervisor process that manages agent lifecycles
// and exposes a JSON-RPC API on a unix socket for CLI interaction.
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/kgatilin/myhome/internal/agent"
	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/container"
)

// agentManager abstracts agent lifecycle operations.
type agentManager interface {
	Create(name string, agentCfg config.AgentConfig, cfg *config.Config) error
	Stop(name string) error
	Restart(name string, agentCfg config.AgentConfig, cfg *config.Config) error
	Remove(name string) error
	Send(name, message string) (string, error)
	RefreshStatus(name string) (*agent.State, error)
}

// agentStore abstracts agent state persistence.
type agentStore interface {
	Load(name string) (*agent.State, error)
	List() ([]*agent.State, error)
}

// Daemon is the long-running supervisor that manages agents.
type Daemon struct {
	socketPath string
	handler    handler
	listener   net.Listener
	mu         sync.Mutex
	stopCh     chan struct{}
}

// Config holds daemon configuration.
type Config struct {
	SocketPath string // defaults to ~/.myhome/myhome.sock
	HomeDir    string
	ExecFn     agent.ExecFunc
}

// New creates a new Daemon instance.
func New(cfg Config) (*Daemon, error) {
	if cfg.SocketPath == "" {
		cfg.SocketPath = filepath.Join(cfg.HomeDir, ".myhome", "myhome.sock")
	}

	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil, fmt.Errorf("finding config: %w", err)
	}
	myhomeCfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	runtime, err := container.DetectRuntime(myhomeCfg.ContainerRuntime)
	if err != nil {
		return nil, fmt.Errorf("detecting container runtime: %w", err)
	}

	agentStore, err := agent.NewStore(filepath.Join(cfg.HomeDir, ".myhome", "agents"))
	if err != nil {
		return nil, fmt.Errorf("creating agent store: %w", err)
	}

	manager := agent.NewManager(agentStore, cfg.ExecFn, runtime, cfg.HomeDir)

	return &Daemon{
		socketPath: cfg.SocketPath,
		handler: handler{
			manager: manager,
			store:   agentStore,
			agents:  myhomeCfg.Agents,
		},
		stopCh: make(chan struct{}),
	}, nil
}

// Run starts the daemon, listens on the unix socket, and blocks until stopped.
func (d *Daemon) Run() error {
	if err := os.MkdirAll(filepath.Dir(d.socketPath), 0o755); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}
	os.Remove(d.socketPath)

	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", d.socketPath, err)
	}
	d.listener = listener

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
			d.listener.Close()
		case <-d.stopCh:
			cancel()
			d.listener.Close()
		}
	}()

	go d.healthCheckLoop(ctx)

	fmt.Printf("Daemon listening on %s\n", d.socketPath)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("accepting connection: %w", err)
			}
		}
		go d.handleConnection(conn)
	}
}

// Stop signals the daemon to shut down.
func (d *Daemon) Stop() {
	close(d.stopCh)
	if d.listener != nil {
		d.listener.Close()
	}
}

// Request represents a JSON-RPC-style request from the CLI.
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC-style response to the CLI.
type Response struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (d *Daemon) handleConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Minute))

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var req Request
	if err := decoder.Decode(&req); err != nil {
		encoder.Encode(Response{Error: fmt.Sprintf("invalid request: %v", err)})
		return
	}

	resp := d.dispatch(req)
	encoder.Encode(resp)
}

func (d *Daemon) dispatch(req Request) Response {
	d.mu.Lock()
	defer d.mu.Unlock()

	h := &d.handler
	switch req.Method {
	case "create":
		return h.handleCreate(req.Params)
	case "list":
		return h.handleList()
	case "send":
		return h.handleSend(req.Params)
	case "stop":
		return h.handleStop(req.Params)
	case "restart":
		return h.handleRestart(req.Params)
	case "remove":
		return h.handleRemove(req.Params)
	case "status":
		return h.handleStatus(req.Params)
	case "ping":
		return jsonResult("pong")
	default:
		return Response{Error: fmt.Sprintf("unknown method: %s", req.Method)}
	}
}

// healthCheckLoop periodically checks agent containers and updates state.
func (d *Daemon) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.mu.Lock()
			states, err := d.handler.store.List()
			if err == nil {
				for _, s := range states {
					if s.Status == agent.StatusRunning {
						d.handler.manager.RefreshStatus(s.Name)
					}
				}
			}
			d.mu.Unlock()
		}
	}
}

func jsonResult(v any) Response {
	data, err := json.Marshal(v)
	if err != nil {
		return Response{Error: fmt.Sprintf("marshal error: %v", err)}
	}
	return Response{Result: data}
}

// SocketPath returns the default socket path for the daemon.
func SocketPath(homeDir string) string {
	return filepath.Join(homeDir, ".myhome", "myhome.sock")
}

// IsRunning checks if the daemon is reachable on its unix socket.
func IsRunning(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)
	encoder.Encode(Request{Method: "ping"})

	var resp Response
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err := decoder.Decode(&resp); err != nil {
		return false
	}
	return resp.Error == ""
}

// Call sends a request to the daemon via unix socket and returns the response.
func Call(socketPath string, method string, params any) (*Response, error) {
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connecting to daemon: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Minute))

	var paramsJSON json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshaling params: %w", err)
		}
		paramsJSON = data
	}

	req := Request{Method: method, Params: paramsJSON}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	return &resp, nil
}
