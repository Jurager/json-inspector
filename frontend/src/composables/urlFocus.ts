// The command line is remounted whenever the request view comes back, so a request for
// its focus has to survive that: it is remembered until a field exists to take it.
// Not a hook but a module singleton, so it lives with the other composables; stores ask
// for the focus as well, which a `use*` file would suggest they can't.
let takeFocus: (() => void) | null = null
let focusPending = false

export function focusUrlField() {
  if (takeFocus) takeFocus()
  else focusPending = true
}

/** Registers the field that can take the focus; returns the unregister. */
export function registerUrlField(onFocus: () => void): () => void {
  takeFocus = onFocus
  if (focusPending) {
    focusPending = false
    onFocus()
  }
  return () => {
    takeFocus = null
  }
}
