// The five tints a workspace can wear, as the mockup draws them. The names are what Go stores — the
// palette is the interface's business, so the row holds a word and this is where the word becomes a
// colour. Tokens, not values: the dark theme reads the same five names and gets its own shades.
const TINTS: Record<string, string> = {
  blue: 'var(--accent)',
  purple: 'var(--purple)',
  green: 'var(--green)',
  orange: 'var(--orange)',
  grey: 'var(--text-tertiary)',
}

// The order the swatches are offered in, and the colour a workspace gets when it was made before
// anyone chose one.
export const WORKSPACE_COLORS = ['blue', 'purple', 'green', 'orange', 'grey'] as const

export function tintOf(color: string): string {
  return TINTS[color] ?? TINTS.blue
}
