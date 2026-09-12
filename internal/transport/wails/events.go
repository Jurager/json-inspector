// Package wails is the app's desktop surface: the services the frontend calls, the events it
// listens to, the windows and the menu. It is the only package that knows Wails exists.
package wails

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/infra/updater"
	"json-inspector/internal/transport/bridge"
)

// Event names, declared once here. Each is registered with the payload type it carries, which is
// what makes the generated TypeScript events typed instead of `any` — see eventdata.d.ts.
const (
	eventCapturedRequest     = "captured-request"
	eventCaptureState        = "capture-state"
	eventCaptureDisconnected = "capture-disconnected"
	eventOpenTab             = "open-tab"
	eventUpdateAvailable     = "update-available"
	eventUpdateCheck         = "update-check"
)

// The calls must be direct and constant-typed: the binding generator reads them statically, and
// anything indirect is invisible to it. Names are checked, so a duplicate panics at startup.
func init() {
	application.RegisterEvent[bridge.CapturedRequest](eventCapturedRequest)
	application.RegisterEvent[bridge.CaptureState](eventCaptureState)
	application.RegisterEvent[application.Void](eventCaptureDisconnected)
	application.RegisterEvent[int](eventOpenTab)
	application.RegisterEvent[*updater.Info](eventUpdateAvailable)
	application.RegisterEvent[application.Void](eventUpdateCheck)
}
