import { onMounted, onBeforeUnmount, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { UpdateService } from '../../bindings/json-inspector/internal/transport/wails'
import type { Info as UpdateInfo } from '../../bindings/json-inspector/internal/usecase/update'
import { describeFailure, t as tr } from '../i18n'

// Where the update check can be. The design draws the first three; the rest are the same line
// carrying what it doesn't cover — an update to install, the install itself, and failures.
export type UpdatePhase = 'idle' | 'checking' | 'uptodate' | 'available' | 'installing' | 'error'

// Three windows show this and each is a document of its own, so nothing here is shared between them:
// the About window, the settings window and the update window meet in the backend and in the event
// below, not in module state.
const phase = ref<UpdatePhase>('idle')
const latest = ref('')
const notes = ref<string[]>([])
const checkedAt = ref(0)
const error = ref('')

let off: (() => void) | null = null

export function useUpdateCheck() {
  // What the app knows about updates, in the one shape every window draws from. A reply that has no
  // time on it is a check that never ran: the line stays empty rather than claiming the app is up to
  // date on the strength of a question nobody asked.
  function apply(info: UpdateInfo | null) {
    if (!info) return
    latest.value = info.latest ?? ''
    notes.value = info.notes ?? []
    checkedAt.value = info.checkedAt ?? 0
    // An install in flight is the one state a reply must not overwrite: the app is about to replace
    // itself, and no answer from before that is more true than the screen already saying so.
    if (phase.value === 'installing') return
    if (!info.checkedAt) {
      phase.value = 'idle'
      return
    }
    phase.value = info.available ? 'available' : 'uptodate'
  }

  async function check() {
    if (phase.value === 'checking' || phase.value === 'installing') return
    phase.value = 'checking'
    error.value = ''
    try {
      apply(await UpdateService.Check())
    } catch {
      error.value = tr('errors.updateCheckFailed')
      phase.value = 'error'
    }
  }

  async function install() {
    if (!latest.value || phase.value === 'installing') return
    phase.value = 'installing'
    error.value = ''
    try {
      // The app replaces its own binary and relaunches, so this call does not return.
      await UpdateService.Install(latest.value)
    } catch (e) {
      error.value = tr('errors.updateFailed', { error: describeFailure(e) })
      phase.value = 'error'
    }
  }

  // "Пропустить эту версию": the backend answers with what it has to say now, and every window is
  // told as well — the window that asked is closing, and the ones staying open are showing the
  // release it just dropped.
  async function skip() {
    try {
      apply(await UpdateService.Skip())
    } catch (e) {
      error.value = tr('errors.updateFailed', { error: describeFailure(e) })
      phase.value = 'error'
    }
  }

  function openWindow() {
    UpdateService.ShowWindow()
  }

  // A check asked for from the main window or the menu. Both entry points take the parked request:
  // the method clears it, so exactly one of them acts on it.
  async function checkIfRequested() {
    try {
      if (await UpdateService.TakeRequest()) check()
    } catch {
      // Nothing to do: the window is usable without a check.
    }
  }

  onMounted(async () => {
    if (!off) off = Events.On('update-changed', (ev) => apply(ev.data as UpdateInfo))
    try {
      apply(await UpdateService.Status())
    } catch {
      // No backend answer (or a plain browser): the line stays on "no check yet".
    }
    await checkIfRequested()
  })

  onBeforeUnmount(() => {
    off?.()
    off = null
  })

  return { phase, latest, notes, checkedAt, error, check, install, skip, openWindow }
}
