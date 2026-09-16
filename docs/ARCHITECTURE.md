# Architecture

How the application is put together, why exactly this way, and **how to add a new feature**. Read it before
your first edit in Go, and re-read it if the recipe has drifted from the code — then it is the document that gets fixed.

## Why everything is in Go

The logic used to live in TypeScript: two Pinia stores held history, capture, draft, environments and
secrets, four `localStorage` keys were the only storage, and Go sent requests and served the extension
bridge. Problems grew out of that: the storage was a blob in the WebView profile (no queries, no retention,
the bodies of all records were pulled into the UI whole), secrets were kept in the macOS keychain and
did not work on Windows/Linux, the form of a captured request was duplicated in three
places, and there was not a single test.

Now the only source of truth is Go: the network, storage (SQLite + migrations), environments and
variables with secrets, history and capture, collections, parsing of documents and commands, user
scripts. Vue draws and holds only view state.

## Layers

The arrow means "is allowed to import". The rule is checked by the `internal/archtest` test, not by
agreement: a violation fails the build.

```
main.go ──► transport/ ──► usecase/ ──► domain/
                │             │
                └──► infra/ ◄─┘
                       ▲
  pure (stdlib): dotenv/ postman/ · pkg/
```

| Layer                        | May import                                  | May not                                     |
|------------------------------|---------------------------------------------|---------------------------------------------|
| `internal/domain`            | stdlib                                      | anything of ours — at all                   |
| `internal/usecase/<feature>` | `domain`, its own interfaces, pure packages | other features, `infra`, `transport`, Wails |
| `internal/infra/*`           | `domain`, pure packages, drivers            | `usecase`, `transport`, Wails               |
| `internal/transport/*`       | everything                                  | — (composition lives here)                  |
| `internal/platform`          | stdlib                                      | everything else                             |
| `pkg/*`                      | stdlib                                      | anything starting with `json-inspector/`    |

Three rules that settle most "where does this go" questions:

- **Features do not import each other.** You need someone else's feature — declare a small interface on
  your side (`VarResolver`, `RecordWriter`) and get the implementation through fx. The **consumer** declares
  the port. That way the `drafts ↔ collections` cycle is caught immediately, not half a year later.
- **The outside world is a package in `infra/` or `transport/`.** A new protocol, driver, service — a new
  package there, not a method in a use case.
- **`pkg/` is what is not a layer.** There is exactly one criterion and it is checkable: the package imports
  not a single line of `json-inspector/`. `pkg/migrate` takes a `*sql.DB` and an `fs.FS` and knows neither
  about the domain nor about use cases — it is a tool we carry with us, not part of the architecture.
  A package that imports anything of ours at all stays under `internal/`: `internal/postman`
  imports `internal/domain`, because it converts into our entities — it is ours.

## The tree

```
main.go                       wiring: fx + Wails, no logic
migrations/0001_*.sql …       schema, one file per area + embed.go
internal/
  domain/                     entities, rules, errors; zero project imports
  usecase/                    use cases: a directory per feature, the facade is usecase.go
  infra/
    sqlite/                   Store: connection, pragmas, migrations
    httpx/                    request engine
    authflow/                 auth schemes: what each answers on the wire
    scriptengine/             script sandbox (goja + prelude.js)
    updater/                  self-update: GitHub, archive, binary swap — mechanism only
  transport/
    wails/                    bound services, events, windows, menu
    bridge/                   extension WebSocket server
  dotenv/ postman/                            pure algorithms, tests next to them
  platform/                   paths, ids, build identity
  tools/manifest/             identity → packager files; checked by a test
  archtest/                   dependency and naming rules, as a test
pkg/
  migrate/                    migration runner + testdata; imports not a line of ours
```

### Where things stand today

