<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import RequestChipPopover from './RequestChipPopover.vue'
import { Popover, PopoverAnchor } from '../ui/popover'
import { useSlidingPill } from '../../composables/useSlidingPill'
import { AuthType } from '../../../bindings/json-inspector/internal/domain'
import type { ChipName, RequestSource } from '../../lib/requestSource'
import { useMessages } from '../../i18n'

// What the request is made of, as a quiet row of underlined tabs under the address: the same grammar
// the answer's own tabs are read by, in the ink of a label rather than the accent of a choice, so the
// two rows are never mistaken for each other. The dot says a section holds something; the rule under
// the open one is the mark that travels to it.
const props = defineProps<{ source: RequestSource }>()

const { t } = useMessages()

// In the order the bar draws them. One list, so the row and the popover it opens can never disagree
// about which sections a request has.
const CHIPS: ChipName[] = ['params', 'headers', 'auth', 'body', 'scripts']

const openChip = computed(() => props.source.openChip)

function toggle(chip: ChipName) {
  props.source.setOpenChip(openChip.value === chip ? null : chip)
}

// The popover keeps drawing the section it was showing while it closes: an empty panel for the length
// of the animation would read as the section having gone.
const displayed = ref<ChipName | null>(null)
watch(
  openChip,
  (chip) => {
    if (chip) displayed.value = chip
  },
  { immediate: true }
)

// Whether a section holds anything, which is what tells a filled one from an empty. "None" and
// "inherit" are answers about who authorizes the request rather than credentials, so they are the two
// the authorization section is empty in.
function filled(chip: ChipName): boolean {
  const source = props.source
  switch (chip) {
    case 'params':
      return source.enabledParamsCount > 0
    case 'headers':
      return source.enabledHeadersCount > 0
    case 'auth':
      return source.auth.type !== AuthType.AuthNone && source.auth.type !== AuthType.AuthInherit
    case 'body':
      return source.hasBody
    case 'scripts':
      return Boolean(source.scripts?.pre?.trim() || source.scripts?.post?.trim())
  }
}

// The rule travels to the section that was opened rather than appearing under it. Nothing is chosen
// while no panel is open, and the mark then fades where it stands — see useSlidingPill.
const tabsEl = ref<HTMLElement | null>(null)
const { style: markStyle, ready: markReady } = useSlidingPill(tabsEl, '.bar-tab.active', () => openChip.value)
</script>

<template>
  <Popover :open="openChip !== null" @update:open="(v) => !v && props.source.setOpenChip(null)">
    <PopoverAnchor ref="tabsEl" class="bar-tabs">
      <span class="slide-mark" :class="{ ready: markReady }" :style="markStyle"></span>
      <button
        v-for="chip in CHIPS"
        :key="chip"
        type="button"
        class="bar-tab"
        :class="{ active: openChip === chip, filled: filled(chip) }"
        @click="toggle(chip)"
      >
        {{ t(`request.chips.${chip}`) }}
        <span v-if="filled(chip)" class="bar-tab-dot"></span>
      </button>
    </PopoverAnchor>
    <RequestChipPopover v-if="displayed" :chip="displayed" :source="props.source" />
  </Popover>
</template>

<style scoped>
@reference "../../style.css";

/* The row is taller than the field above it and stands 4px under it: the tabs are words with a rule
   beneath them, and the rule needs air of its own rather than the field's edge. The padding is inside
   the row so that a tab's offsets — which is how the mark is placed — are measured from its edge. */
.bar-tabs {
  @apply relative flex items-stretch gap-5 h-10;
  margin-top: 4px;
  padding: 0 20px;
}

/* The mark here is a rule at the bottom of the chosen tab, which the measurement gives the tab's own
   box: it is drawn by the background rather than by the mark's height, which is the tab's. The ink is
   a label's rather than the accent an answer's tab carries, because the two rows are two different
   readings and must not be taken for each other. */
.bar-tabs .slide-mark {
  background: linear-gradient(to bottom, transparent calc(100% - 1.5px), var(--text) 0);
}

/* No padding of its own: the tabs stand on the row's 20px gap, which is what the design measures
   between them — a tab that carried the space instead would put its own rule wider than its word. */
.bar-tab {
  @apply flex items-center gap-[7px] p-0 border-0 bg-transparent cursor-pointer;
  color: var(--text-tertiary);
  font-family: inherit;
  font-size: 13px;
  font-weight: 500;
  transition: color 0.15s ease;
  --wails-draggable: no-drag;
}

.bar-tab:hover {
  color: var(--text);
}

.bar-tab.filled {
  color: var(--text-secondary);
}

.bar-tab.active {
  color: var(--text);
  font-weight: 600;
}

.bar-tab-dot {
  @apply flex-none w-[5px] h-[5px] rounded-full bg-text-tertiary;
}

.bar-tab.active .bar-tab-dot {
  @apply bg-text;
}
</style>
