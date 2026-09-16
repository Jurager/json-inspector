import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import wails from '@wailsio/runtime/plugins/vite'

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true
  },
  plugins: [vue(), tailwindcss(), wails('./bindings')],
  // The bundler build of vue-i18n asks its host for these flags and warns in the console about each
  // one it is not given. The app uses the composition API only (never the `$t`-on-`this` one), and
  // devtools support is for a browser extension a desktop window has no use for.
  define: {
    __VUE_I18N_FULL_INSTALL__: true,
    __VUE_I18N_LEGACY_API__: false,
    __INTLIFY_PROD_DEVTOOLS__: false
  },
  // Three entry points: the main window, the About window and Settings. They are
  // separate documents rather than routes — see src/windows/about.ts for why.
  build: {
    rollupOptions: {
      input: {
        main: 'index.html',
        about: 'about.html',
        settings: 'settings.html',
        update: 'update.html'
      }
    }
  }
})
