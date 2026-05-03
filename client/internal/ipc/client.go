package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

// Client connects to the daemon IPC socket and sends RPC calls.
type Client struct {
	mu   sync.Mutex
	conn net.Conn

	pendingMu sync.Mutex
	pending   map[int]chan *Response

	nextID  atomic.Int32
	eventCh chan Event
}

// NewClient creates a new IPC client (not yet connected).
func NewClient() *Client {
	return &Client{
		eventCh: make(chan Event, 64),
	}
}

// Connect dials the daemon socket and performs the token handshake.
// Safe to call multiple times.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	conn, err := Dial()
	if err != nil {
		return fmt.Errorf("connect to daemon: %w", err)
	}

	// Send auth token if one is present on disk.
	if token, err := ReadToken(); err == nil && token != "" {
		if _, err := fmt.Fprintf(conn, "AUTH %s\n", token); err != nil {
			conn.Close()
			return fmt.Errorf("send auth token: %w", err)
		}
		// Read server OK / DENIED.
		buf := make([]byte, 8)
		n, _ := conn.Read(buf)
		if string(buf[:n]) != "OK\n" {
			conn.Close()
			return fmt.Errorf("daemon rejected auth token")
		}
	}

	c.conn = conn
	c.pending = make(map[int]chan *Response)
	go c.readerLoop(conn)
	return nil
}

// Close disconnects from the daemon.
func (c *Client) Close() {
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
}

// IsConnected reports whether the client has an active connection.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// Events returns the channel on which server-pushed events are delivered.
func (c *Client) Events() <-chan Event { return c.eventCh }

// --- RPC methods ---

func (c *Client) ListTunnels() ([]TunnelInfo, error) {
	var result []TunnelInfo
	return result, c.call(MethodListTunnels, nil, &result)
}

func (c *Client) ImportTunnel(name, configContent string) (*TunnelInfo, error) {
	var result TunnelInfo
	return &result, c.call(MethodImportTunnel, ImportParams{Name: name, ConfigContent: configContent}, &result)
}

// ImportEphemeralTunnel imports a tunnel without saving it to disk.
// Used for managed VPN sessions that must not survive a daemon restart.
func (c *Client) ImportEphemeralTunnel(name, configContent string) (*TunnelInfo, error) {
	var result TunnelInfo
	return &result, c.call(MethodImportEphemeral, ImportParams{Name: name, ConfigContent: configContent}, &result)
}

func (c *Client) DeleteTunnel(id string) error {
	return c.call(MethodDeleteTunnel, TunnelIDParam{ID: id}, nil)
}

func (c *Client) ConnectTunnel(id string) error {
	return c.call(MethodConnectTunnel, TunnelIDParam{ID: id}, nil)
}

func (c *Client) DisconnectTunnel(id string) error {
	return c.call(MethodDisconnect, TunnelIDParam{ID: id}, nil)
}

func (c *Client) GetStats(id string) (*StatsInfo, error) {
	var result StatsInfo
	return &result, c.call(MethodGetStats, TunnelIDParam{ID: id}, &result)
}

func (c *Client) DaemonStatus() (*StatusResult, error) {
	var result StatusResult
	return &result, c.call(MethodDaemonStatus, nil, &result)
}

func (c *Client) SetEncryptionKey(key []byte) error {
	return c.call(MethodSetEncryptionKey, SetEncryptionKeyParams{Key: key}, nil)
}

// --- internal ---

// readerLoop is the single goroutine that reads all incoming data.
// It dispatches responses to pending call channels and events to eventCh.
func (c *Client) readerLoop(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		line := scanner.Bytes()

		// Peek at the message type
		var peek struct {
			Type string `json:"type"`
			ID   *int   `json:"id"`
		}
		if err := json.Unmarshal(line, &peek); err != nil {
			continue
		}

		if peek.Type != "" {
			// Server-pushed event
			var evt Event
			if err := json.Unmarshal(line, &evt); err == nil {
				select {
				case c.eventCh <- evt:
				default:
				}
			}
			continue
		}

		if peek.ID != nil {
			// Response to a pending call
			var resp Response
			if err := json.Unmarshal(line, &resp); err != nil {
				continue
			}
			c.pendingMu.Lock()
			ch, ok := c.pending[*peek.ID]
			if ok {
				delete(c.pending, *peek.ID)
			}
			c.pendingMu.Unlock()

			if ok {
				ch <- &resp
			}
		}
	}

	// Connection dropped — fail all pending calls
	c.mu.Lock()
	c.conn = nil
	c.mu.Unlock()

	c.pendingMu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()
}

func (c *Client) call(method string, params, result interface{}) error {
	id := int(c.nextID.Add(1))
	ch := make(chan *Response, 1)

	c.mu.Lock()
	if c.conn == nil {
		c.mu.Unlock()
		return fmt.Errorf("not connected to daemon")
	}

	// Register before writing to avoid missing a fast response
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	req := Request{Method: method, ID: id}
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			c.mu.Unlock()
			c.removePending(id)
			return err
		}
		req.Params = data
	}
	data, err := json.Marshal(req)
	if err != nil {
		c.mu.Unlock()
		c.removePending(id)
		return err
	}
	data = append(data, '\n')
	_, err = c.conn.Write(data)
	c.mu.Unlock()

	if err != nil {
		c.removePending(id)
		return fmt.Errorf("write request: %w", err)
	}

	// Wait for response from the single reader goroutine
	resp, ok := <-ch
	if !ok {
		return fmt.Errorf("connection closed while waiting for response")
	}
	if resp.Error != nil {
		return resp.Error
	}
	if result != nil && resp.Result != nil {
		return json.Unmarshal(resp.Result, result)
	}
	return nil
}

func (c *Client) removePending(id int) {
	c.pendingMu.Lock()
	delete(c.pending, id)
	c.pendingMu.Unlock()
}
