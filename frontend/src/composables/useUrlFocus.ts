// The command line is remounted whenever the request view comes back, so a request for
// its focus has to survive that: it is remembered until a field exists to take it.
let takeFocus: (() => void) | null = null
let pending = false

export function requestUrlFocus() {
  if (takeFocus) takeFocus()
  else pending = true
}

/** Registers the field that can take the focus; returns the unregister. */
export function provideUrlFocus(onFocus: () => void): () => void {
  takeFocus = onFocus
  if (pending) {
    pending = false
    onFocus()
  }
  return () => {
    takeFocus = null
  }
}
