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
	ListTunnels(principal Principal) ([]TunnelInfo, error)
	ImportTunnel(principal Principal, name, configContent string) (*TunnelInfo, error)
	ImportEphemeralTunnel(principal Principal, name, configContent string) (*TunnelInfo, error)
	DeleteTunnel(principal Principal, id string) error
	ConnectTunnel(principal Principal, id string) error
	DisconnectTunnel(principal Principal, id string) error
	GetStats(principal Principal, id string) (*StatsInfo, error)
	DaemonStatus() (*StatusResult, error)
	SetEncryptionKey(principal Principal, key []byte) error
}

// Server listens on the IPC socket and dispatches RPC calls to a Handler.
type Server struct {
	handler  Handler
	listener net.Listener
	token    string // optional legacy session auth token

	mu      sync.RWMutex
	clients map[net.Conn]Principal
}

// NewServer creates a new IPC server backed by the given handler.
func NewServer(h Handler, token string) *Server {
	return &Server{
		handler: h,
		token:   token,
		clients: make(map[net.Conn]Principal),
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

// Broadcast pushes an event to GUI clients owned by the same OS user.
func (s *Server) Broadcast(evt Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	data = append(data, '\n')

	s.mu.RLock()
	defer s.mu.RUnlock()
	for c, principal := range s.clients {
		if !eventVisibleToPrincipal(evt, principal) {
			continue
		}
		_, _ = c.Write(data)
	}
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}
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

	principal, err := PeerPrincipal(conn)
	if err != nil || !principal.Valid() {
		log.Printf("ipc: rejected connection: unable to identify peer: %v", err)
		conn.Write([]byte("DENIED\n")) //nolint:errcheck
		return
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	firstLine := true
	registered := false
	register := func() {
		if registered {
			return
		}
		s.mu.Lock()
		s.clients[conn] = principal
		s.mu.Unlock()
		registered = true
	}
	for scanner.Scan() {
		line := scanner.Bytes()
		if firstLine {
			firstLine = false
			if handled, ok := s.handleOptionalAuthLine(conn, line); handled {
				if !ok {
					return
				}
				register()
				continue
			}
		}
		register()
		writeResponse(conn, s.dispatchLine(line, principal))
	}
}

func (s *Server) handleOptionalAuthLine(conn net.Conn, line []byte) (handled bool, ok bool) {
	text := strings.TrimSpace(string(line))
	if !strings.HasPrefix(text, "AUTH ") {
		return false, true
	}
	if s.token == "" {
		conn.Write([]byte("OK\n")) //nolint:errcheck
		return true, true
	}
	var presented string
	if _, err := fmt.Sscanf(text, "AUTH %s", &presented); err != nil ||
		subtle.ConstantTimeCompare([]byte(presented), []byte(s.token)) != 1 {
		log.Printf("ipc: rejected connection: bad auth token")
		conn.Write([]byte("DENIED\n")) //nolint:errcheck
		return true, false
	}
	conn.Write([]byte("OK\n")) //nolint:errcheck
	return true, true
}

func (s *Server) dispatchLine(line []byte, principal Principal) Response {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return Response{
			Error: &RPCError{Code: ErrCodeBadParams, Message: "invalid JSON"},
			ID:    0,
		}
	}
	return s.dispatch(req, principal)
}

func (s *Server) dispatch(req Request, principal Principal) Response {
	switch req.Method {
	case MethodListTunnels:
		tunnels, err := s.handler.ListTunnels(principal)
		if err != nil {
			return errResponse(req.ID, ErrCodeInternal, err.Error())
		}
		return okResponse(req.ID, tunnels)

	case MethodImportTunnel:
		var p ImportParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		t, err := s.handler.ImportTunnel(principal, p.Name, p.ConfigContent)
		if err != nil {
			return errResponse(req.ID, ErrCodeInternal, err.Error())
		}
		return okResponse(req.ID, t)

	case MethodImportEphemeral:
		var p ImportParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		t, err := s.handler.ImportEphemeralTunnel(principal, p.Name, p.ConfigContent)
		if err != nil {
			return errResponse(req.ID, ErrCodeInternal, err.Error())
		}
		return okResponse(req.ID, t)

	case MethodDeleteTunnel:
		var p TunnelIDParam
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		if err := s.handler.DeleteTunnel(principal, p.ID); err != nil {
			return errResponse(req.ID, ErrCodeTunnelError, err.Error())
		}
		return okResponse(req.ID, true)

	case MethodConnectTunnel:
		var p TunnelIDParam
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		if err := s.handler.ConnectTunnel(principal, p.ID); err != nil {
			return errResponse(req.ID, ErrCodeTunnelError, err.Error())
		}
		return okResponse(req.ID, true)

	case MethodDisconnect:
		var p TunnelIDParam
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		if err := s.handler.DisconnectTunnel(principal, p.ID); err != nil {
			return errResponse(req.ID, ErrCodeTunnelError, err.Error())
		}
		return okResponse(req.ID, true)

	case MethodGetStats:
		var p TunnelIDParam
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResponse(req.ID, ErrCodeBadParams, "bad params")
		}
		stats, err := s.handler.GetStats(principal, p.ID)
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
		if err := s.handler.SetEncryptionKey(principal, p.Key); err != nil {
			return errResponse(req.ID, ErrCodeInternal, err.Error())
		}
		return okResponse(req.ID, true)

	default:
		return errResponse(req.ID, -32601, fmt.Sprintf("unknown method %q", req.Method))
	}
}

func eventVisibleToPrincipal(evt Event, principal Principal) bool {
	if !principal.Valid() {
		return false
	}
	switch evt.Type {
	case EventTunnelChanged:
		var info TunnelInfo
		if err := json.Unmarshal(evt.Payload, &info); err != nil {
			return false
		}
		return ownerVisible(info.OwnerID, principal)
	case EventStatsUpdate:
		var info StatsInfo
		if err := json.Unmarshal(evt.Payload, &info); err != nil {
			return false
		}
		return ownerVisible(info.OwnerID, principal)
	default:
		return true
	}
}

func ownerVisible(ownerID string, principal Principal) bool {
	return ownerID == "" || ownerID == principal.UserID || (ownerID == LegacyOwnerID && !principal.IsLegacy())
}

func okResponse(id int, result interface{}) Response {
	data, _ := json.Marshal(result)
	return Response{Result: data, ID: id}
}

func errResponse(id, code int, msg string) Response {
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
