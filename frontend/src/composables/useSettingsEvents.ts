import { onBeforeUnmount, onMounted } from 'vue'
import { Events } from '@wailsio/runtime'

// What Go says to a settings window that is already open: which category to show. It arrives as an
// event rather than as an address because the window is already painted and reads no URL twice — the
// account menu of another window is what asks.
//
// The subscription lives here rather than in the window for the reason every other one does: the
// event's name belongs beside the subscription and nowhere else, and it has to be dropped exactly
// when the window is.
export function useSettingsEvents(onCategory: (category: string) => void) {
  const offs: (() => void)[] = []

  onMounted(() => {
    offs.push(
      Events.On('settings-tab', (ev) => {
        const asked = ev.data as string
        if (asked) onCategory(asked)
      })
    )
  })

  onBeforeUnmount(() => offs.forEach((off) => off()))
}
