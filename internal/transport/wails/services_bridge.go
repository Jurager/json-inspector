package wails

import (
	"context"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/transport/bridge"
)

// How long the socket gets to close before quitting carries on without it.
const bridgeShutdownTimeout = 2 * time.Second

// BridgeService is the extension endpoint as the app sees it: the port the empty state shows and
// the pause switch the capture bar uses. What arrives on that socket is handled by an ingest, not
// by a bound method — see ingest.go.
type BridgeService struct {
	server *bridge.Server
}

func NewBridgeService(server *bridge.Server) *BridgeService {
	return &BridgeService{server: server}
}

// Port is the port the extension is told to connect to. The service is the bridge already, so the
// name does not repeat it.
func (s *BridgeService) Port() int {
	return s.server.Port()
}

func (s *BridgeService) PauseCapture() {
	s.server.Broadcast([]byte(`{"type":"pause"}`))
}

func (s *BridgeService) ResumeCapture() {
	s.server.Broadcast([]byte(`{"type":"resume"}`))
}

// ServiceStartup starts listening before the window appears — the extension connects whenever it
// notices, and a socket that opens a second later is not worth a visible ordering rule.
//
// A port somebody else holds is logged and not returned: capture is a feature of the app, not a
// condition for it, and a service that fails to start takes the window down with it.
func (s *BridgeService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	if err := s.server.Start(); err != nil {
		log.Printf("[bridge] %v — capture is off", err)
	}
	return nil
}

// ServiceShutdown runs first of all services (registration order is reversed for shutdown): the
// socket goes away before anything it feeds.
func (s *BridgeService) ServiceShutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), bridgeShutdownTimeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}
