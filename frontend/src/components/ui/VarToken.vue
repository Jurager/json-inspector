<script setup lang="ts">
import { computed, ref } from 'vue'
import { useEnvironmentsStore } from '../../stores/environments'
import { usePlatform } from '../../composables/usePlatform'
import { useHoverArrival } from '../../composables/useHoverArrival'
import { Tooltip } from './tooltip'
import { useMessages } from '../../i18n'

const props = defineProps<{ name: string; offset?: number }>()

const store = useEnvironmentsStore()
const { isMac } = usePlatform()

const { t } = useMessages()

const resolvedVar = computed(() => store.resolveVariable(props.name))
const isKnown = computed(() => resolvedVar.value !== null)
const isSecret = computed(() => resolvedVar.value?.kind === 'secret')

const label = computed(() => `{{${props.name}}}`)

const scopeLabel = computed(() => {
  const r = resolvedVar.value
  if (!r) return ''
  return r.source === 'env' ? (store.activeEnvironment?.name ?? t('varToken.environment')) : t('varToken.globals')
})

const modifier = computed(() => t('varToken.clickModifier', { modifier: isMac.value ? '⌥' : 'Alt' }))

const root = ref<HTMLElement | null>(null)

const hintArmed = useHoverArrival(root)

function onClick(e: MouseEvent) {
  if (e.altKey) {
    e.preventDefault()
    e.stopPropagation()
    const r = resolvedVar.value
    store.openSheet({
      envId: r?.source === 'env' ? store.activeId : null,
      varName: props.name,
    })
    return
  }

  if (window.getSelection()?.toString()) return

  const input = nearestInput(root.value?.parentElement ?? null)
  if (!input) return

  e.preventDefault()
  input.focus()
  const at = props.offset ?? input.value.length
  input.setSelectionRange(at, at)
}

// Walks up: the field is an ancestor's descendant, not a sibling of the token.
function nearestInput(from: HTMLElement | null): HTMLInputElement | null {
  let node: HTMLElement | null = from
  while (node) {
    const found = node.querySelector('input')
    if (found) return found
    node = node.parentElement
  }
  return null
}

</script>

<template>
  <Tooltip class="var-tip" :disabled="!hintArmed">
    <template #trigger>
      <span
        ref="root"
        class="var-token"
        :class="{ unknown: !isKnown, secret: isSecret }"
        @click="onClick"
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
