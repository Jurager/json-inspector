BINARY  := json-inspector
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOARCH  := $(shell go env GOARCH)

# Ad-hoc signing by default; override with a real identity in CI (e.g.
# CODE_SIGN_IDENTITY="Developer ID Application: …").
CODE_SIGN_IDENTITY ?= -

.PHONY: build package-darwin package-linux package-windows clean

# Native build via Wails (produces the .app bundle on macOS).
build:
	wails build -ldflags "$(LDFLAGS)"

# macOS: build, code-sign and zip the .app bundle.
package-darwin:
	@mkdir -p dist
	wails build -ldflags "$(LDFLAGS)"
	codesign --force --deep --sign "$(CODE_SIGN_IDENTITY)" "build/bin/$(BINARY).app"
	cd build/bin && ditto -c -k --keepParent "$(BINARY).app" "../../dist/$(BINARY)-darwin-$(GOARCH).app.zip"
	@ls -la dist

# Linux: build the binary and tar.gz it.
package-linux:
	@mkdir -p dist
	wails build -tags "webkit2_41" -ldflags "$(LDFLAGS)"
	cd build/bin && tar -czf "../../dist/$(BINARY)-linux-$(GOARCH).tar.gz" "$(BINARY)"
	@ls -la dist

# Windows: build the .exe and zip it.
package-windows:
	@mkdir -p dist
	wails build -ldflags "$(LDFLAGS)"
	powershell -Command "Compress-Archive -Path 'build/bin/$(BINARY).exe' -DestinationPath 'dist/$(BINARY)-windows-$(GOARCH).exe.zip' -Force"
	@ls -la dist

clean:
	rm -rf dist build/bin
