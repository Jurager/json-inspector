<script setup lang="ts">
import { DropdownMenuItem } from 'reka-ui'
import Icon from '../ui/Icon.vue'
import { useEnvironmentsStore } from '../../stores/environments'
import { useMessages } from '../../i18n'
import { usePlatform } from '../../composables/usePlatform'
import { DropdownMenuContent } from '../ui/dropdown-menu'

const store = useEnvironmentsStore()

const { t } = useMessages()
const { shortcut } = usePlatform()

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
    <div class="env-menu-head">{{ t('environments.menuTitle') }}</div>

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
      <span v-if="env.readonly" class="env-badge">{{ t('common.readOnly') }}</span>
      <span v-else class="env-count">
        {{ env.vars.length }}<template v-if="env.id === store.activeId"> {{ t('environments.variablesShort') }}</template>
      </span>
    </DropdownMenuItem>

    <DropdownMenuItem class="env-row" :class="{ active: store.activeId === null }" @select="choose(null)">
      <span class="env-row-mark">
        <Icon v-if="store.activeId === null" name="check" :size="12" />
      </span>
      <span class="env-row-name none">{{ t('titlebar.noEnvironment') }}</span>
    </DropdownMenuItem>

    <div class="env-divider"></div>

    <DropdownMenuItem class="env-row" @select="edit">
      <span class="env-row-mark"><Icon name="list" :size="13" /></span>
      <span class="env-row-name">{{ t('environments.editVariables') }}</span>
      <span class="env-hint mono">{{ editHint }}</span>
    </DropdownMenuItem>
  </DropdownMenuContent>
</template>

<style scoped>
@reference "../../style.css";

.env-menu-head {
  @apply px-2.5 pt-1.5 pb-2 text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.env-row {
  @apply flex items-center gap-2.5 w-full h-[34px] px-2.5 border-none rounded-[8px] bg-transparent text-text text-left cursor-pointer;
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
  @apply flex-none w-4 inline-flex items-center justify-center text-accent;
}

.env-row-name {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[13px];
}

.env-row-name.none {
  @apply text-text-secondary;
}

.env-count {
  @apply flex-none text-[12px] text-text-tertiary;
  font-family: var(--mono);
}

.env-badge {
  @apply flex-none text-red bg-red-soft text-[11px] font-semibold py-[3px] px-[7px] rounded-md;
}

.env-divider {
  @apply h-px mx-2 my-1.5 bg-border;
}

.env-hint {
  @apply flex-none text-[12px] text-text-tertiary;
}
</style>
