<script setup lang="ts">
import { ref, watch } from 'vue'
import Dialog from '../ui/dialog/Dialog.vue'
import { Button } from '../ui/button'
import Input from '../ui/input/Input.vue'
import Tooltip from '../ui/tooltip/Tooltip.vue'
import { describeFailure, useMessages } from '../../i18n'
import { useWorkspacesStore } from '../../stores/workspaces'
import { WORKSPACE_COLORS, tintOf } from './palette'

const props = defineProps<{ open: boolean }>()

const emit = defineEmits<{ (e: 'close'): void }>()

const store = useWorkspacesStore()

const { t } = useMessages()

const name = ref('')
const color = ref<string>('')
const failure = ref('')

// A card that opens is a card that starts over: the name of the workspace that was just made is not
// the name of the next one, and a refusal from the last attempt is not about this one.
watch(
  () => props.open,
  (open) => {
    if (!open) return
    name.value = ''
    color.value = ''
    failure.value = ''
  }
)

async function submit() {
  try {
    await store.create({ name: name.value, color: color.value })
  } catch (err) {
    failure.value = describeFailure(err)
    return
  }
  emit('close')
}
</script>

<template>
  <Dialog :open="open" class="w-[380px] p-4.5" @escape-key-down.prevent @update:open="emit('close')">
    <div class="card-title">{{ t('workspaces.createTitle') }}</div>

    <label class="field">
      <span class="label">{{ t('workspaces.nameLabel') }}</span>
      <Input v-model="name" size="md" :placeholder="t('workspaces.namePlaceholder')" @keydown.enter="submit" />
    </label>

    <div class="field">
      <span class="label">{{ t('workspaces.kindLabel') }}</span>
      <div class="segment">
        <span class="segment-item active">{{ t('workspaces.kindPersonal') }}</span>
        <!-- Nothing makes a team yet, so the choice says so rather than producing a personal space
             under a team's name. The invitations field the mockup draws arrives with it. -->
        <Tooltip>
          <template #trigger>
            <span class="segment-item muted">{{ t('workspaces.kindTeam') }}</span>
          </template>
          {{ t('workspaces.kindTeamSoon') }}
        </Tooltip>
      </div>
    </div>

    <div class="field">
      <span class="label">{{ t('workspaces.colorLabel') }}</span>
      <div class="swatches">
        <button
          v-for="tint in WORKSPACE_COLORS"
          :key="tint"
          type="button"
          class="swatch"
          :class="{ chosen: color === tint }"
          :style="{ background: tintOf(tint), '--tint': tintOf(tint) }"
          @click="color = color === tint ? '' : tint"
        ></button>
      </div>
    </div>

    <p v-if="failure" class="failure">{{ failure }}</p>

    <div class="actions">
      <Button variant="quiet" @click="emit('close')">{{ t('common.cancel') }}</Button>
      <Button variant="primary" @click="submit">{{ t('workspaces.createAction') }}</Button>
    </div>
  </Dialog>
</template>

<style scoped>
@reference "../../style.css";

.card-title {
  @apply text-[14px] font-semibold mb-3.5;
}

/* The fields sit one under another with the mockup's own gap, and the whole card is a column. */
.field {
  @apply flex flex-col gap-[5px] mb-3.5;
}

.label {
  @apply text-[12px] font-medium;
}

.segment {
  @apply inline-flex gap-0.5 p-0.5 rounded-[7px] bg-bg-inset self-start;
}

.segment-item {
  @apply h-7 px-3 inline-flex items-center rounded-[5px] text-[12.5px] cursor-pointer select-none;
}

.segment-item.active {
  @apply bg-bg-panel text-text shadow-[var(--shadow-btn)];
}

.segment-item.muted {
  @apply text-text-tertiary cursor-default;
}

.swatches {
  @apply flex items-center gap-2;
}

.swatch {
  @apply w-[22px] h-[22px] rounded-full border-none cursor-pointer p-0;
}

/* The ring is the tint itself, drawn twice around the dot: the card's own ground, then the colour —
   which is how the mockup marks the chosen one. */
.swatch.chosen {
  box-shadow:
    0 0 0 2px var(--bg-panel),
    0 0 0 3.5px var(--tint);
}

.failure {
  @apply m-0 mb-3 text-[12px] text-red;
}

.actions {
  @apply flex items-center justify-end gap-2.5 mt-1;
}
</style>
