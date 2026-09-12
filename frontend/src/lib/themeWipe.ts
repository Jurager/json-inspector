// The theme change is a circle growing from the window's top-left corner: `@keyframes theme-wipe`
// in style.css runs `circle(0% at 0 0)` → `circle(150% at 0 0)` over WIPE_MS on these control
// points, and a circle's radius in percent is of hypot(width, height) / sqrt(2).
export const WIPE_MS = 850

// The switch's own reveal (pill-slide, theme-glyphs-*) keeps running past the wipe's own end —
// useTheme's fallback timer adds this on top of WIPE_MS. Duplicated as pill-slide's 0.3s in style.css.
export const SWITCH_TAIL_MS = 300

const EASING = [0.35, 0.25, 0.2, 1] as const
const RADIUS = 1.5

// One axis of the cubic-bezier at parameter u.
function axis(u: number, a: number, b: number): number {
  const rest = 1 - u
  return 3 * rest * rest * u * a + 3 * rest * u * u * b + u * u * u
}

/**
 * How far into the wipe (ms) the fill's front passes the point measured from the window's
 * top-left corner. Whoever has to change with the fill — the switch's pill, its glyphs — waits for
 * this moment: a fixed delay is in step in one window size and out of step in every other.
 */
export function fillArrival(x: number, y: number, width: number, height: number): number {
  const span = (RADIUS * Math.hypot(width, height)) / Math.SQRT2
  const needed = Math.hypot(x, y) / span
  if (needed >= 1) return WIPE_MS

  // The curve goes from time to distance; walking it backwards is a bisection on its parameter
  // (both axes are monotone), then the other axis gives the time.
  const [x1, y1, x2, y2] = EASING
  let lo = 0
  let hi = 1
  for (let i = 0; i < 32; i++) {
    const mid = (lo + hi) / 2
    if (axis(mid, y1, y2) < needed) lo = mid
    else hi = mid
  }
  return Math.round(axis((lo + hi) / 2, x1, x2) * WIPE_MS)
}
