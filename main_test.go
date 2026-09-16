package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/fx"
)

// Every service the window calls is built from the graph, and a port with no implementation is not
// a compile error — it is a failure at startup, when there is no console to say so on. This builds
// the graph without constructing anything, so a missing or doubled provider is a test failure.
func TestTheWiringBuilds(t *testing.T) {
	if err := fx.ValidateApp(appOptions()...); err != nil {
		t.Fatalf("the dependency graph does not build: %v", err)
	}
}

// The window's URL carries the theme and the language (`/?theme=dark&lang=ru`) so the first frame
// is already painted in the right palette and written in the right words — asking Go over IPC is
// too late for either. That trick rests on the asset server resolving on the path alone: if a Wails
// upgrade starts treating the query as part of the filename, every window comes up blank, and this
// is where that shows up instead. It is also what catches a window whose document was never added
// to the frontend build, which is a blank window with nothing in the log.
func TestAssetServerIgnoresTheWindowQuery(t *testing.T) {
	handler := application.AssetFileServerFS(assets)

	cases := []struct {
		name string
		url  string
	}{
		{name: "main window", url: "/?theme=dark"},
		{name: "main window, no theme", url: "/"},
		{name: "main window, language and no material", url: "/?theme=system&lang=ru&translucent=1"},
		{name: "about window", url: "/about.html?theme=light"},
		{name: "about window, no theme", url: "/about.html"},
		{name: "settings window", url: "/settings.html?theme=light&lang=en"},
		{name: "settings window, no theme", url: "/settings.html"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.url, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", tc.url, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "<div id=\"app\">") {
				t.Errorf("GET %s did not serve the page (body starts %q)", tc.url, firstLine(rec.Body.String()))
			}
		})
	}
}

func firstLine(body string) string {
	if at := strings.IndexByte(body, '\n'); at >= 0 {
		body = body[:at]
	}
	if len(body) > 120 {
		return body[:120]
	}
	return body
}
