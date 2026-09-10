package bridge

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// Handler is called for every validated request received over the WebSocket.
type Handler func(CapturedRequest)

// Server is a loopback WebSocket server that receives captured requests from
// the Chrome extension and forwards them to the UI.
type Server struct {
	port     int
	handler  Handler
	upgrader websocket.Upgrader
}

// NewServer creates a server listening on 127.0.0.1:port.
func NewServer(port int, handler Handler) *Server {
	s := &Server{port: port, handler: handler}
	s.upgrader = websocket.Upgrader{CheckOrigin: s.checkOrigin}
	return s
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
	defer conn.Close()
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var req CapturedRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}
		if req.Type != "request" {
			continue
		}
		if s.handler != nil {
			s.handler(req)
		}
	}
}
