// The five tints a workspace can wear, as the mockup draws them. The names are what Go stores — the
// palette is the interface's business, so the row holds a word and this is where the word becomes a
// colour. Tokens, not values: the dark theme reads the same five names and gets its own shades.
export const WORKSPACE_TINTS: Record<string, string> = {
  blue: 'var(--accent)',
  purple: 'var(--purple)',
  green: 'var(--green)',
  orange: 'var(--orange)',
  grey: 'var(--text-tertiary)',
}

// The order the swatches are offered in.
export const WORKSPACE_COLORS = ['blue', 'purple', 'green', 'orange', 'grey'] as const

// A workspace nobody has dressed wears no colour, and that is a state rather than a missing value:
// the avatar goes the neutral grey the mockup draws the personal space in, and the window's glass
// stays the tone it has always had. Every other word is the palette's.
export function tintOf(color: string): string {
  return WORKSPACE_TINTS[color] ?? WORKSPACE_TINTS.grey
}

// hasTint says whether the window's glass should take the workspace's colour at all. Empty is the
// answer for the default workspace and for anything made before the colour was asked for.
export function hasTint(color: string): boolean {
  return color in WORKSPACE_TINTS
}
