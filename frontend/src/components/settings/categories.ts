// The categories of the settings window, in the drawing's order: what each one is called, which glyph
// stands on its tile, and the colour of that tile.
//
// The colours are the window's own palette rather than the six literal hexes the drawing uses for
// them: a literal would be the one surface that never follows the theme. Two of the drawing's eight
// hues have no token — the teal of Export and the violet of Updates — and the violet goes to the
// purple the palette already has. The actions below are a visual index and not a code.
export type CategoryId =
  | 'account'
  | 'general'
  | 'appearance'
  | 'requests'
  | 'proxy'
  | 'security'
  | 'export'
  | 'updates'

export type Category = {
  id: CategoryId
  icon: string
  tile: string
  /**
   * Drawn and not offered: the design gives these a place and the app has nothing to put in them —
   * no proxy in the engine, no keychain, no export formats. Their rows are the drawing's, with every
   * control switched off and a sentence saying why.
   */
  soon?: boolean
}

export const CATEGORIES: readonly Category[] = [
  { id: 'account', icon: 'user', tile: 'var(--accent)' },
  { id: 'general', icon: 'settings-2', tile: 'var(--text-tertiary)' },
  { id: 'appearance', icon: 'contrast', tile: 'var(--purple)' },
  { id: 'requests', icon: 'exchange', tile: 'var(--green)' },
  { id: 'proxy', icon: 'shield', tile: 'var(--orange)', soon: true },
  { id: 'security', icon: 'lock', tile: 'var(--red)', soon: true },
  { id: 'export', icon: 'upload', tile: 'var(--teal)', soon: true },
  { id: 'updates', icon: 'refresh', tile: 'var(--purple)' },
]

// The live four are the ones with a meta of their own; the other four all say the same word. Written
// as the union they are, so that the catalogue — which is typed against the keys used in the window —
// catches a category whose words were never written.
type LiveId = 'account' | 'general' | 'appearance' | 'requests' | 'updates'

// A category that is still coming says so instead of describing itself: a sentence about the keychain
// the app will not have is worse than none, and the one word is true of all four of them.
export function nameKey(id: CategoryId): `settings.cat.${CategoryId}` {
  return `settings.cat.${id}`
}

export function metaKey(category: Category): 'settings.soon' | `settings.meta.${LiveId}` {
  return category.soon ? 'settings.soon' : (`settings.meta.${category.id}` as `settings.meta.${LiveId}`)
}

/**
 * A category some other window asked this one to open on. Anything that is not one of the eight is
 * the default: the value arrives from outside, and a window that trusted it would be a window
 * drawing nothing at all.
 */
export function asCategory(value: string | null): CategoryId {
  const found = CATEGORIES.find((category) => category.id === value)
  return found ? found.id : 'general'
}