Moved to Go: **environments and variables** (`usecase/environment`, secrets are stored in the database in
plain text), **history** (`usecase/record`: sending, recording, bodies, retention, legacy import),
**draft** (`usecase/draft`: model, address and cookie-jar transformations, preview, persistence),
**the collection tree, run and Postman** (`usecase/collection`: nodes, duplicating a subtree, traversal
and sequential sending; `internal/postman` — import and export) and **scripts**
(`usecase/scripting`: inheritance chain, run scope, reports; the sandbox is `infra/scriptengine`
on goja). **Authorization** is a table of schemes: `domain.AuthSchemes()` declares which ones exist, what
each asks for and how to draw it, and `infra/authflow` is the second half of the same table — what
each answers. A scheme is "fields + wiring to a library", and it is added by a row in the table and a
row in the map, not by a branch in a `switch`: no call path looks at the scheme type, neither in Go nor
in the markup. Working: Bearer, Basic, API key (header or query), JWT (signed with `golang-jwt`),
AWS SigV4 (`smithy-go/aws-http-auth`), Digest (the engine answers the challenge, `icholy/digest`) and
OAuth 2.0 (`golang.org/x/oauth2`: four grants, two of them through the browser and a loopback listener).
It is applied to the request in one place — the draft's `prepare`, next to variable substitution; the line
a scheme produces in "Headers" or "Parameters" arrives at the window already computed (`State.Projected`)
and is edited back into the scheme's fields. "Inherit" on a request inside a collection is resolved by the
nearest level above that actually answered: by the same tree traversal the run uses (`collection.AuthFor` —
for the card, `requestsIn` — for the run), and the projection for an inherited level is completed by the
transport — the only layer that knows both the tree and the draft. "None" and "Inherit" are answers about
who authorizes the request, not credentials: a level that gave one of them counts as not having answered,
and the traversal goes past it (`Auth.Answered()` — what asks that of the value, not of the absence of the
value). A level that never answered anything is stored as `NULL`; a level that said "no" after the fields
were already filled keeps them — so that a return to the previous type finds the token in place. A command
line cannot inherit — there is nothing above it, so its chip offers "None" instead of "Inherit". A token
that has been obtained does not become a field: it lives in `authflow` and is not shown to the window —
only "exists" and "until when" (`domain.AuthToken`).
Bound services — `SystemService`, `SettingsService`, `RecordsService`, `DraftService`,
`EnvironmentsService`, `CollectionsService`, `ScriptingService`, `BridgeService`,
`WorkspaceService`, `SearchService`; windows and the menu live in `transport/wails`, the extension bridge —
in `transport/bridge`.

Global search (`usecase/search`) is the only feature that reads **across** aggregates: the palette
searches collections, environments, history and commands at once, and asking each feature separately
is not possible — features do not import each other. That is why the port `search.Index` is declared by the
consumer and implemented by the same `sqlite.Store`, not by a set of methods in other features' use cases:
search is a projection for reading, not a step of someone else's use case. **A new search area is a method
on `Index`, a query in `infra/sqlite/search_store.go` and a row in `areas()`**; `domain` and `transport`
are not touched in the process. The method is called `Find<Area>`, because the name `Collections` on
`sqlite.Store` is already taken by another port, and a type cannot have two methods with the same name.
Matching is done in Go, not in SQL: `LIKE` in SQLite folds case only for Latin, and "Пользователи" would
not be found by "польз". Secrets are searched by name and never by value — the value is not substituted
into the query at all, and it is not in the response either on a match or without one.

A request is sent entirely on the Go side: the window asks `RecordsService.Send()` and sees neither the
values of variables nor what came of them — only the record that landed in history.

An address typed without a scheme (`api.example.com/articles`) is completed in the engine — the only place
where the request leaves the process: that is `https`, and for local names (`localhost`, `127.0.0.1`,
`[::1]`) `http` — as in a browser's address bar. A scheme that was written is left alone, even one the
engine does not know: its refusal is more honest than rewriting (`infra/httpx/address.go`).

The answer to an attempt can overtake the return of the `Send` call itself: an attempt that fails instantly
is ready before the window has learned its id. That is why an answer that no panel recognized waits for its
panel under its id (`lib/earlyAnswers.ts`) and is picked up at the moment the call returns — otherwise the
spinner would be waiting for what has already arrived. The id is unique, so panels cannot be mixed up,
whatever order the two messages arrive in.

Collections leave and arrive as a Postman v2.1 format file: `internal/postman` is a pure package
(bytes ⇄ collection), file dialogs live on `Host` and answer "nothing" to a closed dialog.

Scripts run around the request as a chain: the collection's scripts, then each folder on the way down, then
its own. A level is a collection, a tree node or the command-line draft: a request that nobody saved also
has its own code, and it lives in the draft row (`drafts.scripts_json`), because it has neither a collection
nor a node. The report of each script run hangs on the record around which it ran — that is why both halves
of the scripts are written after the send, and the reports of the first half reach it in `ScriptPass.Ran`.
What the sandbox deliberately lacks: the network, files, timers and `pm.sendRequest`.

A command travels both ways through Go, and it lives in `usecase/draft` — a command is a request written
differently. What was pasted into the address line is read by `DraftService.PasteCommand`: parsing and
replacing the draft in one call, because a window that takes the fields only to hand them back is a window
that parses what Go has just assembled. "Copy as cURL" — `CommandService.Export`, also a draft use case,
not a separate package.

