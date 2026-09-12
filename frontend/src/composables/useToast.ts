import { ref } from 'vue'

// Module-scoped so any component can raise a toast without threading it up to
// the shell, which only renders it (see ui/Toast.vue).
const message = ref('')
const kind = ref<'info' | 'error'>('info')
let timer: ReturnType<typeof setTimeout> | null = null

const VISIBLE_MS = 5000

export function useToast() {
  function show(text: string, severity: 'info' | 'error' = 'info') {
    message.value = text
    kind.value = severity
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => (message.value = ''), VISIBLE_MS)
  }

  function clear() {
    if (timer) clearTimeout(timer)
    timer = null
    message.value = ''
  }

  return { message, kind, show, clear }
}
