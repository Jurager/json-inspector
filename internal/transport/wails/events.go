// Package wails is the app's desktop surface: the services the frontend calls, the events it
// listens to, the windows and the menu. It is the only package that knows Wails exists.
package wails

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
	"json-inspector/internal/transport/bridge"
	"json-inspector/internal/usecase/account"
	"json-inspector/internal/usecase/collection"
	"json-inspector/internal/usecase/record"
	"json-inspector/internal/usecase/settings"
	"json-inspector/internal/usecase/update"
	"json-inspector/internal/usecase/workspace"
)

// Event names, declared once here. Each is registered with the payload type it carries, which is
// what makes the generated TypeScript events typed instead of `any` — see eventdata.d.ts.
const (
	eventCaptureState        = "capture-state"
	eventCaptureDisconnected = "capture-disconnected"
	eventOpenTab             = "open-tab"
	eventSettingsTab         = "settings-tab"
	eventUpdateChanged       = "update-changed"
	eventUpdateCheck         = "update-check"
)

// The calls must be direct and constant-typed: the binding generator reads them statically, and
// anything indirect is invisible to it. Names are checked, so a duplicate panics at startup.
func init() {
	application.RegisterEvent[bridge.CaptureState](eventCaptureState)
	application.RegisterEvent[application.Void](eventCaptureDisconnected)
	application.RegisterEvent[int](eventOpenTab)
	// Which category the preferences window is to show, for the window that is already on screen: a
	// window that has not been created yet reads the same thing off its address.
	application.RegisterEvent[string](eventSettingsTab)
	application.RegisterEvent[*update.Info](eventUpdateChanged)
	application.RegisterEvent[application.Void](eventUpdateCheck)
	// A preference change reaches every window: the About window draws in the same palette, and every
	// window is written in the same language.
	application.RegisterEvent[settings.ThemeChanged](settings.TopicThemeChanged)
	application.RegisterEvent[settings.LanguageChanged](settings.TopicLanguageChanged)
	// A preference that is written in one window and drawn in another: the snapshot travels, because
	// what the listener does with it is replace what it shows.
	application.RegisterEvent[domain.Settings](settings.TopicChanged)
	// Which workspace is on screen moved: every window draws around the same one.
	application.RegisterEvent[workspace.Changed](workspace.TopicChanged)
	// The account: every window draws who this installation is signed in as, and the sign-in itself
	// finishes long after the call that started it returned.
	application.RegisterEvent[account.State](account.TopicChanged)

	// History and the attempts that fill it. A record is the same type the list draws and the same
	// event, whichever side it came from.
	application.RegisterEvent[domain.Record](record.TopicRecordAdded)
	// A history thrown away in the settings window: the list in the main one is not a list of ids it
	// can prune one by one, and it reads itself again on this.
	application.RegisterEvent[record.HistoryCleared](record.TopicHistoryCleared)
	application.RegisterEvent[record.RequestFinished](record.TopicRequestFinished)
	application.RegisterEvent[record.RequestFailed](record.TopicRequestFailed)

	// The finished run's counters are what the overview draws when it opens again.
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
