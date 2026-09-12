import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { App as Backend } from '../../bindings/json-inspector'
import { useToast } from './useToast'

export interface UpdateInfo {
  available: boolean
  current: string
  latest: string
}

const availableUpdate = ref<UpdateInfo | null>(null)
const isChecking = ref(false)
const isInstalling = ref(false)
const isModalOpen = ref(false)

// Three components use this composable, and the events must be subscribed once.
let listening = false

export function useUpdates() {
  const { show } = useToast()
  const offs: (() => void)[] = []

  function openModal() {
    isModalOpen.value = true
  }

  function closeModal() {
    isModalOpen.value = false
  }

  async function checkForUpdates() {
    if (isChecking.value) return
    isChecking.value = true
    try {
      const u = await Backend.CheckForUpdates()
      // The binding types the Go pointer as nullable; CheckForUpdates returns a
      // result or an error, never nil.
      if (!u) return
      if (u.available) {
        availableUpdate.value = u
        // A manual check opens the modal; the startup one only lights the
        // status-bar link.
        isModalOpen.value = true
      } else show(`У вас последняя версия (${u.latest})`)
    } catch {
      show('Не удалось проверить обновления', 'error')
    } finally {
      isChecking.value = false
    }
  }

  async function installUpdate() {
    if (!availableUpdate.value || isInstalling.value) return
    isInstalling.value = true
    const version = availableUpdate.value.latest
    try {
      await Backend.UpdateNow(version)
    } catch (e) {
      show(`Не удалось обновиться: ${e}`, 'error')
    } finally {
      isInstalling.value = false
    }
  }

  onMounted(() => {
    if (listening) return
    listening = true
    offs.push(
      Events.On('update-available', (ev) => {
        availableUpdate.value = ev.data as UpdateInfo
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

  return {
    availableUpdate,
    isChecking,
    isInstalling,
    isModalOpen,
    openModal,
    closeModal,
    checkForUpdates,
    installUpdate,
  }
}
