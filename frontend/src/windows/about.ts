import '@wailsio/runtime'

import { createApp } from 'vue'
import About from './About.vue'
import '../style.css'
import { useTheme } from '../composables/useTheme'

useTheme()

createApp(About).mount('#app')
