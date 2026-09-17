import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import {
  BridgeService,
  RecordsService,
  SettingsService,
} from '../../bindings/json-inspector/internal/transport/wails'
import type { CaptureFilters, Settings } from '../../bindings/json-inspector/internal/domain'
import type { EditorPatch, LayoutPatch } from '../../bindings/json-inspector/internal/usecase/settings'

// What the user has set up, as Go remembers it. The window reads it once at startup and writes back
// what it changes; there is no local copy of the truth here.
const settings = ref<Settings | null>(null)
let loading: Promise<void> | null = null

// Dragging a panel fires continuously, so the write is collected and sent when the drag settles.
const pendingLayout: Record<string, unknown> = {}
let layoutTimer: ReturnType<typeof setTimeout> | null = null
const LAYOUT_DEBOUNCE_MS = 400

// The settings window writes what this one draws — how history is kept, what the raw viewer does with
// its lines, where the list sits — and neither is reloaded when the other writes. So Go's snapshot
// travels, and this is where it lands. A layout of this window's own that is still on its way out
// wins over the arriving one: the panel under the pointer is the one the user is deciding.
Events.On('settings:changed', (ev) => {
  const incoming = ev.data as Settings | null
  if (!incoming) return
  settings.value = { ...incoming, ...pendingLayout }
})

export function useSettings() {
  return {
    settings,
    loadSettings,
    setTheme,
    setLanguage,
    setLayout,
    setRetention,
    setCaptureFilters,
    setUpdateCheck,
    setUpdateChannel,
    setEditor,
    setReopenWorkspace,
  }
}

async function loadSettings(): Promise<void> {
  if (loading) return await loading
  loading = SettingsService.Snapshot()
    .then((loaded) => {
      settings.value = loaded
    })
    .catch(() => {
      // Without settings the window runs on defaults; loading again is not worth a retry loop.
    })
  await loading
}

// The choice is applied in the window by the caller and remembered here. A write that fails costs
// the choice at the next launch, which is not worth taking the switch away over.
async function setTheme(theme: Settings['theme']): Promise<void> {
  try {
    settings.value = await SettingsService.SetTheme(theme)
  } catch {
    // Nothing to do: the palette already changed on screen.
  }
}

// The language is applied in the window by the caller, the same way the theme is, and remembered
// here. A write that fails costs the choice at the next launch, not the change on screen.
async function setLanguage(language: Settings['language']): Promise<void> {
  try {
    settings.value = await SettingsService.SetLanguage(language)
  } catch {
    // Nothing to do: the window is already written in the chosen language.
  }
}

function setLayout(patch: LayoutPatch): void {
  // The window follows the pointer, not the answer: the panels move now and the row is written once
  // the drag settles.
  if (settings.value) Object.assign(settings.value, patch)
  Object.assign(pendingLayout, patch)

  if (layoutTimer) clearTimeout(layoutTimer)
  layoutTimer = setTimeout(() => {
    const flush = { ...pendingLayout }
    for (const key of Object.keys(pendingLayout)) delete pendingLayout[key]
    SettingsService.SetLayout(flush)
      .then((next) => {
        settings.value = next
      })
      .catch(() => {
        // Geometry that failed to save is geometry the next drag will try again.
      })
  }, LAYOUT_DEBOUNCE_MS)
}

// The capture rules are stored here and handed to the extension by the bridge: two calls, because
// they are two different things — one keeps the answer, the other delivers it to whoever is listening
// right now. Which of them fails is what says what went wrong.
async function setCaptureFilters(filters: CaptureFilters): Promise<Settings> {
  const saved = await SettingsService.SetCaptureFilters(filters)
  settings.value = saved
  await BridgeService.ApplyCaptureFilters(saved.captureFilters)
  return saved
}

// Shortening how long history is kept applies at once: the rules are read where records are saved
// and at startup, and without this call a window changed from "forever" to a week would keep the
// older records until twenty more requests had gone out. The count it drops is the answer, and the
// caller that drew a number beside the row reads it again.
async function setRetention(retention: Settings['historyRetention']): Promise<void> {
  settings.value = await SettingsService.SetRetention(retention)
  try {
    await RecordsService.Prune()
  } catch {
    // The choice is stored either way; what did not happen is the pruning, and the next save does it.
  }
}

async function setEditor(patch: EditorPatch): Promise<void> {
  settings.value = await SettingsService.SetEditor(patch)
}

async function setReopenWorkspace(reopen: boolean): Promise<void> {
  settings.value = await SettingsService.SetReopenWorkspace(reopen)
}

// Whether the app may look for a release on its own. The switch moves on the caller's side and the
// stored answer is what comes back: an update check nobody asked for is not a thing to run on the
// click that turns it on.
async function setUpdateCheck(auto: boolean): Promise<void> {
  settings.value = await SettingsService.SetUpdateCheck(auto)
}

async function setUpdateChannel(channel: Settings['updateChannel']): Promise<void> {
  settings.value = await SettingsService.SetUpdateChannel(channel)
}