**Only curl is read, all five are written.** Parsing of a pasted string knows one notation: fetch, wget,
httpie and Invoke-RestMethod are ways to *carry a request away*, and nobody pastes a PowerShell line into
the address field to get a request back. Export, however, knows all five: where a person will take the
request — to a terminal, to the browser console, to a PowerShell script — is his decision, not ours. The
parsing fixtures of the other four tools were deleted along with their readers (57 parse cases), the export
corpus is intact — 20 cases, four per notation.

Parsing was verified against the fixture corpus (`internal/usecase/draft/testdata`), taken from the previous
TS implementation before it was deleted. From TS we deliberately departed in two places, and both are
recorded in the tests: the order of headers — the one in which they were written, not the one in which a JS
object would list them; and no JavaScript model — `encoding/json` instead of `JSON.parse` and
`JSON.stringify`, `url.QueryEscape` instead of `encodeURIComponent`. This did not affect parsing by a single
fixture.

What the window decides and what Go decides: **whether what was pasted looks like a command** is decided by
the window (`lib/commandShape.ts`, a regex), because `preventDefault` must be called before any answer
arrives — and a mistake is possible here. **What it actually is** is decided by Go, and a mistake is no
longer allowed there. An ordinary URL in the address line — the most frequent paste in the application —
does not fall under the check and is pasted by the browser.

A secret does not leave on export either: into history the record lands already with the hidden half of the
substitution (`prepare.go` puts `hidden` into it, not `raw`), so the window receives a ready string with
dots instead of values, and `Export` runs the substitution once more — as insurance, not as work.

Still living in TS and moving on further: parsing of JSON:API documents (`jsonapi.ts`, `schema.ts`) and
`json.ts`. Highlighting (`highlightJson`) and token markup (`tokenSegments`) stay in the window by
definition — this is HTML on top of a real input field, not data.

### How the frontend mirror works

There is one rule: **Vue decides nothing, it repeats what Go said**, and writing goes through a call,
not through a local edit of a field.

- A service is called straight from the store (`RecordsService.List`, `EnvironmentsService.Snapshot`), and
  the answer is put into state whole. There is no separate `ipc/` layer: while each feature has one or two
  calls and one or two events, an extra layer would be one more place where names diverge.
- Events are listened to by **one composable per area** (`useRecordEvents`, `useCaptureEvents`,
  `useTheme`), not by the store: the subscription lives exactly as long as the window, and it is visible
  next to `onMounted`. Event names occur only there and in `events.go`.
- Fields that Go returns as a list with a possible `null` are normalized **at the boundary** — in a getter
  or in a view function (`recordView`), once, not in every template.
- The list of records arrives without bodies, and that is not a mistake: a body is a `BodyRef`, and the
  viewer asks for it with a separate call when it opens a record (`RecordsService.Body`).

### Input buffers

Two texts — the address and the body — the window holds **itself while they are being typed into**: these
are the only data that cannot be re-asked from Go without losing the caret. It works like this:

- on every input the window computes its own `rev`, puts the text into the buffer and hands it over after a
  pause (400 ms), and on blur, Enter, send and window hide — `DraftService.SetText`;
- the answer comes back together with the same `rev`; an answer to a **stale** `rev` is discarded entirely,
  and while its own has not come back in an answer, `/draft.url` from Go is not substituted into the field.
  That way a letter is neither lost nor doubled, and the race "the answer arrived after the next letter" is
  resolved without locks;
- everything else — rows of parameters, headers, the cookie jar, the method, Auth — is written to Go
  **immediately**: this is not text under the caret but discrete edits, and each is addressed by **row id**,
  not by index.

The `{{name}}` tokens are drawn by the window: `{{` and `}}` are markup (highlighting on top of a real
input, like `highlightJson`), its input is the buffer text, its output is pills. The window cannot and must
not substitute a value or a mask: it has no values. That is why **resolution** goes to Go (what turns into
what, what is missing), while **markup** stays in the window (where to draw the pill).

## Naming

- **A package is named for what it gives.** No `util`, `common`, `helpers`, `models`, `types`,
  `constants` — such names are forbidden and checked by a test.
- **A file is a role in a feature**: `usecase.go` — the package facade, `transform.go`/`preview.go` — parts
  of one responsibility, `<name>_test.go` — tests next to it.
- **One aggregate — one feature directory.** Wanted a second responsibility — a separate feature or a
  separate package, not a 34th file in `request`.
- **`module.go` — one line per package** (`var Module = fx.Module(…)`); the feature aggregate is assembled
  by `usecase/module.go`. A new feature = a directory + a line in the aggregate, `main.go` is not touched.
