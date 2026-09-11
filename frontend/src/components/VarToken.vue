<script setup lang="ts">
import { computed, ref } from 'vue'
import { useEnvironmentsStore } from '../stores/environments'
import { isMac } from '../lib/platform'

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

// Fixed coordinates from the token's own box rather than an absolutely
// positioned child: the field clips its overflow for the ellipsis, and a
// nested tooltip would be cut off with it.
const tip = ref<{ left: number; top: number } | null>(null)
const root = ref<HTMLElement | null>(null)

function showTip() {
  const r = root.value?.getBoundingClientRect()
  if (!r) return
  tip.value = { left: r.left, top: r.bottom + 6 }
}

function hideTip() {
  tip.value = null
}

function onClick(e: MouseEvent) {
  if (!e.altKey) return
  // Stop the parent field from also treating this as "start editing" — the two
  // actions share one click.
  e.preventDefault()
  e.stopPropagation()
  const r = resolution.value
  store.openSheet({
    envId: r?.source === 'env' ? store.activeId : null,
    varName: props.name,
  })
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

// A token behaves like one atom of text: clicking it puts the caret at its
// start rather than dropping the user inside a name they can't see the braces
// of. Dragging on plain text around it still selects normally, because the
// layer itself is transparent to the mouse.
function onMouseDown(e: MouseEvent) {
  if (e.altKey) return
  const input = siblingInput(root.value?.parentElement ?? null)
  if (!input) return
  e.preventDefault()
  input.focus()
  const at = props.offset ?? input.value.length
  input.setSelectionRange(at, at)
}
</script>

<template>
  <span
    ref="root"
    class="var-token"
    :class="{ unknown: !known, secret: isSecret }"
    @mouseenter="showTip"
    @mouseleave="hideTip"
    @mousedown="onMouseDown"
    @click="onClick"
    >{{ label }}</span
  >

  <Teleport to="body">
    <div v-if="tip" class="var-tip" :style="{ left: tip.left + 'px', top: tip.top + 'px' }">
      <div class="var-tip-value">
        {{ isSecret ? 'значение скрыто · секрет' : resolution?.value || '(пусто)' }}
      </div>
      <div class="var-tip-meta">{{ scopeLabel }} → {{ name }} · {{ modifier }}, чтобы открыть в редакторе</div>
    </div>
  </Teleport>
</template>

<style scoped>
@reference "../style.css";

/* Interactive while the layer around it is not: the parent display is
   pointer-events:none so the input underneath keeps native caret and selection
   behaviour, and only the tokens opt back in — they're the parts that have a
   hover state and a click action. */
.var-token {
  @apply rounded-sm cursor-text;
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

/* Inverted plaque — --text on --bg rather than a literal #1d1d1f, so it keeps
   its "dark card with light text" look in the light theme and flips correctly
   in the dark one without introducing a new colour. */
.var-tip {
  @apply fixed z-2000 flex flex-col gap-[3px] rounded-md pointer-events-none;
  padding: 7px 10px;
  background: var(--text);
  color: var(--bg);
  box-shadow: 0 8px 22px rgba(0, 0, 0, 0.22);
}

.var-tip-value {
  @apply text-[11.5px] break-all;
  font-family: var(--mono);
}

.var-tip-meta {
  @apply text-[11px];
  color: color-mix(in srgb, var(--bg) 60%, transparent);
}
</style>
