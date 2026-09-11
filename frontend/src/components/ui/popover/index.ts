// Styled wrappers over reka-ui's Popover: behaviour is theirs, appearance is the
// app's own classes. Use it for a panel that hangs off a control; for a list of
// commands use ui/dropdown-menu instead.
//
//   <Popover>
//     <PopoverTrigger as-child><Button size="sm">…</Button></PopoverTrigger>
//     <PopoverContent align="end" :side-offset="4" class="params-panel">…</PopoverContent>
//   </Popover>
export { PopoverRoot as Popover, PopoverTrigger, PopoverAnchor } from 'reka-ui'
export { default as PopoverContent } from './PopoverContent.vue'
