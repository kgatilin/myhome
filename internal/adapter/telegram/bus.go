package telegram

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
)

// BusMessage represents a message sent over the deskd bus.
type BusMessage struct {
	Type          string         `json:"type"`
	ID            string         `json:"id,omitempty"`
	Name          string         `json:"name,omitempty"`
	Source        string         `json:"source,omitempty"`
	Target        string         `json:"target,omitempty"`
	ReplyTo       string         `json:"reply_to,omitempty"`
	Payload       any            `json:"payload,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Subscriptions []string       `json:"subscriptions,omitempty"`
}

// BusClient sends and receives messages on the deskd bus over a Unix socket.
type BusClient struct {
	socketPath string
	conn       net.Conn
	replies    chan BusMessage
}

// NewBusClient creates a new bus client for the given socket path.
func NewBusClient(socketPath string) *BusClient {
	return &BusClient{
		socketPath: socketPath,
		replies:    make(chan BusMessage, 64),
	}
}

// Connect establishes the Unix socket connection and registers as the telegram adapter.
func (c *BusClient) Connect() error {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("connect to bus socket %s: %w", c.socketPath, err)
	}
	c.conn = conn

	reg := BusMessage{
		Type:          "register",
		Name:          "telegram-adapter",
		Subscriptions: []string{"telegram:*"},
	}
	if err := c.send(reg); err != nil {
		return err
	}

	// Read replies in background.
	go c.readLoop()

	return nil
}

// Publish sends a task message to the bus.
func (c *BusClient) Publish(msg BusMessage) error {
	if c.conn == nil {
		return fmt.Errorf("bus client not connected")
	}
	return c.send(msg)
}

// Replies returns the channel for incoming reply messages.
func (c *BusClient) Replies() <-chan BusMessage {
	return c.replies
}

// Close closes the bus connection.
func (c *BusClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *BusClient) send(msg BusMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal bus message: %w", err)
	}
	data = append(data, '\n')
	_, err = c.conn.Write(data)
	if err != nil {
		return fmt.Errorf("write to bus: %w", err)
	}
	return nil
}

func (c *BusClient) readLoop() {
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		var msg BusMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err == nil {
			c.replies <- msg
		}
	}
	close(c.replies)
}
