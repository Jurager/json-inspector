import { computed, ref } from 'vue'
import { System } from '@wailsio/runtime'

type Platform = 'darwin' | 'windows' | 'linux'

// The host injects window._wails.environment asynchronously — measured on Windows it
// is still undefined while the module graph evaluates — so System.IsWindows() cannot
// be sampled at import time; sampling it would latch "not Windows" for the session.
// The user agent is available synchronously and is right (WebView2 "Windows NT",
// WKWebView "Macintosh", WebKitGTK "Linux"): it seeds the value so the first paint is
// correct, and Environment() below has the final say.
function detect(): Platform {
  const ua = navigator.userAgent
  if (/Windows/i.test(ua)) return 'windows'
  if (/Android/i.test(ua)) return 'linux'
  if (/Linux|X11/i.test(ua)) return 'linux'
  return 'darwin'
}

const platform = ref<Platform>(detect())

// If this call fails or times out the seed stands — an unknown platform would mean a
// frameless window with no caption buttons, i.e. no way to close it.
System.Environment()
  .then((env) => {
    if (env.OS) platform.value = env.OS as Platform
  })
  .catch(() => {
    // Keep the detected value.
  })

export const isMac = computed(() => platform.value === 'darwin')

// Windows and Linux have no macOS-style hidden-inset title bar, so they run frameless and
// draw their own caption buttons. Must agree with useCustomTitlebar() in window.go, which
// decides Frameless.
export const useCustomTitlebar = computed(
  () => platform.value === 'windows' || platform.value === 'linux'
)

// "⌘F" on macOS, "Ctrl+F" elsewhere — pass just the key.
export function shortcut(key: string): string {
  return isMac.value ? `⌘${key}` : `Ctrl+${key}`
}
