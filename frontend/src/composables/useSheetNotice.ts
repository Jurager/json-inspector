import { ref } from 'vue'

// Module-scoped because both halves of the environments sheet write it — a
// rejected cell edit and a rejected rename — while one place renders it.
const notice = ref('')

export function useSheetNotice() {
  function setNotice(message: string) {
    notice.value = message
  }

  function clearNotice() {
    notice.value = ''
  }

  return { notice, setNotice, clearNotice }
}
