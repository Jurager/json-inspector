import '@wailsio/runtime'

import { createApp } from 'vue'
import Settings from './Settings.vue'
import '../style.css'
import { i18n } from '../i18n'
import { useTheme } from '../composables/useTheme'
import { useLocale } from '../composables/useLocale'

// A window of its own, so it resolves both settings itself — and the same way: from what Go put on
// its URL, and from the system.
useTheme()
useLocale()

createApp(Settings).use(i18n).mount('#app')
