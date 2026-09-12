<script setup lang="ts">
import {
  TooltipContent,
  TooltipPortal,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from 'reka-ui'

// The plaque is `.tooltip` in style.css: it lands on an element reka builds in its portal,
// where no scope of ours reaches. Pass props explicitly — `v-bind="props"` never opens.
const props = withDefaults(
  defineProps<{
    side?: 'top' | 'right' | 'bottom' | 'left'
    delayDuration?: number
    disabled?: boolean
    sideOffset?: number
    /** Keyboard focus only: a panel opening moves focus onto its own first button. */
    ignoreNonKeyboardFocus?: boolean
  }>(),
  {
    side: 'bottom',
    delayDuration: 150,
    disabled: false,
    sideOffset: 6,
    ignoreNonKeyboardFocus: true,
  }
)

// The root is a provider, not an element: a class from the caller would be lost.
defineOptions({ inheritAttrs: false })
</script>

<template>
  <TooltipProvider :delay-duration="props.delayDuration">
    <TooltipRoot
      :delay-duration="props.delayDuration"
      :disabled="props.disabled"
      :ignore-non-keyboard-focus="props.ignoreNonKeyboardFocus"
    >
      <TooltipTrigger as-child>
        <slot name="trigger" />
      </TooltipTrigger>
      <TooltipPortal>
        <TooltipContent
          v-bind="$attrs"
          class="tooltip"
          :side="props.side"
          :side-offset="props.sideOffset"
        >
          <slot />
        </TooltipContent>
      </TooltipPortal>
    </TooltipRoot>
  </TooltipProvider>
</template>
