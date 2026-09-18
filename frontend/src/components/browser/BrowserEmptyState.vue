<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { BridgeService } from '../../../bindings/json-inspector/internal/transport/wails'
import { Browser } from '@wailsio/runtime'
import { useRequestsStore } from '../../stores/requests'
import EmptyPage from '../ui/EmptyPage.vue'
import { Button } from '../ui/button'
import { useMessages } from '../../i18n'

// The Browser rail with nothing captured yet: what the extension is for, how it is put to work, and
// where the app is listening. It is a page rather than a line of prose because the three steps are
// what the section is waiting for, and a step is easier to see than to read.
const store = useRequestsStore()

const { t } = useMessages()

const port = ref('')

onMounted(async () => {
  try {
    port.value = String(await BridgeService.Port())
  } catch {
    // Runtime not ready yet.
  }
})

const connected = computed(() => store.capture.connected)

const statusText = computed(() =>
  connected.value
    ? t('browser.connected', { port: port.value || '…' })
    : t('browser.notFound', { port: port.value || '…' })
)

// The steps are the mockup's three, with the one thing it could not know left to the app: how this
// extension gets installed. It is a folder in the repository, in developer mode, not a store.
const steps = computed(() => [
  { n: '1', title: t('browser.stepInstall'), body: t('browser.stepInstallBody') },
  { n: '2', title: t('browser.stepTab'), body: t('browser.stepTabBody') },
  { n: '3', title: t('browser.stepCapture'), body: t('browser.stepCaptureBody') },
])

function openInstructions() {
  Browser.OpenURL('https://github.com/Jurager/json-inspector')
}
</script>

<template>
  <EmptyPage
    :badge="statusText"
    :tone="connected ? 'ok' : 'flat'"
    :title="t('browser.homeTitle')"
    :body="t('browser.homeBody')"
    :steps="steps"
  >
    <template #actions>
      <Button variant="primary" size="xl" @click="openInstructions">
        {{ t('browser.install') }}
      </Button>
    </template>

    <template #note>{{ t('browser.listening', { port: port || '…' }) }}</template>
  </EmptyPage>
</template>
