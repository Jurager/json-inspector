package wails

import (
	"context"
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

func (s *BridgeService) BridgePort() int {
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
func (s *BridgeService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	s.server.Start()
	return nil
}

// ServiceShutdown runs first of all services (registration order is reversed for shutdown): the
// socket goes away before anything it feeds.
func (s *BridgeService) ServiceShutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), bridgeShutdownTimeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}
