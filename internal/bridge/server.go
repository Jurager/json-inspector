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

// Handler is called for every validated request received over the WebSocket.
type Handler func(CapturedRequest)

// StateHandler is called for every capture-state message from the extension.
type StateHandler func(CaptureState)

// DisconnectHandler is called when the extension's WebSocket drops, so the UI
// can stop claiming the extension is still connected.
type DisconnectHandler func()

// Server is a loopback WebSocket server that receives captured requests from
// the Chrome extension and forwards them to the UI. It also keeps the live
// socket so the app can push control messages back to the extension (pause).
type Server struct {
	port              int
	handler           Handler
	stateHandler      StateHandler
	disconnectHandler DisconnectHandler
	upgrader          websocket.Upgrader

	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

// NewServer creates a server listening on 127.0.0.1:port.
func NewServer(port int, handler Handler, stateHandler StateHandler, disconnectHandler DisconnectHandler) *Server {
	s := &Server{
		port:              port,
		handler:           handler,
		stateHandler:      stateHandler,
		disconnectHandler: disconnectHandler,
		clients:           make(map[*websocket.Conn]struct{}),
	}
	s.upgrader = websocket.Upgrader{CheckOrigin: s.checkOrigin}
	return s
}

// Broadcast sends a message to every currently connected extension. It is the
// app → extension direction (the opposite of the requests the extension pushes).
func (s *Server) Broadcast(payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		_ = c.WriteMessage(websocket.TextMessage, payload)
	}
}

// checkOrigin limits connections to trusted origins: non-browser clients
// (empty Origin), the extension, and the app's own webview/dev origins. This
// prevents arbitrary websites from opening a socket to localhost and either
// reading or injecting requests.
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

// Start begins serving and blocks until the server fails.
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
	// Chrome's Private Network Access check can preflight a request from an
	// extension page to a loopback address; without this header the extension's
	// availability probe fails even while the app is running, and the popup
	// tells the user to launch something that is already up.
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
		if s.disconnectHandler != nil {
			s.disconnectHandler()
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
			if s.handler != nil {
				s.handler(req)
			}
		case "state":
			var st CaptureState
			if err := json.Unmarshal(msg, &st); err != nil {
				continue
			}
			if s.stateHandler != nil {
				s.stateHandler(st)
			}
		}
	}
}
