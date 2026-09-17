<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import EnvironmentCreatePane from './EnvironmentCreatePane.vue'
import EnvironmentDetailPane from './EnvironmentDetailPane.vue'
import EnvironmentDangerSheet from './EnvironmentDangerSheet.vue'
import EnvironmentList from './EnvironmentList.vue'
import ImportSheet from './ImportSheet.vue'
import { useEnvironmentsStore } from '../../stores/environments'
import { useRequestsStore } from '../../stores/requests'
import { useSheetNotice } from '../../composables/useSheetNotice'
import { parseTokens } from '../../lib/vars'
import { EnvironmentsService } from '../../../bindings/json-inspector/internal/transport/wails'
import type { Entry } from '../../../bindings/json-inspector/internal/dotenv'
import { describeFailure, useMessages } from '../../i18n'

// The window: the list of scopes on the left and, on the right, either the form that makes a new one
// or the environment being looked at. It holds the three things neither side can hold alone — which
// pane the right-hand side is showing, the `.env` import (both panes ask for it, and the file is read
// once), and the confirmation that stands over everything before a scope is taken away.
const emit = defineEmits<{ (e: 'close'): void }>()

const envStore = useEnvironmentsStore()
const reqStore = useRequestsStore()
const { setNotice, clearNotice } = useSheetNotice()
const { t } = useMessages()

const creating = ref(false)
const detailRef = ref<InstanceType<typeof EnvironmentDetailPane> | null>(null)

const envId = computed(() => envStore.editedEnvId)
const env = computed(() => envStore.environments.find((e) => e.id === envId.value) ?? null)
const isGlobals = computed(() => envId.value === null)

function select(id: string | null) {
  envStore.editEnv(id)
  clearNotice()
}

// ---- the .env import ------------------------------------------------------------------------------

const fileInput = ref<HTMLInputElement | null>(null)
const importSheet = ref(false)
// An import asked for while the create form is open has no environment to go into yet, so it waits
// here and is written into the one that comes out of the form.
const pendingEntries = ref<Entry[] | null>(null)
const pendingCount = computed(() => pendingEntries.value?.length ?? 0)

function pickFile() {
  fileInput.value?.click()
}

async function onFileChosen(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  // Reset immediately, so choosing the same file twice still fires a change.
  input.value = ''
  if (!file) return

  // Reading the file is Go's job: what it finds is what goes in, by the three rules the sheet states.
  const entries = (await EnvironmentsService.ParseDotenv(await file.text())) ?? []
  importSheet.value = false

  // A file with nothing in it says so: the import would otherwise look like it worked, since the
  // sheet closes either way and the table looks the same whether nothing was read or nothing matched.
  if (entries.length === 0) {
    setNotice(t('environments.importEmpty'))
    return
  }

  // The window's rule holds however the import was asked for: a closed environment is not written
  // into, and the sheet says which switch opens it rather than filling it behind the user's back.
  if (!isGlobals.value && env.value?.readonly) {
    setNotice(t('environments.locked'))
    return
  }

  if (creating.value) {
    pendingEntries.value = entries
    return
  }

  // How many keys arrived is the one thing the table cannot show: the ones that were already there
  // keep their place, so a file of ten keys into an environment of ten leaves a screen that looks
  // untouched.
  void envStore
    .importDotenv(isGlobals.value ? null : envId.value, entries)
    .then(() => setNotice(t('environments.imported', { n: entries.length })))
    .catch(refused)
}

// A write that fails says so in the window's own line rather than doing nothing: the calls below are
// the ones that can be refused — a name the database already holds, a file with nothing in it — and
// a silence would leave the user pressing a button that appears not to be wired.
function refused(error: unknown) {
  setNotice(describeFailure(error))
}

// ---- making one, and taking one away --------------------------------------------------------------

async function created() {
  creating.value = false
  const made = envStore.editedEnvId
  if (pendingEntries.value && made) {
    const entries = pendingEntries.value
    pendingEntries.value = null
    try {
      await envStore.importDotenv(made, entries)
    } catch (error) {
      refused(error)
    }
  }
}

