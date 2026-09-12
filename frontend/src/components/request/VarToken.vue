<script setup lang="ts">
import { computed, ref } from 'vue'
import { useEnvironmentsStore } from '../../stores/environments'
import { usePlatform } from '../../composables/usePlatform'
import { useHoverArrival } from '../../composables/useHoverArrival'
import { Tooltip } from '../ui/tooltip'

const props = defineProps<{ name: string; offset?: number }>()

const store = useEnvironmentsStore()
const { isMac } = usePlatform()

const resolution = computed(() => store.resolve(props.name))
const known = computed(() => resolution.value !== null)
const isSecret = computed(() => resolution.value?.kind === 'secret')

const label = computed(() => `{{${props.name}}}`)

const scopeLabel = computed(() => {
  const r = resolution.value
  if (!r) return ''
  return r.source === 'env' ? (store.active?.name ?? 'Окружение') : 'Глобальные'
})

const modifier = computed(() => (isMac.value ? '⌥клик' : 'Alt+клик'))

const root = ref<HTMLElement | null>(null)

const hintArmed = useHoverArrival(root)

function onClick(e: MouseEvent) {
  if (e.altKey) {
    e.preventDefault()
    e.stopPropagation()
    const r = resolution.value
    store.openSheet({
      envId: r?.source === 'env' ? store.activeId : null,
      varName: props.name,
    })
    return
  }

  if (window.getSelection()?.toString()) return

  const input = siblingInput(root.value?.parentElement ?? null)
  if (!input) return

  e.preventDefault()
  input.focus()
  const at = props.offset ?? input.value.length
  input.setSelectionRange(at, at)
}

function siblingInput(from: HTMLElement | null): HTMLInputElement | null {
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

.var-token {
  @apply rounded-sm cursor-text select-text;
  pointer-events: auto;
  padding: 1px 4px;
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
  @apply text-[11.5px] break-all;
  font-family: var(--mono);
}

.var-tip-meta {
  @apply text-[11px];
  color: color-mix(in srgb, var(--tip-text) 60%, transparent);
}
</style>
