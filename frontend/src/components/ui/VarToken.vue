<script setup lang="ts">
import { computed, ref } from 'vue'
import { useEnvironmentsStore } from '../../stores/environments'
import { usePlatform } from '../../composables/usePlatform'
import { useHoverArrival } from '../../composables/useHoverArrival'
import { Tooltip } from './tooltip'
import { useMessages } from '../../i18n'

// envId is the environment the request this token sits in pinned itself to, when it has one: the
// pill over a `{{token}}` shows a value, and it has to be the value that would really be sent.
//
// `plain` is the address's own drawing of a token: the characters keep their places, so the pill's
// box is not there to push them along, and a dotted underline is what says the word is a name.
const props = defineProps<{
  name: string
  text?: string
  offset?: number
  envId?: string
  variant?: 'pill' | 'plain'
}>()

const store = useEnvironmentsStore()
const { isMac } = usePlatform()

const { t } = useMessages()

const resolvedVar = computed(() => store.resolveVariable(props.name, props.envId))
const isKnown = computed(() => resolvedVar.value !== null)
const isSecret = computed(() => resolvedVar.value?.kind === 'secret')

// The characters of the field, not a tidied spelling of the name: the pill is painted over the text
// it stands for, so `{{ var3 }}` has to be drawn with its spaces or everything after it slides.
const label = computed(() => props.text ?? `{{${props.name}}}`)

const scopeLabel = computed(() => {
  const r = resolvedVar.value
  if (!r) return ''
  if (r.source !== 'env') return t('varToken.globals')
  // A pinned request answers in its own environment, and the tooltip names that one — the window's
  // own name here would be a tooltip about a different environment than the value under it.
  const env = props.envId
    ? store.environments.find((e) => e.id === props.envId)
    : store.activeEnvironment
  return env?.name ?? t('varToken.environment')
})

const modifier = computed(() => t('varToken.clickModifier', { modifier: isMac.value ? '⌥' : 'Alt' }))

const root = ref<HTMLElement | null>(null)

const hintArmed = useHoverArrival(root)

// The pill takes the press because it has to — a hover over it is what shows the value — so it is
// also the one that hands the press back: the field underneath gets the caret where it was aimed,
// exactly as it would have without the layer. On mousedown rather than click, and with the default
// suppressed: the browser would otherwise blur the field first and start a selection in a layer
// that is not editable, which is a highlight with nothing behind it.
function onMouseDown(e: MouseEvent) {
  if (e.altKey) {
    e.preventDefault()
    e.stopPropagation()
    // The scope the name would be opened in: the one it answers in, and — for a name nothing answers,
    // or a window with no environment chosen — the environment being worked in, which is where the
    // bar's own «Создать переменную» would put it. The globals are the target only for a name that
    // really resolves there, and for a window that has nowhere else to put one.
    // The level a new variable goes into is the one this token answers in, which for a pinned
    // request is its own environment rather than the window's.
    const r = resolvedVar.value
    const scope = props.envId || store.activeId
    const envId = r?.source === 'global' || scope === null ? null : scope
    store.openSheet({ envId, varName: props.name })
    return
  }

  const layer = root.value?.parentElement ?? null
  const field = nearestField(layer)
  if (!field) return

  e.preventDefault()
  field.focus()
  const at = offsetUnder(layer, e) ?? props.offset ?? field.value.length
  field.setSelectionRange(at, at)
}

// The layer covers a field of its own cell — a table cell, a covered box of a form — so the field
// it belongs to is the one beside it, not whatever the next ancestor on the way up happens to hold.
function nearestField(layer: HTMLElement | null): HTMLInputElement | HTMLTextAreaElement | null {
  return layer?.parentElement?.querySelector('input, textarea') ?? null
}

// Which character of the layer was pressed. Nothing here measures a glyph: the layer carries the
// field's own characters (see tokenSegments), so a range from the layer's start to the pressed
// character counts out the index the field is waiting for.
function offsetUnder(layer: HTMLElement | null, e: MouseEvent): number | null {
  if (!layer) return null
  const point = caretPoint(e.clientX, e.clientY)
  if (!point || !layer.contains(point.node)) return null

  const upTo = document.createRange()
  upTo.setStart(layer, 0)
  upTo.setEnd(point.node, point.offset)
  return upTo.toString().length
}

// Chromium has the shorthand; it is the engine under the window, and the browser preview of the
// window is a browser. Firefox named the same question differently, and answers it just as well.
function caretPoint(x: number, y: number): { node: Node; offset: number } | null {
  const range = document.caretRangeFromPoint?.(x, y)
  if (range) return { node: range.startContainer, offset: range.startOffset }
  const pos = document.caretPositionFromPoint?.(x, y)
  return pos ? { node: pos.offsetNode, offset: pos.offset } : null
}

</script>

<template>
  <Tooltip class="var-tip" :disabled="!hintArmed">
    <template #trigger>
      <span
        ref="root"
        class="var-token"
        :class="{ unknown: !isKnown, secret: isSecret, plain: props.variant === 'plain' }"
        @mousedown="onMouseDown"
        @pointerleave="hintArmed = true"
        >{{ label }}</span
      >
    </template>

    <div class="var-tip-value">
      {{ isSecret ? t('varToken.secretHidden') : resolvedVar?.value || t('varToken.empty') }}
    </div>
    <div class="var-tip-meta">{{ t('varToken.hint', { scope: scopeLabel, name, modifier }) }}</div>
  </Tooltip>
</template>

<style scoped>
@reference "../../style.css";

/* The chip of the design, painted *over* the characters of a real field: the padding is what makes
   it a chip, and the negative margin is what keeps it standing on the token rather than beside it —
   without them the pill's own box pushes its text off the characters (the caret, which stays with
   the field, shows the difference at once) and everything after the token slides along with it.
   The face and the size are the field's, inherited. */
.var-token {
  @apply cursor-text select-text;
  pointer-events: auto;
  padding: 3px 4px;
  margin: 0 -4px;
  border-radius: 6px;
  background: var(--accent-soft);
  color: var(--accent);
}

.var-token.unknown {
  background: var(--red-soft);
  color: var(--red);
  text-decoration: underline wavy;
  text-underline-offset: 3px;
}

/* The address's own drawing: no box and no padding, because the characters here stand exactly where
   the input put them and a pill reaching outside its text would push the caret off its mark. */
.var-token.plain {
  padding: 0;
  margin: 0;
  border-radius: 0;
  background: transparent;
  color: var(--accent);
  text-decoration: underline dotted;
  text-decoration-color: var(--accent);
  text-underline-offset: 3px;
}

.var-token.plain.unknown {
  background: transparent;
  color: var(--red);
  text-decoration: underline wavy;
  text-decoration-color: var(--red);
}

.var-tip {
  @apply flex flex-col gap-[3px];
}

.var-tip-value {
  @apply text-xs break-all;
  font-family: var(--mono);
}

.var-tip-meta {
  @apply text-xs;
  color: color-mix(in srgb, var(--tip-text) 60%, transparent);
}
</style>
