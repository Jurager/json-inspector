<script setup lang="ts">
import { computed } from 'vue'
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuItem, DropdownMenuContent } from '../ui/dropdown-menu'
import { useAuthSchemes } from '../../composables/useAuthSchemes'
import { useMessages } from '../../i18n'
import type { AuthType, Scheme } from '../../../bindings/json-inspector/internal/domain'

// Which scheme a request authorizes itself with, offered the way the design offers it: the schemes a
// request usually needs sit in the control, the rest are behind «Ещё», and a scheme that needs a
// level above it is not offered where there is none.
//
// The list comes from Go, so this component has no idea what a Bearer is — it draws whatever it was
// handed, and a scheme added on that side appears here without a line changing.
const props = defineProps<{
  canInherit: boolean
  value: AuthType
  scale?: 'compact' | 'roomy'
}>()

const emit = defineEmits<{ select: [type: AuthType] }>()

const { t } = useMessages()
const { offered } = useAuthSchemes()

const schemes = computed(() => offered(props.canInherit))
const inControl = computed(() => schemes.value.filter((s) => s.primary))
const inMenu = computed(() => schemes.value.filter((s) => !s.primary))
// The menu is one segment of the control, and while it is the one in use it says what it holds: a
// control reading «Ещё» beside a request authorized by OAuth 2.0 would be hiding the answer.
const active = computed(() => schemes.value.find((s) => s.type === props.value) ?? null)
const menuInUse = computed(() => (active.value ? !active.value.primary : false))

// The menu spells a scheme out and the control shortens it: «AWS Signature» is what the list says,
// and «AWS» is what fits in a segment a sixth of the control wide. Two names, because they are two
// places — and the short one is the scheme's own label, which is the name it goes by everywhere else.
function menuNameOf(scheme: Scheme): string {
  return t(scheme.menu || scheme.label)
}
</script>

<template>
  <div class="switch" :class="scale ?? 'compact'">
    <button
      v-for="scheme in inControl"
      :key="scheme.type"
      type="button"
      class="seg"
      :class="{ active: scheme.type === value }"
      @click="emit('select', scheme.type)"
    >
      {{ t(scheme.label) }}
    </button>

    <DropdownMenu v-if="inMenu.length">
      <DropdownMenuTrigger as-child>
        <button type="button" class="seg more" :class="{ active: menuInUse }">
          <span>{{ menuInUse && active ? t(active.label) : t('request.auth.more') }}</span>
          <svg
            width="10"
            height="10"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.6"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="m6 9 6 6 6-6" />
          </svg>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent class="auth-menu" align="end" :side-offset="6">
        <DropdownMenuItem
          v-for="scheme in inMenu"
          :key="scheme.type"
          class="menu-item"
          @select="emit('select', scheme.type)"
        >
          {{ menuNameOf(scheme) }}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.switch {
  @apply flex gap-0.5 p-0.5 bg-bg-inset;
}

.switch.compact {
  border-radius: 8px;
}

/* The sheet's scale: the control is as wide as the sheet's column, because a scheme is chosen once
   for a level and the words in it are worth the room. */
.switch.roomy {
  @apply flex-1 w-full;
  border-radius: 8px;
}

.seg {
  @apply relative flex-1 min-w-0 text-center border-none bg-transparent cursor-pointer text-text-secondary;
  font: inherit;
  font-weight: 500;
  /* A segment is one line by construction, and the control is a fixed width whatever is in it: a
     name that wrapped would make the row two rows tall and shove the fields under it out of place. */
  white-space: nowrap;
  transition: background 0.15s ease, color 0.15s ease;
}

.switch.compact .seg {
  font-size: 13px;
  height: 30px;
  padding: 0;
  border-radius: 7px;
}

/* The sheet's segments are the height of the code sheet's switch: the two controls are one shape in
   the drawing, whichever sheet they are in. */
.switch.roomy .seg {
  font-size: 13px;
  height: 28px;
  padding: 0;
  border-radius: 6px;
}

/* The active segment is the panel's own colour lifted off the inset it sits on, which is what the
   design draws: the control reads as a groove and the choice as the thing that filled it. */
.seg.active {
  @apply bg-bg-panel text-text;
  font-weight: 600;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.seg.more {
  @apply flex items-center justify-center;
  gap: 5px;
}

.switch.roomy .seg.more {
  gap: 3px;
}

.seg.more svg {
  @apply flex-none;
}

/* The name the trigger carries is the only one that can be long, and a name too long for the segment
   is cut rather than pushing the chevron out of it. */
.seg.more span {
  @apply min-w-0 overflow-hidden text-ellipsis;
}
</style>