- **Errors are domain errors**: `errors.Is(err, domain.ErrNotFound)`; `sql.ErrNoRows` does not leak out.
- **Names going out** (to TS) are taken from `domain`, without echo prefixes.

## How to add a feature

1. **Concepts and rules** — in `domain/`. If a new concept appeared, its type and invariants go there too.
2. **Port** — you declare the interface of the I/O you need **on your side**: `usecase/<feature>/ports.go`.
   The implementation will land in `infra/`, but the use case must not know about that.
3. **Use cases** — a directory `usecase/<feature>/` with a `usecase.go` facade. Someone else's feature —
   through your own interface.
4. **Storage** — a migration `migrations/NNNN_*.sql` + a file `<aggregate>_store.go` in `infra/sqlite`.
5. **Outward** — a method in `transport/wails/services_<feature>.go`; need a push — the type and name of the
   event in `events.go` (and `application.RegisterEvent[...]` there too).
6. **In the frontend** — only the mirror: a field in the store + applying the event. No logic.

### Example: pinning a record in the history

```go
// 1. domain/record.go — the concept and the rule live next to each other.
type RecordSummary struct {
    ID      string
    Pinned  bool
    // …
}

// 2. usecase/record/ports.go — what the use case needs from storage. The consumer declares it.
type RecordStore interface {
    SetPinned(ctx context.Context, id string, pinned bool) error
    ListRecent(ctx context.Context, limit int) ([]domain.RecordSummary, error)
}

// 3. usecase/record/usecase.go — the package facade.
func (u *UseCase) Pin(ctx context.Context, id string, pinned bool) error {
    if err := u.store.SetPinned(ctx, id, pinned); err != nil {
        return fmt.Errorf("pinning record %s: %w", id, err)
    }
    u.notify.Publish(events.RecordChanged, domain.RecordChanged{ID: id, Pinned: pinned})
    return nil
}
```

```sql
-- 4. migrations/0007_records_pinned.sql
ALTER TABLE records ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
```

```go
// 5. transport/wails/services_record.go — the binding, and nothing besides it.
func (s *RecordsService) PinRecord(ctx context.Context, id string, pinned bool) error {
    return s.records.Pin(ctx, id, pinned)
}
```

```ts
// 6. In the frontend — the mirror. Types come from the bindings, they are not hand-written.
// The store: the service call and a state field.
async pin(id: string, pinned: boolean) {
  await RecordsService.Pin(id, pinned)
  const record = this.records.find((r) => r.id === id)
  if (record) record.pinned = pinned
}

// The composable: a subscription to the event and nothing more.
Events.On('record:changed', (ev) => store.applyPinned(ev.data))
```

Checks: `go test ./internal/archtest/...` (the layers have not moved), `wails3 generate bindings -clean=true
-ts -i` (otherwise the frontend will not see the method), a use case test with a fake store.

### Review checklist

- [ ] The feature does not import another feature; someone else's — through your own interface.
- [ ] A new outside world is a new package in `infra/` or `transport/`, not a method in a use case.
- [ ] The DTO is declared once, in `domain`; there are no hand-written copies in TS.
- [ ] Every event has a type in `RegisterEvent`, and its name is only in `events.go`.
- [ ] `domain` knows nothing about SQL, windows and HTTP.
- [ ] Tests next to the code; for a use case — with a fake port, without a window and a database.
- [ ] `wails3 generate bindings` has been run, the bindings diff is in the commit.

## What stays in Vue

View state: the active response tab, an open popover, the search text, selection in the tree, the caret and
scroll. Animations (`themeWipe.ts`, `useHoverArrival`, `useResizableWidth`), `clipboard.ts`, localized
strings (`format.ts`), JSON highlighting (`highlightJson` — this is HTML), the theme script before the
first paint in `index.html`/`about.html` and all of the rendering.

Pinia stays a thin mirror: fields come from Go's responses and are updated by events; there are no
computations there. Event names are known only to the subscription composables and `events.go`; there are
no `as` casts left in the code — types for `Events.On` are generated together with the bindings
(`eventdata.d.ts`).

## What we deliberately do not do

- A repository interface per aggregate with a single implementation: an interface appears when there is a
  fake in a test or a second consumer.
- Moving bodies to disk (the `file_path` column is reserved, in v1 — an 8 MiB limit with `truncated`).
- A pool of `http.Client` per profile, while there is no proxy settings UI.
- A mock layer of services to run the UI without Go — we will come back to it if needed, and it will be
  `transport/mock`, not a second set of use cases.
