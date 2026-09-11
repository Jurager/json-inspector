import { computed, ref } from 'vue'
import { System } from '@wailsio/runtime'

type Platform = 'darwin' | 'windows' | 'linux'

// The host injects window._wails.environment asynchronously: measured on
// Windows it is still undefined while the module graph evaluates and only
// appears afterwards. So System.IsWindows() cannot be sampled at import time —
// sampling it would latch "not Windows" for the session.
//
// The user agent, by contrast, is available synchronously and is right: WebView2
// reports "Windows NT", WKWebView reports "Macintosh", WebKitGTK reports "Linux".
// It seeds the value so the very first paint is correct; Environment() then has
// the final say. v2 avoided sniffing to keep every part of the UI agreeing with
// each other — that still holds, because the seed is replaced by the authority
// rather than competing with it.
function detect(): Platform {
  const ua = navigator.userAgent
  if (/Windows/i.test(ua)) return 'windows'
  if (/Android/i.test(ua)) return 'linux'
  if (/Linux|X11/i.test(ua)) return 'linux'
  return 'darwin'
}

const platform = ref<Platform>(detect())

// Refinement, not the source of truth: if this call fails or times out the
// seed above stands. Leaving the platform unknown instead would mean a frameless
// window with no caption buttons — i.e. no way to close it.
System.Environment()
  .then((env) => {
    if (env.OS) platform.value = env.OS as Platform
  })
  .catch(() => {
    // Keep the detected value.
  })

export const isMac = computed(() => platform.value === 'darwin')

// Windows and Linux have no equivalent of macOS's hidden-inset title bar, so
// those platforms run frameless and draw their own titlebar/caption buttons.
// Must agree with useCustomTitlebar() in window.go, which decides Frameless.
export const useCustomTitlebar = computed(
  () => platform.value === 'windows' || platform.value === 'linux'
)

// "⌘F" on macOS, "Ctrl+F" elsewhere — pass just the key, e.g. shortcut('F').
export function shortcut(key: string): string {
  return isMac.value ? `⌘${key}` : `Ctrl+${key}`
}
