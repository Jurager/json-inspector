import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue'

// Dragging a row of the collection tree: which row is being carried, where it would land, and what
// that means once the pointer is let go.
//
// It is written on pointer events rather than the platform's own drag and drop for the reason the
// rest of the app is: the drop target is a place between two rows, which the platform cannot name,
// and a drag image the browser draws cannot be styled. The gesture is the one a resize handle already
// uses here — a press, then listeners on the window until the release.

/** One row as the drag sees it: what it is, where it sits, and what the two mean together. */
export interface DraggableRow {
  id: string
  kind: 'collection' | 'request'
  name: string
  method: string
  depth: number
  /** The collection the row sits in — the empty id for a row at the top of the tree. */
  collectionId: string
  /** Its place among the rows of that level, counted as the level looks now. */
  position: number
}

export type DropZone = 'before' | 'after' | 'inside'

export interface DropTarget {
  id: string
  zone: DropZone
}

/** The row under the pointer, with the place in it the drop would take. */
export interface DragState {
  row: DraggableRow
  target: DropTarget | null
  x: number
  y: number
}

// How far the pointer has to travel before a press becomes a carry. The rows answer clicks, double
// clicks and their own context menu, and a hand that shakes by a pixel while pressing must not turn
// any of them into a move.
const THRESHOLD = 4

// How long a pointer has to rest on a closed collection before it opens for the row being carried.
const OPEN_AFTER = 500

export function useTreeDrag<R extends DraggableRow = DraggableRow>(options: {
  /** The rows on screen, in the order they are drawn. */
  rows: () => R[]
  /** The element a row is drawn in, or null when it is not on screen. */
  elementOf: (id: string) => HTMLElement | null
  /** Whether a collection would take a row dropped into it: never itself, never one of its own. */
  accepts: (dragged: R, collectionId: string) => boolean
  open: (id: string) => void
  /** Where it goes: the component owns the tree, so it is the one that can turn a place into an
   *  index in a level. */
  move: (dragged: R, target: DropTarget) => void
}) {
  // Typed rather than inferred: a generic row is deep-unwrapped by , and what comes back out is
  // no longer the row type the callbacks were given.
  const drag = ref(null) as Ref<{ row: R; target: DropTarget | null; x: number; y: number } | null>

  let pending: { row: R; x: number; y: number } | null = null
  let opening: { id: string; timer: number } | null = null

  function clearOpening() {
    if (!opening) return
    window.clearTimeout(opening.timer)
    opening = null
  }

  // Where the pointer is over one row: the top quarter is before it, the bottom quarter is after it,
  // and the middle is inside it — which only a collection takes, because a request holds nothing.
  function zoneOf(rect: DOMRect, y: number, draggable: boolean): DropZone | null {
    const quarter = rect.height / 4
    if (y < rect.top + quarter) return 'before'
    if (y > rect.bottom - quarter) return 'after'
    return draggable ? 'inside' : null
  }

  function targetAt(x: number, y: number): DropTarget | null {
    const carrying = drag.value
    if (!carrying) return null

    for (const row of options.rows()) {
      if (row.id === carrying.row.id) continue
      const element = options.elementOf(row.id)
      if (!element) continue

      const rect = element.getBoundingClientRect()
      if (y < rect.top || y > rect.bottom) continue
      if (x < rect.left || x > rect.right) continue

      // Only a collection is a place to be dropped into, and only when it would take the row; a
      // collection that would not is still a place to be dropped beside.
      const into = row.kind === 'collection' && options.accepts(carrying.row, row.id)
      const zone = zoneOf(rect, y, into)
      if (!zone) return null

      return { id: row.id, zone }
    }
    return null
  }

  function onMove(e: PointerEvent) {
    if (pending && !drag.value) {
      const travelled = Math.abs(e.clientX - pending.x) + Math.abs(e.clientY - pending.y)
      if (travelled < THRESHOLD) return
      drag.value = { row: pending.row, target: null, x: e.clientX, y: e.clientY }
      pending = null
      document.body.style.userSelect = 'none'
      document.body.style.cursor = 'grabbing'
    }
    if (!drag.value) return

    drag.value = { ...drag.value, x: e.clientX, y: e.clientY, target: targetAt(e.clientX, e.clientY) }

    // A row carried over a closed collection that would take it opens it, so that the pointer can
    // reach what is inside without letting go and starting again.
    const target = drag.value.target
    if (opening && (!target || target.id !== opening.id)) clearOpening()
    if (target && target.zone !== 'inside') clearOpening()
    if (target && target.zone === 'inside' && !opening) {
      const id = target.id
      opening = { id, timer: window.setTimeout(() => options.open(id), OPEN_AFTER) }
    }
  }

  function endDrag(dropped: boolean) {
    clearOpening()
    pending = null
    document.body.style.userSelect = ''
    document.body.style.cursor = ''

    const carrying = drag.value
    drag.value = null
    if (!dropped || !carrying || !carrying.target) return

    options.move(carrying.row, carrying.target)
  }

  // A press and a release on the same row are a click, and a carry is not: the click the browser sends
  // after one is what would select the row the user just moved, so it is answered once and dropped.
  let swallowed = false

  function swallowClick(): boolean {
    const was = swallowed
    swallowed = false
    return was
  }

  function onUp() {
    // The release itself carries the target, so whatever the pointer last hovered is what it lands on.
    swallowed = drag.value !== null
    endDrag(true)
  }

  function onCancel() {
    endDrag(false)
  }

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape' && (drag.value || pending)) endDrag(false)
  }

  // What a row's pointerdown hands over: the press is remembered, and whether it becomes a carry is
  // decided by how far the pointer then travels.
  function press(e: PointerEvent, row: R) {
    if (e.button !== 0) return
    pending = { row, x: e.clientX, y: e.clientY }
  }

  onMounted(() => {
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
    window.addEventListener('pointercancel', onCancel)
    window.addEventListener('keydown', onKeyDown)
  })

  onBeforeUnmount(() => {
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    window.removeEventListener('pointercancel', onCancel)
    window.removeEventListener('keydown', onKeyDown)
    endDrag(false)
  })

  return { drag, press, endDrag, swallowClick }
}