- `pm.sendRequest`, `setTimeout`, a chai clone, a full Postman v2.1, response diffs before there is a UI.
- `fx.Annotate`, named values, `fx.Module` for every little thing: ordinary constructors + one `fx.In`.
- Account, sync, sharing: there are places for them in the design handoff, but no data model. Workspaces,
  however, are done in full (see below), so both the account and the cloud land on a ready axis, not beside
  it.

## Workspaces

A workspace is a top-level container: history, collections, environments and the request draft belong to
one. Application settings — theme, language, panel geometry, list order — do not belong to workspaces at
all: these are global settings of the installation itself, one set for all.

- **Data.** `workspaces(id, name, kind, color, active_environment_id, position, …)`, and `workspace_id` is
  present in `records`, `collections`, `environments`, `variables`, `script_runs` and `drafts` (in the
  latter — as the composite key `(workspace_id, id)`). One database, workspace as a column: this is what
  will map one-to-one onto a server, and what will not have to be rewritten with queries when workspaces
  become many. `settings` and `data_imports` stay global.
- **Who knows about a workspace.** Each usecase has its own mini-port `Scope{ActiveWorkspace(ctx)}`
  (features do not import each other), the implementation is one — `sqlite.Store`, wired in
  `transport/wails/module.go`. The pointer to the current workspace is a setting of the installation
  (`workspace.activeId`), not workspace data: there is always exactly one on screen.
- **A workspace is resolved once per operation** and then travels as a parameter — especially in goroutines
  (`record.Send`, `collection.execute`, `ingest`): a switch in the middle of a run must not take its results
  somewhere else.
- **The personal workspace by default** (`domain.WorkspacePersonalID = "personal"`) is created by a
  migration; it cannot be deleted, the others can — together with all their contents (cascade + one
  `DELETE`).

- **Colour.** It is both the workspace's clothing and the only thing visible from outside its data: `color`
  goes onto the avatar in the switcher, and the chosen colour also tints the window. An empty `color` is a
  state, not an omission: a workspace without a colour looks exactly as the application has always looked
  (the avatar is a neutral grey, like "Personal" in the design handoff), and the window stays untouched.
  The word from the palette travels to `<html data-tint>`, and what it turns into is the business of
  `style.css`, and there are two different things there:
  - **Glass** (header, rail, status bar) gets the colour **under itself**, not on top: the glass fill is the
    top `background-image` layer, the colour gradient is the bottom one. A tint on top of white glass reads
    as grey paint, the same colour under the glass — as light shining through; the frames in the design
    handoff are built the same way. The band is even across its whole width and ends together with it — as
    in the design handoff, where the frame's tint does not fade out but simply runs into the chrome border;
    damping it in advance was tried, and that reads as a sharp gradient right at the content.
  - **Content backgrounds** (`--bg`, `--bg-panel`, `--bg-inset`) shift towards the colour by a couple of
    percent via `color-mix` — the page is slightly coloured instead of just white or just dark grey. A
    couple, not more: the background must read as a background.
  The light and dark themes keep their own pairs of stops: white glass takes the colour more willingly,
  almost black swallows it, hence the numbers differ while the effect is one.

Where this develops without redoing what is done: a team workspace is the same row with `kind = 'team'`;
members — a new table `workspace_members(workspace_id, user_id, role)`, for which a block is already drawn
in the settings card; the cloud — `workspace_id` in every table is exactly the one the server will have;
sync will need `updated_at` and deletion markers where they are missing (`records`, bodies, runs) — an
additive migration.

## Checking your work

```bash
go vet ./... && go test ./...        # including archtest, migrations, Store
task build                            # build the application for the current OS
task dev                              # run with hot reload
wails3 generate bindings -clean=true -ts -i    # after any edit of Go methods
cd frontend && npx vue-tsc --noEmit   # frontend types against the bindings
```

Traps we have already stepped on:

- **Renaming a bound method breaks the frontend at runtime**, not at build time: bindings are called by
  numeric id, and `vue-tsc` does not see it. That is why `generate bindings` is a mandatory step, not a
  "when I remember" one.
- **SQLite pragmas live in the DSN.** `foreign_keys` applies to a connection, and the pool hands out
  different ones, so `db.Exec("PRAGMA …")` silently disables all `ON DELETE CASCADE`. The `store_test.go`
  test checks exactly the connection the pool hands out.
- **Before `Run()` Wails has no window implementation**, so `app.Dialog` panics. A database startup error
  is remembered in `Status` and drawn by the frontend on a reduced set of services.
