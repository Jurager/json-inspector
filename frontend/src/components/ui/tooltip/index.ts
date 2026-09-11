// A styled hint. Its trigger is whatever the caller puts in the slot; the
// plaque comes from Tooltip.vue.
//
//   <Tooltip>
//     <template #trigger><IconButton hint="Удалить" …/></template>
//     Удалить
//   </Tooltip>
//
// IconButton takes a `hint` prop that does this for you — reach for the raw
// Tooltip only when the content is more than a line of text.
export { default as Tooltip } from './Tooltip.vue'
