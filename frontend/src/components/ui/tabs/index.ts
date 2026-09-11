// Styled wrappers over reka-ui's Tabs: roving tabindex, arrow-key navigation
// and the ARIA wiring come from there, the look from the app's .tabs/.tab
// classes. Use it for any strip of views.
//
//   <Tabs v-model="active">
//     <TabsList>
//       <TabsTrigger value="body">Тело</TabsTrigger>
//     </TabsList>
//     <TabsContent value="body" class="col">…</TabsContent>
//   </Tabs>
export { TabsRoot as Tabs, TabsContent } from 'reka-ui'
export { default as TabsList } from './TabsList.vue'
export { default as TabsTrigger } from './TabsTrigger.vue'
