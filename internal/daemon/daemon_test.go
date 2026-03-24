package daemon

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kgatilin/myhome/internal/agent"
)

func TestDaemonPingPong(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")

	// Create a minimal agent store
	agentStore, err := agent.NewStore(filepath.Join(dir, "agents"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	d := &Daemon{
		socketPath: socketPath,
		handler:    handler{store: agentStore},
		stopCh:     make(chan struct{}),
	}

	// Start listener
	os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	d.listener = listener

	// Handle one connection
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		d.handleConnection(conn)
	}()

	// Connect and send ping
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	req := Request{Method: "ping"}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var resp Response
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}

	var result string
	json.Unmarshal(resp.Result, &result)
	if result != "pong" {
		t.Errorf("result = %q, want %q", result, "pong")
	}

	listener.Close()
}

func TestDaemonListEmpty(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")

	agentStore, err := agent.NewStore(filepath.Join(dir, "agents"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	d := &Daemon{
		socketPath: socketPath,
		handler:    handler{store: agentStore},
		stopCh:     make(chan struct{}),
	}

	os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	d.listener = listener

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		d.handleConnection(conn)
	}()

	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	req := Request{Method: "list"}
	json.NewEncoder(conn).Encode(req)

	var resp Response
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	json.NewDecoder(conn).Decode(&resp)

	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}

	listener.Close()
}

func TestIsRunningFalse(t *testing.T) {
	if IsRunning("/tmp/nonexistent-myhome-test.sock") {
		t.Error("IsRunning should be false for nonexistent socket")
	}
}

func TestSocketPath(t *testing.T) {
	path := SocketPath("/home/testuser")
	expected := "/home/testuser/.myhome/myhome.sock"
	if path != expected {
		t.Errorf("SocketPath = %q, want %q", path, expected)
	}
}
