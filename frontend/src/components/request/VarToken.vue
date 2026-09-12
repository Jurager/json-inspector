<script setup lang="ts">
import { computed, ref } from 'vue'
import { useEnvironmentsStore } from '../../stores/environments'
import { isMac } from '../../lib/platform'
import { useHoverArrival } from '../../composables/useHoverArrival'
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

// Portalled by the hint: the field clips its overflow for the ellipsis, so a
// nested panel would be cut off with it.
const root = ref<HTMLElement | null>(null)

// A token can appear under a resting pointer while the URL is being typed.
const hintArmed = useHoverArrival(root)

function onClick(e: MouseEvent) {
  if (e.altKey) {
    // Alt-click opens the sheet; the field must not read the same click as
    // "start editing".
    e.preventDefault()
    e.stopPropagation()
    const r = resolution.value
    store.openSheet({
      envId: r?.source === 'env' ? store.activeId : null,
      varName: props.name,
    })
    return
  }

  // Something is selected: that was a double click or a drag, and the selection
  // is the point of the gesture.
  if (window.getSelection()?.toString()) return

  const input = siblingInput(root.value?.parentElement ?? null)
  if (!input) return

  // One atom of text: the caret goes to the token's start, not inside a name
  // whose braces aren't editable.
  e.preventDefault()
  input.focus()
  const at = props.offset ?? input.value.length
  input.setSelectionRange(at, at)
}

// The highlight layer covers a real input, so a click on a token never reaches
// it: walk up to the field that holds both.
function siblingInput(from: HTMLElement | null): HTMLInputElement | null {
  let node: HTMLElement | null = from
  while (node) {
    const found = node.querySelector('input')
    if (found) return found
    node = node.parentElement
  }
  return null
}

// Nothing is prevented on mousedown: preventDefault is what stops the browser
// from starting a selection, and then the token couldn't be selected or copied
// as the text it looks like. The caret gesture is decided in onClick instead.
</script>

<template>
  <Tooltip class="var-tip" :disabled="!hintArmed">
    <template #trigger>
      <span
        ref="root"
        class="var-token"
        :class="{ unknown: !known, secret: isSecret }"
        @click="onClick"
        @pointerleave="hintArmed = true"
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

/* The surrounding display layer is pointer-events:none so the input underneath
   keeps native caret and selection behaviour; tokens are the parts that opt back
   in, because they have a hover state and a click action. */
.var-token {
  /* select-text: the body is select-none, and a double click is the natural way
     to grab the variable's name. */
  @apply rounded-sm cursor-text select-text;
  pointer-events: auto;
  padding: 1px 4px;
  background: var(--accent-soft);
  color: var(--accent);
}

/* Not found in the environment or in globals: the app's red for "this request
   can't go out". */
.var-token.unknown {
  background: var(--red-soft);
  color: var(--red);
  text-decoration: underline wavy;
  text-underline-offset: 3px;
}

/* Layout only: the plaque belongs to ui/Tooltip. */
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
