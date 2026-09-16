import { nextTick, onBeforeUnmount, onMounted, watch } from 'vue'

// Arrow keys over the list a panel draws. ↓ and ↑ move the selection to the row above or below, which
// is what a click does there: the panels of this window drive the pane beside them, so "move" and
// "open" are one gesture. What the rows are — a tree, tab groups, a filtered list — is the panel's own
// business, and it is the panel that answers with the ids it is drawing right now, in its own order.
//
// ← and → are handed back to the panel: a tree has something to do with them, a flat list has not.
export interface ListKeys {
  // The rows on screen, in the order they are drawn.
  ids(): string[]
  // What is selected, so a move starts from where the user is looking.
  current(): string | null
  move(id: string): void
  // The selector of the row that is selected, so the list can scroll to it when it moves.
  selected: string
  // The keys the panel wants for itself. Answering false lets the key through untouched.
  onSideKey?(key: 'ArrowLeft' | 'ArrowRight', id: string | null): boolean
}

export function useListKeys(keys: ListKeys): void {
  function onKeydown(e: KeyboardEvent) {
    // A chord belongs to the window's shortcuts, and a list is not walked with a modifier held.
    if (e.metaKey || e.ctrlKey || e.altKey || e.shiftKey) return
    // A field the user is typing in owns every key that reaches it.
    if (typing(e.target) || covered()) return

    const key = e.key
    if (key === 'ArrowLeft' || key === 'ArrowRight') {
      if (keys.onSideKey?.(key, keys.current()) && e.cancelable) e.preventDefault()
      return
    }
    if (key !== 'ArrowDown' && key !== 'ArrowUp') return

    const list = keys.ids()
    if (list.length === 0) return
    const at = keys.current() ? list.indexOf(keys.current() as string) : -1
    const next = neighbour(list, at, key === 'ArrowDown' ? 1 : -1)
    if (next < 0) return

    keys.move(list[next])
    if (e.cancelable) e.preventDefault()
  }

  // A row that walks off the bottom of the panel is a row the user cannot see; the list follows the
  // selection the way it follows a click. It waits for the answer rather than scrolling on the spot:
  // opening a row can be deferred — a card with unsaved edits asks first.
  watch(
    () => keys.current(),
    () => {
      void nextTick(() => {
        document.querySelector<HTMLElement>(keys.selected)?.scrollIntoView({ block: 'nearest' })
      })
    }
  )

  onMounted(() => window.addEventListener('keydown', onKeydown))
  onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
}

// Where the move lands, or -1 when there is nothing that way. A list with nothing selected is entered
// from the end the key came from, which is what the design of every list does.
function neighbour(list: string[], at: number, step: number): number {
  if (at < 0) return step > 0 ? 0 : list.length - 1
  const next = at + step
  return next < 0 || next >= list.length ? -1 : next
}

function typing(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null
  if (!el || !el.tagName) return false
  return el.isContentEditable || el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT'
}

// Whatever stands over the list owns its arrows: a dialog, a context menu, a list of its own.
function covered(): boolean {
  return !!document.querySelector('[role="dialog"],[role="alertdialog"],[role="menu"],[role="listbox"]')
}
