// The colours an environment can wear. Go stores the word — which colour it is, is the interface's
// business — and this is where a word becomes one. Tokens, not values: the dark theme reads the same
// four names and gets its own shades.
//
// The tints are the design system's dark tones rather than its bright fills, because the same colour
// is drawn behind the white initials of a rail avatar. The green, orange and red ones are the tokens
// the window already writes text in; purple has no twin of that family, so it is mixed down to the
// same depth instead of being added as a fifth name.
export const ENVIRONMENT_TINTS: Record<string, string> = {
  green: 'var(--green-text)',
  orange: 'var(--orange-text)',
  red: 'var(--red-text)',
  purple: 'color-mix(in srgb, var(--purple) 72%, var(--text))',
}

// The order the swatches are offered in.
export const ENVIRONMENT_COLORS = ['green', 'orange', 'red', 'purple'] as const

// What a new environment is made in. A colour is always chosen, so no avatar has to fall back — the
// drawing has none that is not filled.
export const DEFAULT_ENVIRONMENT_COLOR = 'green'

// The globals are not an environment and hold no colour of their own; the drawing tints that one row
// purple, and purple is what a scope that applies everywhere wears.
export const GLOBALS_COLOR = 'purple'

// The fill behind an avatar's initials, and the circle of a swatch.
export function tintOf(color: string): string {
  return ENVIRONMENT_TINTS[color] ?? ENVIRONMENT_TINTS[DEFAULT_ENVIRONMENT_COLOR]
}

// The 7px mark the titlebar draws beside the active environment's name: a chip's mark and not a fill
// behind letters, so it takes the bright fill the window uses for a state anywhere else.
const DOTS: Record<string, string> = {
  green: 'var(--green)',
  orange: 'var(--orange)',
  red: 'var(--red)',
  purple: 'var(--purple)',
}

export function dotOf(color: string): string {
  return DOTS[color] ?? DOTS[DEFAULT_ENVIRONMENT_COLOR]
}

// The letter an avatar shows. Spread rather than charAt: a name that begins with anything outside the
// basic plane is one character and not half of two.
export function initialOf(name: string): string {
  return [...name][0]?.toUpperCase() ?? ''
}
