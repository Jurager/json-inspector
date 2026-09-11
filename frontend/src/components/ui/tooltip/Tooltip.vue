<script setup lang="ts">
import {
  TooltipContent,
  TooltipPortal,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from 'reka-ui'

// Hover, the delay, the placement and the ARIA wiring are reka-ui's; the plaque
// is the app's `.tooltip` class in style.css — global, not scoped here, because
// the class lands on the element reka-ui builds inside its portal and never
// carries this component's scope attribute. Read-only hints only — anything
// that has to be clicked belongs in a Popover.
//
// The provider lives here rather than once at the app root: the About window is
// a separate Vue app, and a tooltip has to work in whichever window renders it.
// The only thing lost is the shared delay between neighbouring tooltips.
//
// Deliberately NOT reka's whole prop surface: passing `open`, `defaultOpen` and
// the rest through as `undefined` (which is what `v-bind="props"` and a bound
// `:open="props.open"` both do) leaves the root never opening on hover — checked
// with a real mouse over CDP, hovered tooltip by tooltip. These four, always
// defined, are what the app needs.
const props = withDefaults(
  defineProps<{
    side?: 'top' | 'right' | 'bottom' | 'left'
    delayDuration?: number
    disabled?: boolean
    /** How far from the trigger the plaque sits, in px. */
    sideOffset?: number
  }>(),
  { side: 'bottom', delayDuration: 150, disabled: false, sideOffset: 6 }
)

// The same reason as ui/dialog and ui/popover: the root is a provider, not an
// element, so a class the caller passes would land on nothing and be lost.
defineOptions({ inheritAttrs: false })
</script>

<template>
  <TooltipProvider :delay-duration="props.delayDuration">
    <TooltipRoot
      :delay-duration="props.delayDuration"
      :disabled="props.disabled"
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
