import { computed, ref } from 'vue'
import { System } from '@wailsio/runtime'
import { detectPlatform, drawsOwnTitlebar, shortcutFor, type Platform } from '../lib/platform'

// window._wails.environment arrives asynchronously, so the user agent seeds the platform;
// the seed stands if the probe fails — an unknown platform has no caption buttons.
const platform = ref<Platform>(detectPlatform(navigator.userAgent))

System.Environment()
  .then((env) => {
    if (env.OS) platform.value = env.OS as Platform
  })
  .catch(() => {})

export function usePlatform() {
  return {
    isMac: computed(() => platform.value === 'darwin'),
    customTitlebar: computed(() => drawsOwnTitlebar(platform.value)),
    shortcut: (key: string) => shortcutFor(key, platform.value),
  }
}
