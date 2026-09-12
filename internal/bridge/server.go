package bridge

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type Handler func(CapturedRequest)

type StateHandler func(CaptureState)

type DisconnectHandler func()

type FocusHandler func(FocusRequest)

// What the app answers for; an unset callback is a no-op, so a caller only names the ones
// it cares about instead of passing four positional functions of different types.
type Handlers struct {
	Request    Handler
	State      StateHandler
	Disconnect DisconnectHandler
	Focus      FocusHandler
}

type Server struct {
	port     int
	handlers Handlers
	upgrader websocket.Upgrader

	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func NewServer(port int, handlers Handlers) *Server {
	if handlers.Request == nil {
		handlers.Request = func(CapturedRequest) {}
	}
	if handlers.State == nil {
		handlers.State = func(CaptureState) {}
	}
	if handlers.Disconnect == nil {
		handlers.Disconnect = func() {}
	}
	if handlers.Focus == nil {
		handlers.Focus = func(FocusRequest) {}
	}
	s := &Server{
		port:     port,
		handlers: handlers,
		clients:  make(map[*websocket.Conn]struct{}),
	}
	s.upgrader = websocket.Upgrader{CheckOrigin: s.checkOrigin}
	return s
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

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", s.handleWS)
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	log.Printf("[bridge] listening on ws://%s", addr)
	return http.ListenAndServe(addr, mux)
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
		s.mu.Unlock()
		s.handlers.Disconnect()
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
			s.handlers.Request(req)
		case "state":
			var st CaptureState
			if err := json.Unmarshal(msg, &st); err != nil {
				continue
			}
			s.handlers.State(st)
		case "focus":
			var fr FocusRequest
			if err := json.Unmarshal(msg, &fr); err != nil {
				continue
			}
			s.handlers.Focus(fr)
		}
	}
}
