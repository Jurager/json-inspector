import { computed, ref } from 'vue'
import { SWITCH_TAIL_MS, WIPE_MS } from '../lib/themeWipe'

export type Theme = 'light' | 'dark' | 'system'

const THEME_STORAGE_KEY = 'ji-theme-v1'
const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)')

// Not in TypeScript's DOM lib yet; the WebView is Chromium-based, and the webview on
// macOS may be older Safari — hence the fallback rather than a requirement.
type ViewTransition = { skipTransition(): void; finished?: Promise<void> }
type TransitionDocument = Document & { startViewTransition?: (cb: () => void) => ViewTransition }

function storedTheme(): Theme {
  const raw = localStorage.getItem(THEME_STORAGE_KEY)
  return raw === 'light' || raw === 'dark' || raw === 'system' ? raw : 'system'
}

const theme = ref<Theme>(storedTheme())
const systemIsDark = ref(systemPrefersDark.matches)
const isDark = computed(() => (theme.value === 'system' ? systemIsDark.value : theme.value === 'dark'))

// True while a wipe is on screen. Nothing in the window can be clicked until it ends — the
// transition takes the hit test with it (events land on `<html>`) — so controls that must stay
// usable over it answer by coordinates instead.
const isSwitching = ref(false)

let running: ViewTransition | null = null

// What the window has to do when it repaints, registered by whoever needs the two palettes to be
// snapshotted around it — the switch gives its pill its own layer there, so the outgoing snapshot
// still has it where the user left it and the wipe reveals the new one travelling. The flag tells
// the hook whether a wipe is really about to run: without the API, or at boot, no snapshot is
// taken and a layer name left behind would leak into the next transition.
const paintHooks = new Set<(wiping: boolean) => void>()

export function onPaint(fn: (wiping: boolean) => void): () => void {
  paintHooks.add(fn)
  return () => paintHooks.delete(fn)
}

function paint(dark: boolean, wiping: boolean) {
  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.classList.toggle('light', !dark)
  paintHooks.forEach((fn) => fn(wiping))
}

// A theme change is a diagonal wipe (see `.theme-wipe` in style.css): the document is
// mutated inside the transition, so the old palette is snapshotted and the new one is
// revealed over it instead of both changing at once.
function apply(dark: boolean, animate: boolean) {
  const start = (document as TransitionDocument).startViewTransition?.bind(document)
  if (!animate || !start) {
    paint(dark, false)
    return
  }
  // A switch mid-wipe starts a fresh one: waiting for the running transition would leave the
  // click unanswered for the rest of its second, which reads as the switch being stuck.
  running?.skipTransition()
  const transition = start(() => paint(dark, true))
  running = transition
  isSwitching.value = true

  // Only the transition still in charge finishes it, or a skipped one would lower the flag while
  // its successor is on screen. The timer covers a `finished` that never settles.
  // `--pill-shift`/`--wipe-arrival` are never cleared here: pill-slide reads them live through
  // `var(x, fallback)`, so clearing mid-animation would snap it to the fallback early — `arm()`
  // overwrites both before the next wipe needs them.
  const finish = () => {
    if (running !== transition) return
    isSwitching.value = false
  }
  transition.finished?.then(finish, finish)
  window.setTimeout(finish, WIPE_MS + SWITCH_TAIL_MS + 150)
}

function persist(next: Theme) {
  try {
    localStorage.setItem(THEME_STORAGE_KEY, next)
  } catch {
    // Quota or unavailable storage: the choice still holds for this session.
  }
}

systemPrefersDark.addEventListener('change', (e) => {
  systemIsDark.value = e.matches
  if (theme.value === 'system') apply(e.matches, true)
})

// The About window is a separate app: the storage event is the only signal that crosses.
window.addEventListener('storage', (e) => {
  if (e.key !== THEME_STORAGE_KEY) return
  theme.value = storedTheme()
  apply(isDark.value, true)
})

// At boot there is nothing to wipe from.
apply(isDark.value, false)

export function useTheme() {
  return {
    theme,
    isDark,
    isSwitching,
    setTheme(next: Theme) {
      theme.value = next
      persist(next)
      apply(isDark.value, true)
    },
  }
}
