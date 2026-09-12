// The About window is its own document and must not inherit App.vue's mount-time work:
// history hydration, keychain, event subscriptions, global handlers.
import '@wailsio/runtime'

import { createApp } from 'vue'
import About from './About.vue'
import '../style.css'

createApp(About).mount('#app')
