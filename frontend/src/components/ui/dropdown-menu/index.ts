// Styled wrappers over reka-ui's DropdownMenu: behaviour (placement, focus,
// keyboard, click-outside, ARIA) is theirs, appearance is the app's own
// .menu / .menu-item classes. Use these rather than hand-rolling a menu —
// the ones written by hand each re-implemented the same three things and
// usually missed one of them.
//
//   <DropdownMenu>
//     <DropdownMenuTrigger as-child><Button size="sm">…</Button></DropdownMenuTrigger>
//     <DropdownMenuContent align="start" :side-offset="4">
//       <DropdownMenuItem @select="…">Пункт</DropdownMenuItem>
//       <DropdownMenuSeparator />
//     </DropdownMenuContent>
//   </DropdownMenu>
export { DropdownMenuRoot as DropdownMenu, DropdownMenuTrigger } from 'reka-ui'
export { default as DropdownMenuContent } from './DropdownMenuContent.vue'
export { default as DropdownMenuItem } from './DropdownMenuItem.vue'
export { default as DropdownMenuSeparator } from './DropdownMenuSeparator.vue'
export { default as DropdownMenuLabel } from './DropdownMenuLabel.vue'
