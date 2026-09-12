import { computed, ref } from 'vue'

export type Theme = 'light' | 'dark' | 'system'

const THEME_STORAGE_KEY = 'ji-theme-v1'
const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)')

// Not in TypeScript's DOM lib yet; the WebView is Chromium-based, and the webview on
// macOS may be older Safari — hence the fallback rather than a requirement.
type TransitionDocument = Document & { startViewTransition?: (cb: () => void) => { skipTransition(): void } }

function storedTheme(): Theme {
  const raw = localStorage.getItem(THEME_STORAGE_KEY)
  return raw === 'light' || raw === 'dark' || raw === 'system' ? raw : 'system'
}

const theme = ref<Theme>(storedTheme())
const systemIsDark = ref(systemPrefersDark.matches)
const isDark = computed(() => (theme.value === 'system' ? systemIsDark.value : theme.value === 'dark'))

let running: { skipTransition(): void } | null = null

function paint(dark: boolean) {
  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.classList.toggle('light', !dark)
}

// A theme change is a diagonal wipe (see `.theme-wipe` in style.css): the document is
// mutated inside the transition, so the old palette is snapshotted and the new one is
// revealed over it instead of both changing at once.
function apply(dark: boolean, animate: boolean) {
  const start = (document as TransitionDocument).startViewTransition?.bind(document)
  if (!animate || !start) {
    paint(dark)
    return
  }
  running?.skipTransition()
  running = start(() => paint(dark))
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
    setTheme(next: Theme) {
      theme.value = next
      persist(next)
      apply(isDark.value, true)
    },
  }
}