// A name a saved request still carries is worth saying out loud: after the delete its token stops
// resolving, and the request that holds it is not the one being looked at.
function isNameReferenced(): boolean {
  const texts: string[] = [reqStore.url, reqStore.body]
  for (const p of reqStore.params) texts.push(p.name, p.value)
  for (const h of reqStore.headers) texts.push(h.name, h.value)
  // History keeps a record's URL and headers in full but not its bodies — a body stays in the
  // database until something opens it — so a name that only ever appeared inside one is missed
  // here. That is a hint not given, not a wrong answer.
  for (const r of reqStore.records) {
    texts.push(r.url)
    for (const h of r.requestHeaders ?? []) texts.push(h.name, h.value)
  }
  const names = new Set((env.value?.vars ?? []).map((v) => v.name))
  if (names.size === 0) return false
  return texts.some((text) => text && parseTokens(text).some((tok) => names.has(tok.name)))
}

const danger = ref<'delete' | 'clear' | null>(null)

async function confirmDanger() {
  const kind = danger.value
  danger.value = null

  try {
    await (kind === 'clear' ? clear() : remove())
  } catch (error) {
    refused(error)
  }
}

async function clear() {
  await envStore.clearGlobals()
}

async function remove() {
  const gone = envId.value
  if (!gone) return
  await envStore.removeEnv(gone)
  // Deleting what you were looking at leaves the pane with nothing to show: it moves to what is left,
  // which is why the write is waited for — the list this picks from has to be the one without it.
  if (envStore.editedEnvId === gone) envStore.editEnv(envStore.environments[0]?.id ?? null)
}

function onKeydown(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  if (danger.value) {
    danger.value = null
    return
  }
  if (importSheet.value) {
    importSheet.value = false
    return
  }
  if (creating.value) {
    creating.value = false
    return
  }
  if (detailRef.value?.cancelTop()) return
  emit('close')
}

onMounted(() => {
  clearNotice()
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="sheet-overlay">
    <div class="sheet">
      <div class="sheet-head">
        <span class="sheet-title">{{ t('environments.title') }}</span>
        <span class="head-spacer"></span>
        <button type="button" class="done" @click="emit('close')">{{ t('environments.done') }}</button>
      </div>

      <div class="sheet-body">
        <!-- The list steps aside while a new environment is being made: the form is the only thing to
             do at that moment, and a row to click beside it would only take the room the name and the
             choice of a base want. Cancel and Escape are the way back, and they are both on screen. -->
        <EnvironmentList v-if="!creating" @select="select" @create="creating = true" />

        <EnvironmentCreatePane
          v-if="creating"
          :pending-count="pendingCount"
          @cancel="creating = false"
          @created="created"
          @import="importSheet = true"
        />
        <EnvironmentDetailPane
          v-else
          ref="detailRef"
          @import="importSheet = true"
          @danger="danger = isGlobals ? 'clear' : 'delete'"
        />
      </div>

      <input
        ref="fileInput"
        type="file"
        accept=".env,text/plain"
        class="file-input"
        @change="onFileChosen"
      />

      <EnvironmentDangerSheet
        v-if="danger"
        :kind="danger"
        :referenced="danger === 'delete' && isNameReferenced()"
        @close="danger = null"
        @confirm="confirmDanger"
      />

      <ImportSheet
        v-if="importSheet"
        @close="importSheet = false"
        @choose="pickFile"
      />
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.sheet-overlay {
  @apply fixed inset-0 z-1500 flex items-center justify-center;
  padding: 28px;
  background: rgba(0, 0, 0, 0.22);
}

/* The whole window is one acrylic leaf — blurring only the toolbar left a glass strip lying on an
   ordinary card. The strips inside it are veils of the same shade, not a fill of their own, and the
   lines between them are the window's own hairline rather than the leaf's: the drawing draws the
   head and the rail with `--line`, and a sheet's inner edges are a touch lighter than its outer. */
.sheet {
  @apply flex flex-col w-full h-full max-w-[900px] max-h-[580px] rounded-[14px] overflow-hidden;
  background: var(--glass-sheet);
  backdrop-filter: var(--blur-sheet);
  box-shadow: var(--glass-sheet-shadow), 0 0 0 1px var(--glass-overlay-border);
}

.sheet-head {
  @apply flex-none flex items-center gap-3 h-[50px] px-4 border-b;
  background: var(--glass-sheet-head);
  border-color: var(--border);
}

.sheet-title {
  @apply text-sm font-semibold;
}

/* The head's one button: a field's fill and hairline rather than a bar's, because it ends a form
   rather than standing in a row of controls. */
.done {
  @apply flex-none h-[30px] px-3.5 rounded-lg cursor-pointer text-text text-[13px] font-medium;
  font-family: inherit;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
}

.done:hover {
  @apply bg-bg-hover;
}

.sheet-body {
  @apply flex flex-1 min-h-0;
}

.file-input {
  @apply hidden;
}
</style>
