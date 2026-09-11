// The app's text field — see Input.vue for the metrics. The handoff's own
// field is `field`; use `bare` only for an input that sits inside a container
// drawing the frame (a filter pill, the command line's URL box).
//
//   <Input v-model="query" placeholder="Поиск…" />
//   <Input v-model="filter" variant="bare" size="sm" class="flex-1 min-w-0" />
export { default as Input } from './Input.vue'
