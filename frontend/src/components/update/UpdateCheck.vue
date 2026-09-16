<script setup lang="ts">
import { computed } from 'vue'
import { useUpdateCheck } from '../../composables/useUpdateCheck'
import { Button } from '../ui/button'
import Icon from '../ui/Icon.vue'
import { formatCheckedAt, useMessages } from '../../i18n'
import { formatVersion } from '../../lib/format'

// The check button and the line under it. Two windows draw them — the About window and the settings
// window — and what changes between them is the arrangement and one word, not the states: the same
// five, in the same order, off the same composable, so neither can drift from the other.
//
// `inline` is the settings window's: the button and its answer share a row, and the answer has no
// idle case there, because the row above already carries the date of the last check.
const props = defineProps<{ inline?: boolean }>()

const { t } = useMessages()
const { phase, latest, checkedAt, error, check, openWindow } = useUpdateCheck()

const label = computed(() => {
  switch (phase.value) {
    case 'checking':
      return t('update.checking')
    case 'installing':
      return t('update.installing')
    case 'available':
      return t('update.showUpdate')
    case 'uptodate':
    case 'error':
      return t('update.checkAgain')
    default:
      return t('update.check')
  }
})

const busy = computed(() => phase.value === 'checking' || phase.value === 'installing')

// One button, and what it does follows the state: with a release to offer it opens the window that
// describes it, and otherwise it runs a check. Installing is that window's business — it is the one
// that can show what is about to be installed.
const act = computed(() => (phase.value === 'available' ? openWindow : check))

const availableLabel = computed(() =>
  t(props.inline ? 'update.availableLinkShort' : 'update.availableLink', {
    version: formatVersion(latest.value),
  })
)

const idleLabel = computed(() => t('update.lastChecked', { at: formatCheckedAt(checkedAt.value) }))
</script>

<template>
  <div class="update-check" :class="{ 'update-check-inline': props.inline }">
    <Button class="update-button" variant="primary" size="lg" :disabled="busy" @click="act">
      <span v-if="busy" class="update-spinner"></span>
      {{ label }}
    </Button>

    <div class="update-status">
      <template v-if="phase === 'uptodate'">
        <span class="update-ok">
          <Icon name="check" :size="13" :stroke-width="2.4" />
          <span>{{ t('update.upToDate') }}</span>
        </span>
      </template>
      <template v-else-if="phase === 'checking'">{{ t('update.checkingLong') }}</template>
      <template v-else-if="phase === 'installing'">{{ t('update.installing') }}</template>
      <template v-else-if="phase === 'available'">
        <button class="update-link" @click="openWindow">
          <span class="update-dot"></span>
          <span>{{ availableLabel }}</span>
        </button>
      </template>
      <template v-else-if="phase === 'error'">
        <span class="update-error">{{ error }}</span>
      </template>
      <!-- No check has ever run: there is no date to give, and the box keeps its height either way
           so the button does not move when the first answer lands. -->
      <template v-else-if="!props.inline && checkedAt">{{ idleLabel }}</template>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.update-check {
  @apply flex flex-col items-center;
}

.update-check-inline {
  @apply flex-row items-center gap-3;
}

.update-button {
  @apply min-w-[186px];
}

/* White on the accent button, where the app's own `.spinner` (accent ring) would vanish. */
.update-spinner {
  @apply w-[13px] h-[13px] rounded-full inline-block mr-1.5;
  border: 2px solid rgba(255, 255, 255, 0.45);
  border-top-color: #fff;
  animation: spin 0.7s linear infinite;
}

.update-status {
  @apply h-7 mt-3 flex items-center justify-center text-xs text-text-tertiary;
}

.update-check-inline .update-status {
  @apply mt-0;
}

.update-ok {
  @apply inline-flex items-center gap-[5px] text-green font-medium;
}

/* The release is a link rather than a third label, because there is something to open behind it.
   It takes the accent and the weight the design gives it, over the line's own quiet grey. */
.update-link {
  @apply inline-flex items-center gap-[5px] text-accent font-semibold bg-transparent border-none p-0 cursor-pointer;
  font: inherit;
  font-size: 12px;
}

.update-link:hover {
  text-decoration: underline;
}

.update-link:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
  border-radius: 3px;
}

.update-dot {
  @apply w-1.5 h-1.5 rounded-full flex-none;
  background: var(--accent);
}

.update-error {
  @apply text-red;
}
</style>
