package wails

// What the app tells a window that is already open: which tab to show, and that a release is
// waiting. Both are parked while the page cannot take an event yet.

import (
	"json-inspector/internal/infra/updater"
)

// What the app tells a window that is already open: which tab to show, and that a release is
// waiting. Both are parked while the page cannot take an event yet.

// OpenTab switches the rail to the extension's tab, parking the request if the window can't
// take events yet. The latest request wins: an earlier tab doesn't need reopening.
func (h *Host) OpenTab(tab int) {
	h.mu.Lock()
	if !h.ready.Load() {
		h.pendingTab = tab
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	h.Emit(eventOpenTab, tab)
}

// AnnounceUpdate tells the window a release is available, parking it the same way OpenTab does.
func (h *Host) AnnounceUpdate(u *updater.Info) {
	h.mu.Lock()
	if !h.ready.Load() {
		h.pendingUpdate = u
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	h.Emit(eventUpdateAvailable, u)
}

// RequestUpdateCheck parks a check for the About window and asks it to run one. The request is
// parked as well as sent, because only the About window listens for it here — and a window that
// is being created right now has no listeners yet. It picks the parked request up on mount.
//
// The flag is cleared by TakeUpdateCheckRequest alone, never here: that is what makes the two
// paths deliver exactly one check between them, however they interleave.
func (h *Host) RequestUpdateCheck() {
	h.mu.Lock()
	h.pendingUpdateCheck = true
	h.mu.Unlock()

	// Events go to every window in this version of Wails; nothing but the About window listens.
	h.Broadcast(eventUpdateCheck)
	h.ShowAbout()
}

// TakeUpdateCheckRequest reports and clears a request parked by RequestUpdateCheck. Destructive
// on purpose: it is called from both the mount and the event handler, and only the first caller
// may act on it, or an old request would fire a check the next time the window opens.
func (h *Host) TakeUpdateCheckRequest() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	pending := h.pendingUpdateCheck
	h.pendingUpdateCheck = false
	return pending
}
