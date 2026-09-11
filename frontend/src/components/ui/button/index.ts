// The app's buttons. Metrics from the handoff, nothing else — see Button.vue
// and IconButton.vue.
//
//   <Button @click="save">Сохранить</Button>
//   <Button variant="primary" :disabled="busy" @click="apply">Импортировать</Button>
//   <Button variant="ghost" @click="add">+ Параметр</Button>
//   <IconButton title="Удалить" variant="danger" size="sm" @click="remove">
//     <Icon name="xmark" :size="12" />
//   </IconButton>
export { default as Button } from './Button.vue'
export { default as IconButton } from './IconButton.vue'
