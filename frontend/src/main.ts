import '@wailsio/runtime'

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'
import { i18n } from './i18n'
import { useTheme } from './composables/useTheme'
import { useLocale } from './composables/useLocale'

// Both are installed before the app is mounted: each resolves its setting against what the window was
// created with and against the system, and the first frame has to be right the first time.
useTheme()
useLocale()

createApp(App).use(i18n).use(createPinia()).mount('#app')
