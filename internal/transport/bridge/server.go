// Package bridge is the local WebSocket endpoint used by the Chrome extension.
// It handles the wire protocol and connection lifecycle; Ingest handles received data.
package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"

	"json-inspector/internal/platform"
)

// Port is the local port used by the extension.
type Port int

type Server struct {
	ingest   Ingest
	port     Port
	build    platform.BuildInfo
	upgrader websocket.Upgrader
	http     *http.Server

	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func NewServer(ingest Ingest, port Port, build platform.BuildInfo) *Server {
	if ingest == nil {
		ingest = nopIngest{}
	}

	s := &Server{
		ingest:  ingest,
		port:    port,
		build:   build,
		clients: make(map[*websocket.Conn]struct{}),
	}

	s.upgrader = websocket.Upgrader{CheckOrigin: s.checkOrigin}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", s.handleWS)

	s.http = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	return s
}

func (s *Server) Port() int {
	return int(s.port)
}

// Start binds the port and serves on it without blocking the caller. The listen happens here rather
// than in the goroutine because a taken port is a fact the caller has to hear: the window shows the
// address it is listening on, and an address nothing owns is a worse answer than a failure.
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.http.Addr, err)
	}
	log.Printf("[bridge] listening on ws://%s", s.http.Addr)

	go func() {
		if err := s.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[bridge] error: %v", err)
		}
	}()
	return nil
}

// Shutdown closes active connections and stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	for c := range s.clients {
		_ = c.Close()
	}
	s.clients = make(map[*websocket.Conn]struct{})
	s.mu.Unlock()

	return s.http.Shutdown(ctx)
}

// Broadcast tells every connected window what just happened. The writes happen outside the lock:
// a socket whose peer has stopped reading blocks until the TCP window fills, and holding the lock
// across that would stop every other client — and Shutdown, which needs the same lock — until it
// drained. Sending is best-effort either way, so a client that went away is skipped.
func (s *Server) Broadcast(payload []byte) {
	s.mu.Lock()
	clients := make([]*websocket.Conn, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	for _, c := range clients {
		_ = c.WriteMessage(websocket.TextMessage, payload)
	}
}

// checkOrigin decides which pages may open a socket. A browser sends the origin of the page that
// asked, so this is what stands between a page on the web and the app's capture: the host is parsed
// rather than matched as a prefix, because `http://localhost.evil.com` is not this machine.
func (s *Server) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if strings.HasPrefix(origin, "chrome-extension://") {
		return true
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	return host == "localhost" || net.ParseIP(host).IsLoopback()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Private-Network", "true")

	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// The version is the running build's, not a constant: the extension probes this endpoint to
	// decide whether the app is up, and a version nobody can act on is worse than none.
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"name":    "json-inspector",
		"version": s.build.Version,
	})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.mu.Lock()
	s.clients[conn] = struct{}{}
	s.mu.Unlock()

	defer conn.Close()
	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		remaining := len(s.clients)
		s.mu.Unlock()

		if remaining == 0 {
			s.ingest.ClientGone()
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var envelope struct {
			Type string `json:"type"`
		}

		if err := json.Unmarshal(msg, &envelope); err != nil {
			continue
		}

		switch envelope.Type {
		case "request":
			var req CapturedRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				continue
			}
			s.ingest.Captured(req)

		case "state":
			var st CaptureState
			if err := json.Unmarshal(msg, &st); err != nil {
				continue
			}
			s.ingest.StateChanged(st)

		case "focus":
			var fr FocusRequest
			if err := json.Unmarshal(msg, &fr); err != nil {
				continue
			}
			s.ingest.FocusRequested(fr)
		}
	}
}
