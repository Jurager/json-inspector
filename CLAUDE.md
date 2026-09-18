# JSON Inspector

A desktop client for JSON:API: Wails v3 (Go) + Vue 3. All the logic is in Go; Vue only draws.

## Before your first edit

**Read `docs/ARCHITECTURE.md`.** It has the layers, the naming rules and a step-by-step recipe for
adding a feature so that it lands in the structure — `domain` → port → `usecase` → `infra`/`transport` →
the mirror in the frontend. The layers are checked by the `internal/archtest` test: a violation fails
`go test`.

## Commands

```bash
task dev                                      # run with hot reload
task build                                    # build for the current OS
go vet ./... && go test ./...                 # including archtest and the migration tests
wails3 generate bindings -clean=true -ts -i   # MANDATORY after changing a Go method
cd frontend && npx vue-tsc --noEmit           # frontend types
```

Renaming or deleting a bound Go method breaks the frontend **at runtime** — calls go by numeric id —
and `vue-tsc` will not catch it. So the bindings are regenerated in the same change.

## Project rules

- **Comments explain "why", not "what"**; a line is at most 100 characters. We do not write the history
  of a change — git writes that.
- **The look is checked against the design handoff**
  `design_handoff_json_inspector/JSON Inspector - Зоны и редизайн.dc.html` (its numbers are exact; where
  it disagrees with the README, the handoff wins). No new colour tokens.
- **Behavior is checked with real events.** reka-ui **does not accept synthetic `dispatchEvent`**: it
  takes Chrome with `--remote-debugging-port` and `Input.dispatchMouseEvent` over CDP. For the extension
  headless is no good either — `Extensions.loadUnpacked` over CDP.
- **Secrets are stored in SQLite as plain text** — a deliberate trade for portability (there is no
  keychain). Therefore: `0600` on the database and its `-wal`/`-shm`, a mark in the UI, and the values do
  not reach the collection export, the logs or the DTOs — a variable DTO carries `hasValue`, and only an
  explicit `Reveal` hands a value over. A request copied as a command line is the exception, and by
  design: it carries the request's own credentials, because a request is copied whole. Whoever pastes it
  somewhere carries that.
- **Tests live next to the code**, one scenario per test, with a fake port — no window and no database.

## Where things live

| What                           | Where                                                                                                                                         |
|--------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------|
| Database                       | `platform.DataDir()` → `os.UserConfigDir()/json-inspector/app.db`                                                                             |
| Schema                         | `migrations/NNN_*.sql`; the runner is `pkg/migrate`                                                                                           |
| Name, identifier, version      | `internal/platform/identity.go`; written into the packagers' files by `internal/tools/manifest`, and a test of its own catches a disagreement |
| Services bound to the frontend | `internal/transport/wails/services_*.go`                                                                                                      |
| Event names and types          | `internal/transport/wails/events.go` (the only place)                                                                                         |
| The request engine             | `internal/infra/httpx`                                                                                                                        |
| The script sandbox             | `internal/infra/scriptengine` (goja)                                                                                                          |
| The contracts                  | `github.com/Jurager/json-inspector-proto` — their own repository, taken by version (all of them: the account is the first); a local `go.work` (in `.gitignore`) bridges to a checkout beside this one when both are edited together |
| The design handoff             | `design_handoff_json_inspector/`                                                                                                              |
