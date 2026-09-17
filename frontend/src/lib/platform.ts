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

// Not every chord is the app's on every platform. WebView2 answers Ctrl+R itself, and a reload tears
// the window's runtime context down — the app has been broken that way once already — so running the
// selected level is a macOS chord until the other one can be shown to be interceptable. The menu asks
// this same question before it prints a key beside an item, so a hint cannot outlive its binding.
export function chordAvailable(key: 'N' | 'D' | 'R', platform: Platform): boolean {
  return key !== 'R' || platform === 'darwin'
}

// A key that stands on its own — Enter, Delete — names no modifier, and the two platforms spell it
// differently: a Mac keyboard carries ↩ and ⌫ on the key itself, a PC one says Enter and Delete.
export function keyName(key: 'enter' | 'delete', platform: Platform): string {
  if (platform === 'darwin') return key === 'enter' ? '↩' : '⌫'
  return key === 'enter' ? 'Enter' : 'Delete'
}
