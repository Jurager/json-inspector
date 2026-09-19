# Frontend

Vue decides nothing here: it repeats what Go said, and a change goes through a call rather than through
editing a field. What exactly is left to the window — view state, animations, highlighting, strings —
is laid out in [ARCHITECTURE.md](ARCHITECTURE.md), under "How the frontend mirror works" and "What
stays in Vue". This file is about where things live and what to build a screen out of.

## Folders

A component lives in the folder of its own feature; there is no flat list under `components/`:

| Folder          | What is in it                                                                       |
|-----------------|-------------------------------------------------------------------------------------|
| `request/`      | the request line and its popovers: the Query, Headers, Body, Auth and Scripts chips |
| `response/`     | the response area and its tabs: body, map, raw, headers, cookies, timings, scripts  |
| `json/`         | the document tree, the relationship map, the node inspector                         |
| `history/`      | the history list and its filter                                                     |
| `collections/`  | the collection tree, its nodes, a run, import and export                            |
| `environments/` | environments, variables and secrets                                                 |
| `workspaces/`   | the workspace switcher and its settings                                             |
| `browser/`      | captured requests and the link to the extension                                     |
| `search/`       | the search palette                                                                  |
| `update/`       | the check button and the status line — shared by the two windows that draw them     |
| `layout/`       | the titlebar, the rail, the work area, the status bar                               |
| `ui/`           | the shared primitives                                                               |

## `ui/` — the primitives

Behaviour comes from `reka-ui` (focus, Escape, click outside, ARIA, positioning) and the look from our
classes. The rule is one: **needed a second time, it goes into `ui/` instead of being written again**.

- `ui/button` — `Button` (`outline` / `primary` / `ghost` / `quiet`) and `IconButton` (`outline` /
  `bare` / `subtle` / `danger`), sizes `sm` / `md` / `lg`. The metrics come from the design handoff and
  live in one place: `lg` is 32px with an 8px radius, `md` is 26px with a 7px radius.
- `ui/input` — `Input`: 32px with an 8px radius in `md`, 28px with a 7px radius in `sm`. The `bare`
  variant is the same field without a frame, for a container that draws the frame itself: the filter
  pills, the address in the request line. Layout (`flex-1`, `w-full`) is the caller's business.
- `ui/switch` — a 38×22 toggle, green when on; its colours and size come from the handoff rather than
  from the accent.
- `ui/dialog`, `ui/popover`, `ui/dropdown-menu`, `ui/context-menu`, `ui/tabs`, `ui/checkbox`,
  `ui/tooltip` — `reka-ui` primitives under our classes. The tooltip's plate is the global `.tooltip`
  class in `style.css`: it lands in the primitive's portal, where the caller's scope does not reach.
- `ui/alert` — the frame of the question that stands between a click and unsaved work: an icon, a
  title, a hint, and the caller's own buttons in a slot. Two places ask it — a collection card and the
  command line — and only the caller knows what discarding throws away, which is why the answers are
  its business and the frame is not.

Outside that layer only our own shapes remain: chips, tags, rail tiles, segmented controls and the
window's caption buttons (`.cap-btn`). Next to the primitives sit `Icon.vue` — every glyph in one
dictionary — and `VarToken.vue`, `PanelFilter.vue`, `Toast.vue`, which need no primitive.

## `lib/` — pure functions

No Vue and no state: `format` (dates, sizes), `clipboard`, `highlightMatch`, `json`, `jsonapi` and
`schema` (JSON:API types and relationships — what the tree is drawn from), `vars` (the markup of
`{{tokens}}`), `bodyFormat`, `sample`, `collectionTree`, `commandShape`, `requestRecord`,
`requestSource`, `earlyAnswers`, `searchKinds`, `platform`, `address` (how an address is read),
`draftBuffer` (the buffer a text is typed into), `recordTabs` (what a captured tab is).

Parsing a document happens in Go, and what is left here is only what has to be **drawn**: `vars` knows
where in a text a token is, but not what it comes to — the window has no values to know.

Storage keys are declared where they are read and named `*_STORAGE_KEY`; there are two left —
`ji-theme-v1` and `ji-lang-v1`, because everything else moved into the database.

## `composables/` and `stores/`

- `composables/` — repeated reactive logic (`useTheme`, `useResizableWidth`, `useListKeys`,
  `useTreeDrag`, `useHoverArrival`, `useToast`, `useGlobalShortcuts`, `useSessionPersistence`) and
  subscriptions to Go's events, **one per area** (`useRecordEvents`, `useCaptureEvents`,
  `useWorkspaceEvents`): a subscription lives exactly as long as the window, and it is visible next to
  `onMounted`. Event names appear only there and in `internal/transport/wails/events.go`.
- `urlFocus` — a module singleton without reactivity: the request line lives for less time than a
  request for focus aimed at it.
- `stores/` — Pinia: `requests`, `collections`, `environments`, `workspaces`, `search`, and `calls`,
  which is not a store but the one place a call to Go is wrapped so that a refusal is said out loud —
  `asked` for a call that answers with something, `done` for one that answers with nothing, because
  `undefined` cannot mean both "it came back empty" and "it failed". A thin mirror otherwise: fields
  come from Go's answers and are updated by events, and nothing is computed there. A service is called
  straight from a store; there is no separate `ipc/` layer.

## Theme

The palette is one class on `<html>`, and **both** are written: `dark` and `light`. That way a rule that
has to answer "which window is this" finds an answer without a third state in mind. The palettes live in
`style.css` (`:root` and `:root.dark`), the choice in `localStorage` under `ji-theme-v1`, and
`composables/useTheme.ts` applies it.

Changing the theme is a cross-fade: the window is snapshotted as it is, the class moves, and the engine
fades one snapshot through the other (`::view-transition-*` in `style.css`, `startViewTransition` in
`useTheme.ts`). A choice made while a fade is running starts its own: waiting for the previous one to
finish would leave the click unanswered for all of it. An engine without that API (an older WKWebView)
simply changes the palette.

The same class is set by an inline script in **every** entry point — `index.html`, `about.html`,
`settings.html`, `update.html`: it runs before the first paint, or a dark window would flash light. The
script and the composable change together — it is one rule in two places, and they must not drift.
