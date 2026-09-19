import { computed } from 'vue'
import type { RequestSource } from '../lib/requestSource'
import { useEnvironmentsStore } from '../stores/environments'
import { formatNumber, useMessages } from '../i18n'

// Which environment a request goes out under: the window's own, or one it pinned for itself.
//
// Two places need the same answer — the button in the bar, and the sentence that names the
// environment a missing variable is missing from — and the answer has a rule in it: a pin that names
// an environment which is no longer there reads as no environment at all, the way Go reads the same
// id, rather than quietly as the window's. The rule lives here so the two cannot disagree about it,
// and so a third reader gets it rather than re-deriving it.
export function useRequestEnvironment(source: RequestSource) {
  const envStore = useEnvironmentsStore()
  const { t } = useMessages()

  // What the window is on, which is what an unpinned request follows.
  const windowName = computed(() => envStore.activeEnvironment?.name ?? t('titlebar.noEnvironment'))

  const pinned = computed(() => Boolean(source.environmentId))

  const pinnedEnv = computed(
    () => envStore.environments.find((e) => e.id === source.environmentId) ?? null
  )

  // What the button says: the pinned environment's name, or — for a pin nothing answers to — the
  // window's, which is what the request has fallen back to.
  const label = computed(() => (pinned.value ? (pinnedEnv.value?.name ?? windowName.value) : windowName.value))

  // The environment a sentence about this request should name, which for a pinned one is not the
  // window's. Nothing when there is no environment at all: the sentence then says that instead.
  const answersIn = computed(() =>
    pinned.value ? pinnedEnv.value?.name : envStore.activeEnvironment?.name
  )

  // Every choice the popover offers: following the window, or one environment named outright. The
  // window's own name is in the first row's words, so a person can see what "follow" means right now
  // without opening anything else.
  const options = computed(() => [
    { value: '', name: t('request.envFollow', { name: windowName.value }), vars: '' },
    ...envStore.environments.map((e) => ({
      value: e.id,
      name: e.name,
      vars: formatNumber(e.vars.length),
    })),
  ])

  function choose(id: string) {
    if (id === source.environmentId) return
    void source.setEnvironmentOverride(id)
  }

  return { windowName, pinned, label, answersIn, options, choose }
}
