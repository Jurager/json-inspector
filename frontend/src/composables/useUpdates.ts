import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { App as Backend } from '../../bindings/json-inspector'
import { useToast } from './useToast'

export interface UpdateInfo {
  available: boolean
  current: string
  latest: string
}

const update = ref<UpdateInfo | null>(null)
const checking = ref(false)
const updating = ref(false)
const modalOpen = ref(false)

// Three components use this composable, and the events must be subscribed once.
let listening = false

// Everything about the app's own updater in one place: the rail starts a
// check, the status bar offers the result, the modal applies it, and the
// events the Go side emits on its own (a startup check, a menu click) land
// here too rather than in the shell.
export function useUpdates() {
  const { show } = useToast()
  const offs: (() => void)[] = []

  function open() {
    modalOpen.value = true
  }

  function close() {
    modalOpen.value = false
  }

  async function check() {
    if (checking.value) return
    checking.value = true
    try {
      const u = await Backend.CheckForUpdates()
      // The binding types the Go pointer as nullable; CheckForUpdates returns a
      // result or an error, never nil.
      if (!u) return
      if (u.available) {
        update.value = u
        // A manual check gives immediate feedback, unlike the silent startup
        // check which only lights up the status-bar link.
        modalOpen.value = true
      } else show(`У вас последняя версия (${u.latest})`)
    } catch {
      show('Не удалось проверить обновления', 'error')
    } finally {
      checking.value = false
    }
  }

  async function apply() {
    if (!update.value || updating.value) return
    updating.value = true
    const version = update.value.latest
    try {
      await Backend.UpdateNow(version)
    } catch (e) {
      show(`Не удалось обновиться: ${e}`, 'error')
    } finally {
      updating.value = false
    }
  }

  onMounted(() => {
    if (listening) return
    listening = true
    offs.push(
      Events.On('update-available', (ev) => {
        // The startup check only advertises itself in the status bar.
        update.value = ev.data as UpdateInfo
      }),
      Events.On('update-up-to-date', (ev) => {
        const u = ev.data as UpdateInfo
        show(`У вас последняя версия (${u.latest})`)
      }),
      Events.On('update-error', (ev) => {
        show(ev.data as string, 'error')
      })
    )
  })

  onBeforeUnmount(() => offs.forEach((off) => off()))

  return { update, checking, updating, modalOpen, open, close, check, apply }
}
