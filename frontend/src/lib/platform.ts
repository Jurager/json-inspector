import { computed, ref } from 'vue'
import { Environment } from '../../wailsjs/runtime/runtime'

// A single, reliable source for "which OS is this" — sourced from Wails'
// own Environment() call rather than user-agent sniffing, so every part of
// the UI that needs to look different per platform (shortcut hints, the
// custom titlebar) agrees with each other. Resolved once at module load;
// starts as macOS (the safe default — a wrong guess here just shows "⌘"
// briefly instead of "Ctrl" while Environment() resolves, on this app's own
// home platform).
const platform = ref<'darwin' | 'windows' | 'linux' | ''>('darwin')

Environment()
  .then((env) => {
    platform.value = env.platform as typeof platform.value
  })
  .catch(() => {
    // keep the default
  })

export const isMac = computed(() => platform.value === 'darwin')

// Windows and Linux have no equivalent of macOS's hidden-inset title bar, so
// those platforms run frameless and draw their own titlebar/caption buttons.
export const useCustomTitlebar = computed(() => platform.value === 'windows' || platform.value === 'linux')

// "⌘F" on macOS, "Ctrl+F" elsewhere — pass just the key, e.g. shortcut('F')
// or shortcut('↵').
export function shortcut(key: string): string {
  return isMac.value ? `⌘${key}` : `Ctrl+${key}`
}
