import { onMounted, onBeforeUnmount, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { App as Backend } from '../../bindings/json-inspector'

// Where the update check can be. The design draws the first three; the rest are the same line
// carrying what it doesn't cover — an update to install, the install itself, and failures.
export type UpdatePhase = 'idle' | 'checking' | 'uptodate' | 'available' | 'installing' | 'error'

// The About window's own state: it is a separate document, so nothing here is shared with the
// main window's link — the two meet in the backend, not in module state.
const phase = ref<UpdatePhase>('idle')
const latest = ref('')
const checkedAt = ref(0)
const error = ref('')

let off: (() => void) | null = null

export function useUpdateCheck() {
  async function applyStatus() {
    try {
      const s = await Backend.UpdateStatus()
      if (!s) return
      // An update that the last check found is offered without asking again. Never over a
      // check that has already started or finished: this answer is the older of the two.
      if (s.available && phase.value === 'idle') {
        latest.value = s.latest
        phase.value = 'available'
      }
      if (phase.value === 'idle') checkedAt.value = s.checkedAt ?? 0
    } catch {
      // No backend answer (or a plain browser): the line stays on "no check yet".
    }
  }

  async function check() {
    if (phase.value === 'checking' || phase.value === 'installing') return
    phase.value = 'checking'
    error.value = ''
    try {
      const u = await Backend.CheckForUpdates()
      checkedAt.value = u?.checkedAt ?? Date.now()
      if (u?.available) {
        latest.value = u.latest
        phase.value = 'available'
      } else {
        phase.value = 'uptodate'
      }
    } catch {
      error.value = 'Не удалось проверить обновления'
      phase.value = 'error'
    }
  }

  async function install() {
    if (!latest.value || phase.value === 'installing') return
    phase.value = 'installing'
    error.value = ''
    try {
      // The app replaces its own binary and relaunches, so this call does not return.
      await Backend.UpdateNow(latest.value)
    } catch (e) {
      error.value = `Не удалось обновиться: ${e}`
      phase.value = 'error'
    }
  }

  // A check asked for from the main window or the menu. Both entry points take the parked
  // request: the method clears it, so exactly one of them acts on it.
  async function checkIfRequested() {
    try {
      if (await Backend.TakeUpdateCheckRequest()) check()
    } catch {
      // Nothing to do: the window is usable without a check.
    }
  }

  onMounted(async () => {
    if (!off) off = Events.On('update-check', checkIfRequested)
    await applyStatus()
    await checkIfRequested()
  })

  onBeforeUnmount(() => {
    off?.()
    off = null
  })

  return { phase, latest, checkedAt, error, check, install }
}
