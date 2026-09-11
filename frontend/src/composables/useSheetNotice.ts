import { ref } from 'vue'

// The one red line at the bottom of the environments sheet. It is written from
// both halves — a rejected cell edit on the right, a rejected rename on the
// left — and shown under the table, so it is shared state rather than a prop
// threaded between siblings.
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
