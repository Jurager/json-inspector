// Side-effect import, and it must come first: the runtime's index pulls in the
// contextmenu, drag and appregion modules. Without it the --wails-draggable
// regions silently stop working — the window just won't drag.
import '@wailsio/runtime'

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'

createApp(App).use(createPinia()).mount('#app')
