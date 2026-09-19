import { nextTick, ref, shallowRef, type ComponentPublicInstance } from 'vue'

// A name typed where the row will be: the tree renames a row and creates a request the same way — a
// field appears in place, opens selected, and Enter writes it while Escape drops it. Both had written
// that machine out whole, down to the invalid flag, the next-frame focus and the blur that commits.
//
// `target` is whatever the name is about — a row's id, the collection a request is joining — and
// nothing here looks inside it: the write is the caller's, because only the caller knows whether it
// is a rename or a creation.
export function useInlineName<T>(write: (target: T, name: string) => Promise<void>) {
  const target = shallowRef<T | null>(null)
  const draft = ref('')
  const invalid = ref(false)
  const input = ref<HTMLInputElement | null>(null)

  function setInput(el: Element | ComponentPublicInstance | null) {
    input.value = (el as HTMLInputElement | null) ?? null
  }

  // A field that appears where a menu item was clicked cannot be focused in the same tick: the menu
  // is still closing, and whatever it does with focus on the way out lands after this. The next frame
  // is the first moment the caret stays where it was put.
  function open(at: T, initial: string) {
    target.value = at
    draft.value = initial
    invalid.value = false
    nextTick(() => {
      requestAnimationFrame(() => input.value?.focus())
      input.value?.select()
    })
  }

  async function commit() {
    const at = target.value
    if (at === null) return
    const name = draft.value.trim()
    if (!name) {
      // An empty name is refused by Go, and refusing it here keeps the field open on the text instead
      // of closing it and letting a call fail.
      invalid.value = true
      input.value?.focus()
      return
    }
    target.value = null
    invalid.value = false
    await write(at, name)
  }

  function cancel() {
    target.value = null
    invalid.value = false
  }

  function onKeydown(e: KeyboardEvent) {
    e.stopPropagation()
    if (e.key === 'Enter') {
      e.preventDefault()
      void commit()
    } else if (e.key === 'Escape') {
      e.preventDefault()
      cancel()
    }
  }

  return { target, draft, invalid, input, setInput, open, commit, cancel, onKeydown }
}
