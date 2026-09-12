import '@wailsio/runtime'

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'
import { useTheme } from './composables/useTheme'

useTheme()

createApp(App).use(createPinia()).mount('#app')
