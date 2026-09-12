export type Platform = 'darwin' | 'windows' | 'linux'

export function detectPlatform(userAgent: string): Platform {
  if (/Windows/i.test(userAgent)) return 'windows'
  if (/Android/i.test(userAgent)) return 'linux'
  if (/Linux|X11/i.test(userAgent)) return 'linux'
  return 'darwin'
}

// Windows and Linux run frameless with their own caption buttons; must agree with
// useCustomTitlebar() in window.go, which decides Frameless.
export function drawsOwnTitlebar(platform: Platform): boolean {
  return platform === 'windows' || platform === 'linux'
}

// "⌘F" on macOS, "Ctrl+F" elsewhere — pass just the key.
export function shortcutFor(key: string, platform: Platform): string {
  return platform === 'darwin' ? `⌘${key}` : `Ctrl+${key}`
}
