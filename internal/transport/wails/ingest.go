package wails

import (
	"json-inspector/internal/transport/bridge"
)

// captureIngest turns what the extension sends into events for the window. It is deliberately not
// a bound service: Wails binds every exported method of a service, and these are callbacks rather
// than an API the frontend may call.
type captureIngest struct {
	host *Host
}

func newCaptureIngest(host *Host) bridge.Ingest {
	return captureIngest{host: host}
}

func (c captureIngest) Captured(req bridge.CapturedRequest) {
	c.host.Emit(eventCapturedRequest, req)
}

func (c captureIngest) StateChanged(state bridge.CaptureState) {
	c.host.Emit(eventCaptureState, state)
}

func (c captureIngest) ClientGone() {
	c.host.Emit(eventCaptureDisconnected)
}

func (c captureIngest) FocusRequested(req bridge.FocusRequest) {
	c.host.FocusMain()
	if req.Tab > 0 {
		c.host.OpenTab(req.Tab)
	}
}
