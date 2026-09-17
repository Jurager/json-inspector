<script setup lang="ts">
import Icon from '../Icon.vue'
import { useMessages } from '../../../i18n'

// The list sheet: one 440px card over a dimmed page, holding a title, a sentence, a body that
// scrolls and a pair of buttons. The collections page, the browser page and the environments window
// all raise a leaf in this shape, and it used to be a copy of the same forty lines in each of them.
//
// Not the environments window itself, which is a full-size leaf with a rail of its own: that one
// borrows this shape only for the confirmations it puts over itself.
const props = withDefaults(
  defineProps<{
    open?: boolean
    title: string
    sub?: string
    // A missing label is a missing button: a sheet that only reports something has one, and a sheet
    // that asks a question has two.
    cancel?: string
    action?: string
    actionDisabled?: boolean
  }>(),
  { open: true, sub: '', cancel: '', action: '', actionDisabled: false }
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'cancel'): void
  (e: 'action'): void
}>()

const { t } = useMessages()
</script>

<template>
  <div v-if="props.open" class="sheet-overlay" @click.self="emit('close')">
    <div class="sheet">
      <div class="sheet-head">
        <div class="sheet-titles">
          <span class="sheet-title">{{ props.title }}</span>
          <span v-if="props.sub" class="sheet-sub">{{ props.sub }}</span>
        </div>
        <button class="sheet-close" :title="t('common.close')" @click="emit('close')">
          <Icon name="xmark" :size="15" :stroke-width="2.2" />
        </button>
      </div>

      <div class="sheet-body">
        <slot />
      </div>

      <div v-if="props.cancel || props.action" class="sheet-foot">
        <button v-if="props.cancel" class="sheet-cancel" @click="emit('cancel')">
          {{ props.cancel }}
        </button>
        <button
          v-if="props.action"
          class="sheet-action"
          :disabled="props.actionDisabled"
          @click="emit('action')"
        >
          {{ props.action }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../style.css";

.sheet-overlay {
  @apply fixed inset-0 z-1500 flex items-center justify-center;
  background: rgba(0, 0, 0, 0.22);
}

/* The drawing's sheet: one card rather than a panel with a bar, sized to what it holds and padded
   once — what stands inside brings no padding of its own, and the body is the only thing that
   scrolls. */
.sheet {
  @apply flex flex-col w-[440px] max-w-[92vw] rounded-[14px];
  max-height: calc(100vh - 96px);
  padding: 18px;
  gap: 16px;
  background: var(--glass-sheet);
  backdrop-filter: var(--blur-sheet);
  box-shadow: var(--glass-sheet-shadow), 0 0 0 1px var(--glass-overlay-border);
}

.sheet-head {
  @apply flex-none flex items-start gap-3;
}

/* The head is a name and a sentence: what this is, and what it is about. */
.sheet-titles {
  @apply flex flex-col flex-1 gap-[5px] min-w-0;
}

.sheet-title {
  @apply text-[16px] font-semibold;
}

.sheet-sub {
  @apply text-[13px] text-text-secondary;
  line-height: 1.5;
}

.sheet-close {
  @apply flex-none inline-flex items-center justify-center w-7 h-7 rounded-[7px] border-0
         bg-transparent text-text-tertiary cursor-pointer;
  --wails-draggable: no-drag;
}

.sheet-close:hover {
  @apply bg-bg-active text-text;
}

.sheet-body {
  @apply flex-1 min-h-0 overflow-y-auto;
}

.sheet-foot {
  @apply flex-none flex justify-end gap-2.5 pt-1 border-t border-border;
}

.sheet-cancel {
  @apply h-[34px] px-3.5 rounded-lg border border-border-strong bg-bg-inset text-text
         text-[13.5px] font-medium cursor-pointer;
  font-family: inherit;
}

.sheet-cancel:hover {
  @apply bg-bg-hover;
}

.sheet-action {
  @apply h-[34px] px-4 rounded-lg border-0 bg-accent text-accent-text text-[13.5px] font-semibold
         cursor-pointer;
  font-family: inherit;
}

.sheet-action:hover:not(:disabled) {
  @apply brightness-110;
}

.sheet-action:disabled {
  @apply opacity-50 cursor-default;
}
</style>
