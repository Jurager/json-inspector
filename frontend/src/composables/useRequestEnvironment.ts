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

  // A pin nothing answers to: the environment was deleted while the request was pointed at it. It is
  // not the same trouble as having no environment at all — the window has one, and the request is the
  // thing that has to be told where to go — so the two are told apart wherever the difference shows.
  const stale = computed(() => pinned.value && pinnedEnv.value === null)

  // What the button says: the pinned environment's name, or — for a pin nothing answers to — the
  // window's, which is what the request has fallen back to.
  const label = computed(() => (pinned.value ? (pinnedEnv.value?.name ?? windowName.value) : windowName.value))

  // The environment a sentence about this request should name, which for a pinned one is not the
  // window's. Nothing when there is no environment at all: the sentence then says that instead.
  const answersIn = computed(() =>
    pinned.value ? pinnedEnv.value?.name : envStore.activeEnvironment?.name
  )

  // Where a `{{token}}` written into this request would go, and whether it may go there at all. A
  // request that answers in an environment of its own puts its variables in that one: a name the
  // sentence says is missing from «Prod» cannot be created in «Local» and be found.
  //
  // A stale pin is the one case where this is not the pin: nothing answers to that id, so the request
  // is answering in no environment rather than in a deleted one, and the window's own is what it
  // falls back to — which is also where a variable for it belongs.
  const scopeId = computed(() => (pinned.value ? (pinnedEnv.value?.id ?? null) : envStore.activeId))

  const scopeReadonly = computed(() =>
    pinned.value ? Boolean(pinnedEnv.value?.readonly) : Boolean(envStore.activeEnvironment?.readonly)
  )

  // Every choice the popover offers: following the window, or one environment named outright. The
  // window's own name is in the first row's words, so a person can see what "follow" means right now
  // without opening anything else — and what that row answers for is the globals, which apply under
  // every environment, so it is the one row whose second column is a word rather than a count.
  const options = computed(() => [
    { value: '', name: `${t('request.envFollow')} · ${windowName.value}`, vars: t('request.envFollowScope') },
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

  return { windowName, pinned, stale, label, answersIn, scopeId, scopeReadonly, options, choose }
}
