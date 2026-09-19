<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../ui/Icon.vue'
import { Popover, PopoverContent, PopoverTrigger } from '../ui/popover'
import { useEnvironmentsStore } from '../../stores/environments'
import { usePlatform } from '../../composables/usePlatform'
import { useRequestEnvironment } from '../../composables/useRequestEnvironment'
import type { RequestSource } from '../../lib/requestSource'
import { useMessages } from '../../i18n'

// The environment this one request goes out under, drawn beside the address it applies to: it is the
// last thing a person checks before pressing send.
//
// Following the window is an answer of its own rather than the absence of one, so the button says
// which of the two it is — a name and a filled dot for a request pinned to one environment, a name
// and a ring for a request that follows — before the popover is opened.
const props = defineProps<{ source: RequestSource }>()

const envStore = useEnvironmentsStore()
const { shortcut } = usePlatform()
const { t } = useMessages()

const { pinned, label, options, choose } = useRequestEnvironment(props.source)

const open = ref(false)

function pick(id: string) {
  open.value = false
  choose(id)
}

// Editing is not one of the choices — it is another window — so the popover closes behind it.
function editVariables() {
  open.value = false
  envStore.openSheet()
}

const editHint = computed(() => shortcut('E'))
</script>

<template>
  <Popover v-model:open="open">
    <!-- A trigger rather than an anchor: the button is what opens the panel, and reka exempts its own
         trigger from the outside-press that would otherwise close it under the finger. -->
    <PopoverTrigger as-child>
      <button
        type="button"
        class="bar-env"
        :class="{ open, pinned }"
        :title="t('request.envTitle')"
      >
        <span class="bar-env-dot"></span>
        <span class="bar-env-name">{{ label }}</span>
      </button>
    </PopoverTrigger>
    <PopoverContent class="bar-env-pop" align="end" :side-offset="6">
      <div class="bar-env-pop-head">{{ t('request.envFor') }}</div>
      <button
        v-for="option in options"
        :key="option.value"
        type="button"
        class="bar-env-row"
        :class="{ active: option.value === props.source.environmentId }"
        @click="pick(option.value)"
      >
        <span class="bar-env-row-mark">
          <Icon v-if="option.value === props.source.environmentId" name="check" :size="15" />
        </span>
        <span class="bar-env-row-name">{{ option.name }}</span>
        <span class="bar-env-row-vars mono">{{ option.vars }}</span>
      </button>
      <div class="bar-env-pop-divider"></div>
      <button type="button" class="bar-env-row" @click="editVariables">
        <span class="bar-env-row-name">{{ t('environments.editVariables') }}</span>
        <span class="bar-env-row-hint">{{ editHint }}</span>
      </button>
    </PopoverContent>
  </Popover>
</template>

<style scoped>
@reference "../../style.css";

/* The panel's own box lives in style.css with the other popovers': this component's scope reaches
   the head and the rows, which are its own children, but not the portalled root they sit in. */

.bar-env {
  @apply flex-none flex items-center gap-[7px] h-[34px] px-2.5 rounded-[7px] cursor-pointer;
  border: 1px solid var(--border-strong);
  background: var(--bg-inset);
  color: var(--text-secondary);
  font: inherit;
  font-size: 12px;
  font-weight: 500;
  max-width: 190px;
  --wails-draggable: no-drag;
}

.bar-env:hover,
.bar-env.open {
  background: var(--bg-hover);
  color: var(--text);
}

.bar-env-name {
  @apply truncate;
}

/* Pinned to one of its own is green; following the window is a ring, which is the shape this app
   uses for an answer that has not been given. */
.bar-env-dot {
  @apply flex-none w-[7px] h-[7px] rounded-full;
  box-shadow: inset 0 0 0 1.5px var(--text-tertiary);
}

.bar-env.pinned .bar-env-dot {
  background: var(--green);
  box-shadow: none;
}

.bar-env-pop-head {
  @apply px-2.5 pt-1.5 pb-2 text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.bar-env-row {
  @apply flex items-center gap-2.5 w-full h-[34px] px-2.5 border-none rounded-[9px] bg-transparent text-text text-left cursor-pointer;
  font: inherit;
  font-size: 13px;
  --wails-draggable: no-drag;
}

.bar-env-row:hover {
  background: var(--bg-hover);
}

.bar-env-row.active {
  background: var(--bg-hover);
  font-weight: 600;
}

.bar-env-row-mark {
  @apply flex-none w-4 inline-flex items-center justify-center text-accent;
}

.bar-env-row-name {
  @apply flex-1 min-w-0 truncate;
}

.bar-env-row-vars {
  @apply flex-none text-[11px] text-text-tertiary;
  font-family: var(--mono);
}

/* The shortcut is a word of the app rather than a value of the environment: it is set in the app's
   own face and a shade larger, so it is not read as a second count beside the first. */
.bar-env-row-hint {
  @apply flex-none text-[12px] text-text-tertiary;
}

.bar-env-pop-divider {
  @apply h-px mx-2 my-1.5 bg-border;
}
</style>
