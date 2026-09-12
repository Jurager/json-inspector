// Must come first: the runtime's index pulls in the contextmenu, drag and appregion
// modules, without which the --wails-draggable regions silently stop working.
import '@wailsio/runtime'

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'

createApp(App).use(createPinia()).mount('#app')
