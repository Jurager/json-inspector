import { computed, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { useSettings } from './useSettings'
import { Language as DomainLanguage } from '../../bindings/json-inspector/internal/domain'
import { i18n, systemLocale, t, type Locale } from '../i18n'
import { SystemService } from '../../bindings/json-inspector/internal/transport/wails'

// The choice is declared once, in Go, and reaches the window as a generated enum.
export type Language = DomainLanguage

const LANGUAGE_STORAGE_KEY = 'ji-lang-v1'

/** Reads a stored or URL-supplied value; anything else — a newer build's language — is not one. */
function asLanguage(raw: string | null): Language | null {
  switch (raw) {
    case 'ru':
      return DomainLanguage.LanguageRU
    case 'en':
      return DomainLanguage.LanguageEN
    case 'system':
      return DomainLanguage.LanguageSystem
    default:
      return null
  }
}

function toLocale(language: Language): Locale {
  return language === DomainLanguage.LanguageRU ? 'ru' : 'en'
}

/** The choice Go put on the window's URL — the language its first frame is already written in. */
function urlLanguage(): Language | null {
  return asLanguage(new URLSearchParams(location.search).get('lang'))
}

/** The cache a dev reload falls back on, since a reload carries no query string. */
function cachedLanguage(): Language {
  try {
    return asLanguage(localStorage.getItem(LANGUAGE_STORAGE_KEY)) ?? DomainLanguage.LanguageSystem
  } catch {
    return DomainLanguage.LanguageSystem
  }
}

const language = ref<Language>(urlLanguage() ?? cachedLanguage())

// The system's own language, read once. Unlike the theme's `prefers-color-scheme` the webview raises no
// event for it changing under a running window, and asking again would only repeat the same answer.
const systemLanguage = systemLocale()

const locale = computed<Locale>(() =>
  language.value === DomainLanguage.LanguageSystem ? systemLanguage : toLocale(language.value)
)

// Two surfaces are drawn outside the page — the native menu, by the system, and the page a sign-in
// puts in the browser, by Go — and neither can read the catalogue. Go cannot resolve "system" either.
// So their words are the one thing handed over instead of read, and they go together: they are needed
// at the same two moments, as the window learns the language and whenever it moves.
function applyLanguage() {
  void SystemService.ApplyLanguage(
    {
      about: t('rail.about'),
      help: t('menu.help'),
      checkUpdates: t('menu.checkUpdates'),
    },
    {
      waiting: { title: t('signIn.waiting.title'), text: t('signIn.waiting.text') },
      done: { title: t('signIn.done.title'), text: t('signIn.done.text') },
      refused: { title: t('signIn.refused.title'), text: t('signIn.refused.text') },
      failed: t('signIn.failed'),
    },
  )
}

function persist(next: Language) {
  try {
    localStorage.setItem(LANGUAGE_STORAGE_KEY, next)
  } catch {
    // Quota or unavailable storage: the choice still holds for this session.
  }
}

// A choice is taken, and the window is redrawn in it. Both halves of that are written every time: the
// catalogue the messages come from, and the document's own tag — which is what a screen reader and
// the spell checker go by. Both catalogues are already in the bundle, so this is a redraw and nothing
// has to be fetched.
function choose(next: Language) {
  language.value = next
  i18n.global.locale.value = locale.value
  document.documentElement.lang = locale.value
  persist(next)
  applyLanguage()
}

// The About window is a separate app, and it follows along on this event: a change made in either
// window reaches both. Go is where the choice lives; the cache below only serves the next first frame.
Events.On('settings:language', (ev) => {
  const next = (ev.data as { language: Language }).language
  // The window that made the choice has redrawn already; this news is for the other one.
  if (!next || next === language.value) return
  choose(next)
})

// At boot the stored choice replaces what the URL carried only if the two disagree — which happens
// when a window was created before the language was changed.
const { settings, loadSettings, setLanguage: saveLanguage } = useSettings()

void loadSettings().then(() => {
  const stored = settings.value?.language
  if (stored && stored !== language.value) choose(stored)
})

// The pre-paint script has already put the language on the document; this makes the two agree when
// the window's URL carried nothing.
document.documentElement.lang = locale.value

// And Go is told the words it cannot look up itself.
applyLanguage()

export function useLocale() {
  return {
    /** The choice, as the settings screen shows it. */
    language,
    /** What the choice resolves to — what a formatter or a `lang` attribute needs. */
    locale,
    setLanguage(next: Language) {
      const before = locale.value
      choose(next)
      // Go is told after the redraw, the other way round from the theme: there is no material to
      // re-tint and no fade to coordinate with, so the window should never wait on the round trip.
      void saveLanguage(next).catch(() => {
        // Nothing to do: the window is already in the chosen language.
      })
      return before !== locale.value
    },
  }
}
