package wails

// The `json-inspector://` links the OS hands the app, and the second launch that carries one.

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The `json-inspector://` links the OS hands the app, and the second launch that carries one.

func (h *Host) HandleURLOpen(rawURL string) {
	h.FocusMain()
	if tabID, ok := tabFromURL(rawURL); ok {
		h.OpenTab(tabID)
	}
}

func (h *Host) OnSecondInstance(data application.SecondInstanceData) {
	for _, arg := range data.Args {
		if strings.HasPrefix(arg, deepLinkScheme+"://") {
			h.HandleURLOpen(arg)
			return
		}
	}
	h.FocusMain()
}

func tabFromURL(rawURL string) (int, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, false
	}
	value := u.Query().Get("tab")
	if value == "" {
		return 0, false
	}
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
