package github

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
)

func TestBusClient_ConnectAndPublish(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")

	// Start a fake bus server.
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	received := make(chan BusMessage, 2)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			var msg BusMessage
			if err := json.Unmarshal(scanner.Bytes(), &msg); err == nil {
				received <- msg
			}
		}
	}()

	client := NewBusClient(socketPath)
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer client.Close()

	// Should have received the register message.
	reg := <-received
	if reg.Type != "register" {
		t.Errorf("expected register message, got %q", reg.Type)
	}
	if reg.Name != "github-adapter" {
		t.Errorf("expected name 'github-adapter', got %q", reg.Name)
	}

	// Publish a message.
	msg := BusMessage{
		Type:    "message",
		ID:      "test-123",
		Source:  "github:kgatilin/home",
		Target:  "agent:dev",
		Payload: "Issue #1: test",
	}
	if err := client.Publish(msg); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	pub := <-received
	if pub.Type != "message" {
		t.Errorf("expected message type, got %q", pub.Type)
	}
	if pub.Payload != "Issue #1: test" {
		t.Errorf("expected payload 'Issue #1: test', got %q", pub.Payload)
	}
}

func TestBusClient_ConnectFailure(t *testing.T) {
	client := NewBusClient("/nonexistent/socket.sock")
	if err := client.Connect(); err == nil {
		t.Error("expected error connecting to nonexistent socket")
	}
}

func TestBusClient_PublishNotConnected(t *testing.T) {
	client := NewBusClient("/tmp/fake.sock")
	err := client.Publish(BusMessage{Type: "message"})
	if err == nil {
		t.Error("expected error publishing without connection")
	}
}

func TestBusClient_CloseNilConn(t *testing.T) {
	client := NewBusClient("/tmp/fake.sock")
	if err := client.Close(); err != nil {
		t.Errorf("Close() on nil conn should not error: %v", err)
	}
}

func TestBusMessage_JSON(t *testing.T) {
	msg := BusMessage{
		Type:    "message",
		ID:      "github-kgatilin/home-42-12345",
		Source:  "github:kgatilin/home",
		Target:  "agent:dev",
		Payload: "Issue #42: Fix bug\n\nSome body text",
		Metadata: map[string]any{
			"priority": 5,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded BusMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Type != "message" || decoded.Source != "github:kgatilin/home" {
		t.Errorf("round-trip failed: %+v", decoded)
	}

}
