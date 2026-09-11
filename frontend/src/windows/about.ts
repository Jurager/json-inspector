// Separate entry point for the About window: it is its own document, so it does
// not inherit App.vue's mount-time work (history hydration, keychain secrets,
// the six event subscriptions, the global key/click handlers). A route in the
// main bundle would have meant gating every one of those on "am I the About
// window"; a second input costs three lines in vite.config.ts instead.
import '@wailsio/runtime'

import { createApp } from 'vue'
import About from './About.vue'
import '../style.css'

createApp(About).mount('#app')
