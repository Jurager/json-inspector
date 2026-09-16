package wails

import (
	"json-inspector/internal/usecase/update"
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

// PublishUpdate tells every window that the update state moved, parked until the page can take one.
// It is published on a check and on a skip, not only when a release turns up: a skip has to reach
// the windows that are showing the release just as much as a find does, and the About window is a
// document of its own that cannot see the update window's click.
func (h *Host) PublishUpdate(u *update.Info) {
	h.mu.Lock()
	if !h.ready.Load() {
		h.pendingUpdate = u
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	h.Emit(eventUpdateChanged, u)
}

// RequestUpdateCheck parks a check for the About window and asks it to run one. Both halves are
// needed and they cover different cases: the event reaches a window that is already open, and the
// flag reaches one that is being created right now, which has no listeners yet and picks the
// request up as it mounts. Neither alone is enough — an open window never mounts again, and a
// window that does not exist yet hears nothing.
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
