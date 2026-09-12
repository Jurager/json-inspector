// Package bridge is the local WebSocket endpoint the Chrome extension talks to. It owns the wire
// protocol and the connection bookkeeping; what the app does with what arrives is an Ingest.
package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// Port is the fixed local port the extension probes; it is a named type so it cannot be confused
// with any other number travelling through the dependency graph.
type Port int

type Server struct {
	ingest   Ingest
	port     Port
	upgrader websocket.Upgrader
	http     *http.Server

	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func NewServer(ingest Ingest, port Port) *Server {
	if ingest == nil {
		ingest = nopIngest{}
	}
	s := &Server{
		ingest:  ingest,
		port:    port,
		clients: make(map[*websocket.Conn]struct{}),
	}
	s.upgrader = websocket.Upgrader{CheckOrigin: s.checkOrigin}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", s.handleWS)
	s.http = &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}
	return s
}

func (s *Server) Port() int {
	return int(s.port)
}

// Start begins listening and returns at once — the caller is the app's startup path, which must not
// block on a socket.
func (s *Server) Start() {
	log.Printf("[bridge] listening on ws://%s", s.http.Addr)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[bridge] error: %v", err)
		}
	}()
}

// Shutdown stops accepting connections and closes the ones the extension holds, so a quit does not
// leave the extension reconnecting to a dead port.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	for c := range s.clients {
		_ = c.Close()
	}
	s.clients = make(map[*websocket.Conn]struct{})
	s.mu.Unlock()
	return s.http.Shutdown(ctx)
}

func (s *Server) Broadcast(payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		_ = c.WriteMessage(websocket.TextMessage, payload)
	}
}

func (s *Server) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if strings.HasPrefix(origin, "chrome-extension://") {
		return true
	}
	if strings.HasPrefix(origin, "http://localhost") ||
		strings.HasPrefix(origin, "http://127.0.0.1") {
		return true
	}
	return false
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
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"name":    "json-inspector",
		"version": "0.1.0",
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
		// Only the last one going away means capture is really off: with several tabs the
		// extension holds one connection per worker, and an earlier one closing says nothing.
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
