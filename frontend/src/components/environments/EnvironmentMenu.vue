<script setup lang="ts">
import { DropdownMenuItem } from 'reka-ui'
import Icon from '../ui/Icon.vue'
import { useEnvironmentsStore } from '../../stores/environments'
import { shortcut } from '../../lib/platform'
import { DropdownMenuContent } from '../ui/dropdown-menu'

const store = useEnvironmentsStore()

const editHint = shortcut('E')

function choose(id: string | null) {
  store.setActive(id)
}

function edit() {
  store.openSheet()
}
</script>

<template>
  <DropdownMenuContent class="env-menu" align="end">
    <div class="env-menu-head">Окружение</div>

    <DropdownMenuItem
      v-for="env in store.environments"
      :key="env.id"
      class="env-row"
      :class="{ active: env.id === store.activeId }"
      @select="choose(env.id)"
    >
      <span class="env-row-mark">
        <Icon v-if="env.id === store.activeId" name="check" :size="12" />
      </span>
      <span class="env-row-name">{{ env.name }}</span>
      <span v-if="env.readonly" class="env-badge">только чтение</span>
      <span v-else class="env-count">
        {{ env.vars.length }}<template v-if="env.id === store.activeId"> перем.</template>
      </span>
    </DropdownMenuItem>

    <DropdownMenuItem class="env-row" :class="{ active: store.activeId === null }" @select="choose(null)">
      <span class="env-row-mark">
        <Icon v-if="store.activeId === null" name="check" :size="12" />
      </span>
      <span class="env-row-name none">Без окружения</span>
    </DropdownMenuItem>

    <div class="env-divider"></div>

    <DropdownMenuItem class="env-row" @select="edit">
      <span class="env-row-mark"><Icon name="list" :size="13" /></span>
      <span class="env-row-name">Редактировать переменные…</span>
      <span class="env-hint mono">{{ editHint }}</span>
    </DropdownMenuItem>
  </DropdownMenuContent>
</template>

<style scoped>
@reference "../../style.css";

.env-menu-head {
  @apply pt-1.5 px-2.5 pb-1 text-[10px] uppercase tracking-[0.08em] text-text-tertiary;
  font-family: var(--mono);
}

.env-row {
  @apply flex items-center gap-2 w-full py-[7px] px-2.5 border-none rounded-md bg-transparent text-text text-[13px] text-left cursor-pointer;
  font: inherit;
  --wails-draggable: no-drag;
}

.env-row:hover,
.env-row[data-highlighted] {
  @apply bg-bg-hover;
  outline: none;
}

.env-row.active {
  @apply bg-accent-soft font-semibold;
}

.env-row-mark {
  @apply flex-none w-3 inline-flex items-center justify-center text-accent;
}

.env-row-name {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap;
}

.env-row-name.none {
  @apply text-text-secondary;
}

.env-count {
  @apply flex-none text-[10.5px] text-text-tertiary;
  font-family: var(--mono);
}

.env-badge {
  @apply flex-none text-red bg-red-soft text-[10px] font-semibold py-px px-[5px] rounded-sm;
}

.env-divider {
  @apply h-px mx-2 my-[5px] bg-border;
}

.env-hint {
  @apply flex-none text-[10.5px] text-text-tertiary;
}
</style>
