<script setup lang="ts">
import { computed, ref } from 'vue'
import { useEnvironmentsStore } from '../../stores/environments'
import { isMac } from '../../lib/platform'
import { Tooltip } from '../ui/tooltip'

const props = defineProps<{ name: string; offset?: number }>()

const store = useEnvironmentsStore()

const resolution = computed(() => store.resolve(props.name))
const known = computed(() => resolution.value !== null)
const isSecret = computed(() => resolution.value?.kind === 'secret')

const label = computed(() => `{{${props.name}}}`)

const scopeLabel = computed(() => {
  const r = resolution.value
  if (!r) return ''
  return r.source === 'env' ? (store.active?.name ?? 'Окружение') : 'Глобальные'
})

const modifier = computed(() => (isMac ? '⌥клик' : 'Alt+клик'))

// The hint is a ui/Tooltip now: it portals itself out of the field, which is
// what the hand-rolled version needed fixed coordinates for — the field clips
// its overflow for the ellipsis, and a nested panel would be cut off with it.
const root = ref<HTMLElement | null>(null)

function onClick(e: MouseEvent) {
  if (e.altKey) {
    // Stop the parent field from also treating this as "start editing" — the
    // two actions share one click.
    e.preventDefault()
    e.stopPropagation()
    const r = resolution.value
    store.openSheet({
      envId: r?.source === 'env' ? store.activeId : null,
      varName: props.name,
    })
    return
  }

  // The browser selected something: that was a double click or a drag, and the
  // selection is the whole point of it.
  if (window.getSelection()?.toString()) return

  const input = siblingInput(root.value?.parentElement ?? null)
  if (!input) return
  // One atom of text: the caret goes to the token's start rather than inside a
  // name whose braces the user can't see or edit.
  e.preventDefault()
  input.focus()
  const at = props.offset ?? input.value.length
  input.setSelectionRange(at, at)
}

// The highlight layer sits on top of a real input, so a click that lands on a
// token never reaches it. Find the input by walking up — the token is always
// rendered inside the same field/cell as the input it belongs to.
function siblingInput(from: HTMLElement | null): HTMLInputElement | null {
  let node: HTMLElement | null = from
  while (node) {
    const found = node.querySelector('input')
    if (found) return found
    node = node.parentElement
  }
  return null
}

// Nothing is prevented on mousedown, and that is the point: preventDefault is
// what stops the browser from ever starting a text selection, so a token could
// not be selected or copied as the text it looks like — not by dragging, not by
// double clicking. Dragging on plain text around it works for the same reason
// the layer is transparent to the mouse.
//
// The caret gesture happens in onClick instead, once we can tell the two apart:
// a click that left a selection behind was a selection gesture, anything else
// means "put the caret here".
</script>

<template>
  <Tooltip class="var-tip">
    <template #trigger>
      <span
        ref="root"
        class="var-token"
        :class="{ unknown: !known, secret: isSecret }"
        @click="onClick"
        >{{ label }}</span
      >
    </template>

    <div class="var-tip-value">
      {{ isSecret ? 'значение скрыто · секрет' : resolution?.value || '(пусто)' }}
    </div>
    <div class="var-tip-meta">{{ scopeLabel }} → {{ name }} · {{ modifier }}, чтобы открыть в редакторе</div>
  </Tooltip>
</template>

<style scoped>
@reference "../../style.css";

/* Interactive while the layer around it is not: the parent display is
   pointer-events:none so the input underneath keeps native caret and selection
   behaviour, and only the tokens opt back in — they're the parts that have a
   hover state and a click action. */
.var-token {
  /* select-text: the body carries select-none, so without this the token can't
     be selected or copied — and a double click on it is the natural way to grab
     the variable's name. */
  @apply rounded-sm cursor-text select-text;
  pointer-events: auto;
  padding: 1px 4px;
  background: var(--accent-soft);
  color: var(--accent);
}

.var-token.secret {
  background: var(--accent-soft);
  color: var(--accent);
}

/* Not found in the active environment or in globals: the same red the rest of
   the app uses for "this request can't go out". */
.var-token.unknown {
  background: var(--red-soft);
  color: var(--red);
  text-decoration: underline wavy;
  text-underline-offset: 3px;
}

/* Layout only: the plaque itself (inverted card, padding, shadow) is ui/Tooltip's,
   this just stacks the value over the scope line. */
.var-tip {
  @apply flex flex-col gap-[3px];
}

.var-tip-value {
  @apply text-[11.5px] break-all;
  font-family: var(--mono);
}

.var-tip-meta {
  @apply text-[11px];
  color: color-mix(in srgb, var(--tip-text) 60%, transparent);
}
</style>
