import { computed, ref } from 'vue'
import { System } from '@wailsio/runtime'

type Platform = 'darwin' | 'windows' | 'linux'

// window._wails.environment is injected asynchronously, so sampling it at import time would
// latch "not Windows" for the session: the user agent seeds the platform instead.
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

// Windows and Linux run frameless with their own caption buttons; must agree with
// useCustomTitlebar() in window.go, which decides Frameless.
export const useCustomTitlebar = computed(
  () => platform.value === 'windows' || platform.value === 'linux'
)

// "⌘F" on macOS, "Ctrl+F" elsewhere — pass just the key.
export function shortcut(key: string): string {
  return isMac.value ? `⌘${key}` : `Ctrl+${key}`
}
