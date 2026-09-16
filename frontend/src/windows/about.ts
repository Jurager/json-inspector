import '@wailsio/runtime'

import { createApp } from 'vue'
import About from './About.vue'
import '../style.css'
import { i18n } from '../i18n'
import { useTheme } from '../composables/useTheme'
import { useLocale } from '../composables/useLocale'

// The About window is a window of its own, so it needs its own copy of both settings resolved — and
// the same way, from what Go put on its URL and from the system.
useTheme()
useLocale()

createApp(About).use(i18n).mount('#app')
