import { computed, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { useSettings } from './useSettings'
import { Theme as DomainTheme } from '../../bindings/json-inspector/internal/domain'

// The choice is declared once, in Go, and reaches the window as a generated enum.
export type Theme = DomainTheme

/** Reads a stored or URL-supplied value; anything else — a newer build's theme — is not one. */
function asTheme(raw: string | null): Theme | null {
  switch (raw) {
    case 'light':
      return DomainTheme.ThemeLight
    case 'dark':
      return DomainTheme.ThemeDark
    case 'system':
      return DomainTheme.ThemeSystem
    default:
      return null
  }
}

const THEME_STORAGE_KEY = 'ji-theme-v1'
const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)')

// Not in TypeScript's DOM lib yet; the WebView is Chromium-based, and the webview on
// macOS may be older Safari — hence the fallback rather than a requirement.
type ViewTransition = { skipTransition(): void }
type TransitionDocument = Document & { startViewTransition?: (cb: () => void) => ViewTransition }

/** The choice Go put on the window's URL — what the first frame has already painted with. */
function urlTheme(): Theme | null {
  return asTheme(new URLSearchParams(location.search).get('theme'))
}

/** The cache a dev reload falls back on, since a reload carries no query string. */
function cachedTheme(): Theme {
  try {
    return asTheme(localStorage.getItem(THEME_STORAGE_KEY)) ?? DomainTheme.ThemeSystem
  } catch {
    return DomainTheme.ThemeSystem
  }
}

const theme = ref<Theme>(urlTheme() ?? cachedTheme())
const systemIsDark = ref(systemPrefersDark.matches)
const isDark = computed(() =>
  theme.value === DomainTheme.ThemeSystem ? systemIsDark.value : theme.value === DomainTheme.ThemeDark
)

let running: ViewTransition | null = null

// The palette is one class on <html> and nothing else. Both classes are written, so a rule that has
// to answer "which way is this window" finds an answer without a third state to think about.
function paint(dark: boolean) {
  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.classList.toggle('light', !dark)
}

// The change is a cross-fade: the window is snapshotted as it is, the class moves, and the engine
// fades the two snapshots through each other — see `::view-transition-*` in style.css. An engine
// without the API (an older WKWebView) simply changes the palette.
function apply(dark: boolean, animate: boolean) {
  const start = (document as TransitionDocument).startViewTransition?.bind(document)
  if (!animate || !start) {
    paint(dark)
    return
  }
  // A choice made while a fade is still running starts its own: waiting for the one on screen would
  // leave the click unanswered for the rest of it.
  running?.skipTransition()
  running = start((): void => paint(dark))
}

function persist(next: Theme) {
  try {
    localStorage.setItem(THEME_STORAGE_KEY, next)
  } catch {
    // Quota or unavailable storage: the choice still holds for this session.
  }
}

// A choice is taken, and the palette follows it. The answer is whether the palette moved: choosing
// the one already on screen — «системная» under a dark system while the app is dark — is not a change
// to fade, and the switch shows it on its own, because it reads the choice and not the paint.
function choose(next: Theme): boolean {
  const before = isDark.value
  theme.value = next
  persist(next)
  return isDark.value !== before
}

systemPrefersDark.addEventListener('change', (e) => {
  systemIsDark.value = e.matches
  if (theme.value === DomainTheme.ThemeSystem) apply(e.matches, true)
})

// The About window is a separate app, and it follows along on this event: a change made in either
// window reaches both. Go is where the choice lives; the cache below only serves the next first frame.
Events.On('settings:theme', (ev) => {
  const next = (ev.data as { theme: Theme }).theme
  // The window that made the choice has painted it already; this news is for the other one.
  if (!next || next === theme.value) return
  if (choose(next)) apply(isDark.value, true)
})

// At boot there is nothing to fade from, and the stored choice replaces what the URL carried only
// if the two disagree — which happens when a window was created before the theme was changed.
const { settings, loadSettings, setTheme: saveTheme } = useSettings()

void loadSettings().then(() => {
  const stored = settings.value?.theme
  if (stored && stored !== theme.value) {
    theme.value = stored
    persist(stored)
    apply(isDark.value, false)
  }
})

apply(isDark.value, false)

export function useTheme() {
  return {
    theme,
    isDark,
    setTheme(next: Theme) {
      const fading = choose(next)
      // Go is told first: re-tinting a window's material makes Windows repaint the window's own frame
      // and shadow, and a repaint that arrives after the fade reads as the window blinking. The fade
      // waits for the round trip, which is a few milliseconds.
      void saveTheme(next).finally(() => {
        if (fading) apply(isDark.value, true)
      })
    },
  }
}
