// Package wails is the app's desktop surface: the services the frontend calls, the events it
// listens to, the windows and the menu. It is the only package that knows Wails exists.
package wails

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
	"json-inspector/internal/infra/updater"
	"json-inspector/internal/transport/bridge"
	"json-inspector/internal/usecase/collection"
	"json-inspector/internal/usecase/record"
	"json-inspector/internal/usecase/settings"
	"json-inspector/internal/usecase/workspace"
)

// Event names, declared once here. Each is registered with the payload type it carries, which is
// what makes the generated TypeScript events typed instead of `any` — see eventdata.d.ts.
const (
	eventCaptureState        = "capture-state"
	eventCaptureDisconnected = "capture-disconnected"
	eventOpenTab             = "open-tab"
	eventUpdateAvailable     = "update-available"
	eventUpdateCheck         = "update-check"
)

// The calls must be direct and constant-typed: the binding generator reads them statically, and
// anything indirect is invisible to it. Names are checked, so a duplicate panics at startup.
func init() {
	application.RegisterEvent[bridge.CaptureState](eventCaptureState)
	application.RegisterEvent[application.Void](eventCaptureDisconnected)
	application.RegisterEvent[int](eventOpenTab)
	application.RegisterEvent[*updater.Info](eventUpdateAvailable)
	application.RegisterEvent[application.Void](eventUpdateCheck)
	// A preference change reaches every window: the About window draws in the same palette, and every
	// window is written in the same language.
	application.RegisterEvent[settings.ThemeChanged](settings.TopicThemeChanged)
	application.RegisterEvent[settings.LanguageChanged](settings.TopicLanguageChanged)
	// Which workspace is on screen moved: every window draws around the same one.
	application.RegisterEvent[workspace.Changed](workspace.TopicChanged)

	// History and the attempts that fill it. A record is the same type the list draws — and the same
	// event, whether it came from a request this app sent or from the browser.
	application.RegisterEvent[domain.Record](record.TopicRecordAdded)
	application.RegisterEvent[record.RequestFinished](record.TopicRequestFinished)
	application.RegisterEvent[record.RequestFailed](record.TopicRequestFailed)

	// A run of a collection: one event per request it reaches, and the finished run with its
	// counters, which is what the overview draws when it opens again.
	application.RegisterEvent[collection.RunProgress](collection.TopicRunProgress)
	application.RegisterEvent[domain.CollectionRun](collection.TopicRunFinished)
}

// bus is the Notifier the features publish to. A topic is the event name — one spelling, so the
// registration above and the publish below cannot drift apart.
type bus struct {
	host *Host
}

func newBus(host *Host) *bus {
	return &bus{host: host}
}

func (b *bus) Publish(topic string, payload any) {
	b.host.Broadcast(topic, payload)
}
