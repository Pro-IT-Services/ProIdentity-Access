package ipc

import (
	"bufio"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
)

// Handler is implemented by the daemon to handle RPC calls.
type Handler interface {
	ListTunnels() ([]TunnelInfo, error)
	ImportTunnel(name, configContent string) (*TunnelInfo, error)
	ImportEphemeralTunnel(name, configContent string) (*TunnelInfo, error)
	DeleteTunnel(id string) error
	ConnectTunnel(id string) error
	DisconnectTunnel(id string) error
	GetStats(id string) (*StatsInfo, error)
	DaemonStatus() (*StatusResult, error)
	SetEncryptionKey(key []byte) error
}

// Server listens on the IPC socket and dispatches RPC calls to a Handler.
type Server struct {
	handler  Handler
	listener net.Listener
	token    string // session auth token; empty = auth disabled

	mu      sync.RWMutex
	clients map[net.Conn]struct{}
}

// NewServer creates a new IPC server backed by the given handler.
// token is the session secret that every client must present immediately
// after connecting. Pass an empty string to disable auth (testing only).
func NewServer(h Handler, token string) *Server {
	return &Server{
		handler: h,
		token:   token,
		clients: make(map[net.Conn]struct{}),
	}
}

// Start begins listening and serving connections.
func (s *Server) Start() error {
	l, err := Listen()
	if err != nil {
		return fmt.Errorf("ipc listen: %w", err)
	}
	s.listener = l
	log.Printf("IPC server listening on %s", SocketPath())
	go s.acceptLoop()
	return nil
}

// Stop closes the listener and all active connections.
func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		c.Close()
	}
}

// Broadcast pushes an event to all connected GUI clients.
func (s *Server) Broadcast(evt Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	data = append(data, '\n')

	s.mu.RLock()
	defer s.mu.RUnlock()
	for c := range s.clients {
		_, _ = c.Write(data)
	}
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}
		s.mu.Lock()
		s.clients[conn] = struct{}{}
		s.mu.Unlock()

		go s.serveConn(conn)
	}
}

func (s *Server) serveConn(conn net.Conn) {
	defer func() {
		conn.Close()
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
	}()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	// Token handshake: first line must be "AUTH <token>".
	// If auth is enabled and the token doesn't match, close immediately.
	if s.token != "" {
		if !scanner.Scan() {
			return
		}
		line := strings.TrimSpace(scanner.Text())
		var presented string
		if _, err := fmt.Sscanf(line, "AUTH %s", &presented); err != nil || subtle.ConstantTimeCompare([]byte(presented), []byte(s.token)) != 1 {
			log.Printf("ipc: rejected connection — bad auth token")
			conn.Write([]byte("DENIED\n")) //nolint:errcheck
			return
		}
		conn.Write([]byte("OK\n")) //nolint:errcheck
	}

	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			writeResponse(conn, Response{
				Error: &RPCError{Code: ErrCodeBadParams, Message: "invalid JSON"},
				ID:    0,
			})
			continue
		}
		resp := s.dispatch(req)
		writeResponse(conn, resp)
	}
}

func (s *Server) dispatch(req Request) Response {
	switch req.Method {
	case MethodListTunnels:
		tunnels, err := s.handler.ListTunnels()
		if err != nil {
			return errResponse(req.ID, ErrCodeInternal, err.Error())
		}
		return okResponse(req.ID, tunnels)

	case MethodImportTunnel:
		var p ImportParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		t, err := s.handler.ImportTunnel(p.Name, p.ConfigContent)
		if err != nil {
			return errResponse(req.ID, ErrCodeInternal, err.Error())
		}
		return okResponse(req.ID, t)

	case MethodImportEphemeral:
		var p ImportParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		t, err := s.handler.ImportEphemeralTunnel(p.Name, p.ConfigContent)
		if err != nil {
			return errResponse(req.ID, ErrCodeInternal, err.Error())
		}
		return okResponse(req.ID, t)

	case MethodDeleteTunnel:
		var p TunnelIDParam
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		if err := s.handler.DeleteTunnel(p.ID); err != nil {
			return errResponse(req.ID, ErrCodeTunnelError, err.Error())
		}
		return okResponse(req.ID, true)

	case MethodConnectTunnel:
		var p TunnelIDParam
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		if err := s.handler.ConnectTunnel(p.ID); err != nil {
			return errResponse(req.ID, ErrCodeTunnelError, err.Error())
		}
		return okResponse(req.ID, true)

	case MethodDisconnect:
		var p TunnelIDParam
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		if err := s.handler.DisconnectTunnel(p.ID); err != nil {
			return errResponse(req.ID, ErrCodeTunnelError, err.Error())
		}
		return okResponse(req.ID, true)

	case MethodGetStats:
		var p TunnelIDParam
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		stats, err := s.handler.GetStats(p.ID)
		if err != nil {
			return errResponse(req.ID, ErrCodeTunnelError, err.Error())
		}
		return okResponse(req.ID, stats)

	case MethodDaemonStatus:
		status, err := s.handler.DaemonStatus()
		if err != nil {
			return errResponse(req.ID, ErrCodeInternal, err.Error())
		}
		return okResponse(req.ID, status)

	case MethodSetEncryptionKey:
		var p SetEncryptionKeyParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		if err := s.handler.SetEncryptionKey(p.Key); err != nil {
			return errResponse(req.ID, ErrCodeInternal, err.Error())
		}
		return okResponse(req.ID, true)

	default:
		return errResponse(req.ID, -32601, fmt.Sprintf("unknown method %q", req.Method))
	}
}

func okResponse(id int, result interface{}) Response {
	data, _ := json.Marshal(result)
	return Response{Result: data, ID: id}
}

func errResponse(id, code int, msg string) Response {
	// Sanitize message — strip internal path info
	msg = strings.TrimSpace(msg)
	return Response{Error: &RPCError{Code: code, Message: msg}, ID: id}
}

func writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_, _ = conn.Write(data)
}
