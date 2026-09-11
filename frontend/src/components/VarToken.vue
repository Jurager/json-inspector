<script setup lang="ts">
import { computed, ref } from 'vue'
import { useEnvironmentsStore } from '../stores/environments'
import { isMac } from '../lib/platform'

const props = defineProps<{ name: string }>()

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

const modifier = computed(() => (isMac.value ? '⌥клик' : 'Alt+клик'))

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
</script>

<template>
  <span
    ref="root"
    class="var-token"
    :class="{ unknown: !known, secret: isSecret }"
    @mouseenter="showTip"
    @mouseleave="hideTip"
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

.var-token {
  @apply rounded-sm;
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
