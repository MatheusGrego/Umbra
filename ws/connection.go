// Package ws — connection.go
// Connection manages a single WebSocket session: auth, read/write loops, reconnect.
// It dispatches inbound envelopes to a MessageHandler — it does NOT process them.
package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Envelope mirrors the server's wire format.
type Envelope struct {
	Type    string          `json:"type"`
	From    string          `json:"from,omitempty"`
	To      string          `json:"to,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// MessageHandler is called for every inbound envelope.
// Implementations must be non-blocking and thread-safe.
type MessageHandler interface {
	OnMessage(env Envelope)
}

// MessageHandlerFunc adapts a function to MessageHandler.
type MessageHandlerFunc func(env Envelope)

func (f MessageHandlerFunc) OnMessage(env Envelope) { f(env) }

// Connection wraps a gorilla WebSocket connection with auth + lifecycle management.
type Connection struct {
	serverURL string
	authFn    AuthFunc
	handler   MessageHandler

	mu     sync.Mutex
	conn   *websocket.Conn
	send   chan Envelope
	closed bool
}

// AuthFunc builds the auth envelope that the server expects on first connect.
type AuthFunc func() (Envelope, error)

const sendBufferSize = 128

// NewConnection constructs a Connection; call Connect() to establish the WS session.
func NewConnection(serverURL string, authFn AuthFunc, handler MessageHandler) *Connection {
	return &Connection{
		serverURL: serverURL,
		authFn:    authFn,
		handler:   handler,
		send:      make(chan Envelope, sendBufferSize),
	}
}

// Connect dials the server, sends auth, and starts I/O pumps.
// Blocks until the connection is authenticated or fails.
func (c *Connection) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(c.serverURL, nil)
	if err != nil {
		return fmt.Errorf("ws.Connect: dial: %w", err)
	}

	authEnv, err := c.authFn()
	if err != nil {
		conn.Close()
		return fmt.Errorf("ws.Connect: build auth: %w", err)
	}

	if err := conn.WriteJSON(authEnv); err != nil {
		conn.Close()
		return fmt.Errorf("ws.Connect: send auth: %w", err)
	}

	// Wait for auth_ok or auth_err.
	var resp Envelope
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err := conn.ReadJSON(&resp); err != nil {
		conn.Close()
		return fmt.Errorf("ws.Connect: read auth response: %w", err)
	}
	conn.SetReadDeadline(time.Time{})

	if resp.Type == "auth_err" {
		conn.Close()
		return fmt.Errorf("ws.Connect: auth rejected by server")
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	go c.writePump()
	go c.readPump()

	// Deliver auth_ok to handler so the app can update online-friends list.
	c.handler.OnMessage(resp)
	return nil
}

// Send enqueues an envelope for delivery. Safe for concurrent use.
func (c *Connection) Send(env Envelope) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("ws: connection closed")
	}
	c.mu.Unlock()

	select {
	case c.send <- env:
		return nil
	default:
		return fmt.Errorf("ws: send buffer full")
	}
}

// Close shuts down the connection gracefully.
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.send)
		if c.conn != nil {
			c.conn.Close()
		}
	}
}

func (c *Connection) writePump() {
	for env := range c.send {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			continue
		}
		if err := conn.WriteJSON(env); err != nil {
			log.Printf("[ws] write error: %v", err)
			return
		}
	}
}

func (c *Connection) readPump() {
	defer c.Close()
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}
		var env Envelope
		if err := conn.ReadJSON(&env); err != nil {
			log.Printf("[ws] read error: %v", err)
			return
		}
		c.handler.OnMessage(env)
	}
}

// MustMarshal is a panic-on-error json.Marshal for guaranteed-serialisable values.
func MustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
