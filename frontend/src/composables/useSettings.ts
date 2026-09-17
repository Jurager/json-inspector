import { ref } from 'vue'
import { BridgeService, SettingsService } from '../../bindings/json-inspector/internal/transport/wails'
import type { CaptureFilters, Settings } from '../../bindings/json-inspector/internal/domain'
import type { LayoutPatch } from '../../bindings/json-inspector/internal/usecase/settings'

// What the user has set up, as Go remembers it. The window reads it once at startup and writes back
// what it changes; there is no local copy of the truth here.
const settings = ref<Settings | null>(null)
let loading: Promise<void> | null = null

// Dragging a panel fires continuously, so the write is collected and sent when the drag settles.
const pendingLayout: Record<string, unknown> = {}
let layoutTimer: ReturnType<typeof setTimeout> | null = null
const LAYOUT_DEBOUNCE_MS = 400

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

async function setRetention(retention: Settings['historyRetention']): Promise<void> {
  settings.value = await SettingsService.SetRetention(retention)
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
