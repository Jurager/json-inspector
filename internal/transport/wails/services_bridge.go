package wails

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
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

// Port is the port the extension connects to — the service is the bridge, so the name omits it.
func (s *BridgeService) Port() int {
	return s.server.Port()
}

// Listening is whether anything answers on that port. A port taken by something else — a second
// copy of the app, most often — leaves capture off for the whole session, and the window has to be
// able to say so instead of drawing an address that leads nowhere.
func (s *BridgeService) Listening() bool {
	return s.server.Listening()
}

func (s *BridgeService) PauseCapture() {
	s.server.Broadcast([]byte(`{"type":"pause"}`))
}

func (s *BridgeService) ResumeCapture() {
	s.server.Broadcast([]byte(`{"type":"resume"}`))
}

// ApplyCaptureFilters hands the extension the rules it filters by. The app stores them and this is
// the delivery: a frame to whoever is connected now, and nothing to whoever is not — the window
// sends this again on every state frame it receives, which is how an extension that has just
// reconnected is told the rules it missed.
func (s *BridgeService) ApplyCaptureFilters(filters domain.CaptureFilters) error {
	frame, err := json.Marshal(bridge.FiltersFrame{Type: "filters", Filters: filters})
	if err != nil {
		return err
	}
	s.server.Broadcast(frame)
	return nil
}

// ServiceStartup starts listening before the window appears: capture is a feature of the app, not
// a condition for it — a service that fails to start takes the window down with it.
func (s *BridgeService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	if err := s.server.Start(); err != nil {
		// Kept rather than returned: the window is worth more than capture. The failure is not lost —
		// Listening is false from here on, and that is what the window draws.
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
