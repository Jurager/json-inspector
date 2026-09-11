BINARY  := json-inspector
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOARCH  := $(shell go env GOARCH)

# Ad-hoc signing by default; override with a real identity in CI (e.g.
# CODE_SIGN_IDENTITY="Developer ID Application: …").
CODE_SIGN_IDENTITY ?= -

# Exported so the v3 Taskfile's {{.VERSION}} picks it up and forwards it to
# -X main.version (see the BUILD_FLAGS lines in build/*/Taskfile.yml).
export VERSION

# Release asset names are a contract with internal/update/assetName() and the
# swap_* implementations — the archives published here are what the in-app
# updater downloads. Do not rename them; add installers alongside instead.
.PHONY: build icons package-darwin package-linux package-windows clean

build:
	task build

# `.icns` and `.ico` are generated from the single source build/appicon.png.
icons:
	task common:generate:icons

# macOS ships two artefacts: a .dmg for humans and the .app.zip the updater
# consumes. Sign with the real identity before zipping, so both carry it.
package-darwin: icons
	@mkdir -p dist
	task darwin:package:universal
	codesign --force --deep --sign "$(CODE_SIGN_IDENTITY)" "bin/$(BINARY).app"
	cd bin && ditto -c -k --keepParent "$(BINARY).app" "../dist/$(BINARY)-darwin-arm64.app.zip"
	@# One universal build, published under both architecture names. The updater
	@# picks the asset by runtime.GOARCH (see internal/update/assetName), so an
	@# Intel Mac asking for -darwin-amd64.app.zip would otherwise get a 404 and
	@# silently never update. Duplicating the archive keeps that code untouched.
	cp "dist/$(BINARY)-darwin-arm64.app.zip" "dist/$(BINARY)-darwin-amd64.app.zip"
	task darwin:create:dmg
	cp "bin/$(BINARY).dmg" "dist/$(BINARY)-darwin-universal.dmg"
	@ls -la dist

package-linux: icons
	@mkdir -p dist
	task linux:build
	cd bin && tar -czf "../../dist/$(BINARY)-linux-$(GOARCH).tar.gz" "$(BINARY)"
	@# Installers for humans, alongside the tarball the updater downloads. These
	@# carry the .desktop entry, which is what registers json-inspector:// with
	@# xdg-mime — the tarball alone cannot, being just a bare binary.
	task linux:create:deb
	task linux:create:rpm
	task linux:create:appimage
	@# The nfpm/AppImage output names are the CLI's business, so copy whatever it
	@# produced rather than hardcoding them.
	@for f in bin/*.deb bin/*.rpm bin/*.AppImage; do \
		if [ -e "$$f" ]; then cp "$$f" dist/; fi; \
	done
	@ls -la dist

# Windows ships the same pair: an NSIS installer that registers the
# json-inspector:// scheme, and the .exe.zip the updater swaps in place.
package-windows: icons
	@mkdir -p dist
	task windows:build
	powershell -Command "Compress-Archive -Path 'bin/$(BINARY).exe' -DestinationPath 'dist/$(BINARY)-windows-$(GOARCH).exe.zip' -Force"
	task windows:create:nsis:installer
	cp "bin/$(BINARY)-$(GOARCH)-installer.exe" "dist/$(BINARY)-windows-$(GOARCH)-installer.exe"
	@ls -la dist

clean:
	rm -rf dist bin .task
	rm -f build/linux/*.desktop
