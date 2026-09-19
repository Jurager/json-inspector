import { onBeforeUnmount, ref } from 'vue'
import { copyToClipboard } from '../lib/clipboard'

// The «Скопировано» tick a button shows: on for a moment, then off. It is the same gesture in four
// places — the response body, its headers, the raw view, a node's address — and each of them had
// written its own timer, none of which was dropped when the component went away.
//
// The flag says the text reached the clipboard and not that a button was pressed: a tick over a
// clipboard that never took the text is the one lie the app could tell about its own state, and the
// helper underneath answers whether it worked.
const TICK_MS = 1500

export function useCopyFeedback() {
  const copied = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null

  async function copy(text: string): Promise<boolean> {
    if (!(await copyToClipboard(text))) return false
    if (timer) clearTimeout(timer)
    copied.value = true
    timer = setTimeout(() => (copied.value = false), TICK_MS)
    return true
  }

  onBeforeUnmount(() => {
    if (timer) clearTimeout(timer)
  })

  return { copied, copy }
}
