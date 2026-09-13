package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The window's URL carries the theme (`/?theme=dark`) so the first frame is already painted in the
// right palette — asking Go over IPC is too late for that. That trick rests on the asset server
// resolving on the path alone: if a Wails upgrade starts treating the query as part of the filename,
// both windows come up blank, and this is where that shows up instead.
func TestAssetServerIgnoresTheThemeQuery(t *testing.T) {
	handler := application.AssetFileServerFS(assets)

	cases := []struct {
		name string
		url  string
	}{
		{name: "main window", url: "/?theme=dark"},
		{name: "main window, no theme", url: "/"},
		{name: "about window", url: "/about.html?theme=light"},
		{name: "about window, no theme", url: "/about.html"},
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
