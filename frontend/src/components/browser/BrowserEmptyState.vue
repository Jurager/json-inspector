<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { BridgeService } from '../../../bindings/json-inspector/internal/transport/wails'
import { Browser } from '@wailsio/runtime'
import { useRequestsStore } from '../../stores/requests'
import Icon from '../ui/Icon.vue'
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
  <div class="home">
    <div class="home-head">
      <span class="state-chip" :class="{ ok: connected }">
        <span class="state-dot"></span>{{ statusText }}
      </span>
      <span class="home-title">{{ t('browser.homeTitle') }}</span>
      <span class="home-body">{{ t('browser.homeBody') }}</span>
    </div>

    <div class="home-actions">
      <Button variant="primary" size="xl" @click="openInstructions">
        {{ t('browser.install') }}
      </Button>
    </div>

    <div class="steps">
      <div v-for="step in steps" :key="step.n" class="step">
        <span class="step-num">{{ step.n }}</span>
        <span class="step-title">{{ step.title }}</span>
        <span class="step-body">{{ step.body }}</span>
      </div>
    </div>

    <div class="listen">
      <span class="listen-icon"><Icon name="info" :size="18" :stroke-width="1.8" /></span>
      <span class="listen-text">{{ t('browser.listening', { port: port || '…' }) }}</span>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.home {
  /* The drawing's own lines: the window's base is Tailwind's 1.5, which leaves a 26px title taller
     than the handoff's. */
  line-height: normal;
  @apply flex-1 min-h-0 overflow-y-auto flex flex-col gap-7;
  padding: 40px 44px;
}

.home-head {
  @apply flex flex-col gap-2.5 max-w-[620px];
}

/* The window's own vocabulary for the state, in the badge's clothes: a connected extension is the
   one thing on this page that is good news. */
.state-chip {
  @apply inline-flex items-center gap-[9px] self-start py-[5px] px-2.5 rounded-[7px]
         text-[12px] font-semibold text-text-secondary bg-bg-hover;
}

.state-chip.ok {
  @apply bg-green-soft;
  color: var(--green-text);
}

.state-dot {
  @apply w-2 h-2 rounded-full bg-current flex-none;
}

.home-title {
  @apply text-[26px] font-semibold;
  letter-spacing: -0.02em;
}

.home-body {
  @apply text-[14.5px] text-text-secondary;
  line-height: 1.55;
}

.home-actions {
  @apply flex items-center gap-3;
}

.steps {
  @apply grid grid-cols-3 gap-3 max-w-[1000px];
}

.step {
  @apply flex flex-col gap-2.5 border border-border rounded-xl p-[18px];
}

.step-num {
  @apply inline-flex items-center justify-center flex-none w-[26px] h-[26px] rounded-full
         text-[12.5px] font-bold text-accent bg-accent-soft;
}

.step-title {
  @apply text-[14px] font-semibold;
}

.step-body {
  @apply text-[13px] text-text-secondary;
  line-height: 1.5;
}

/* Where the app is listening, in the line the page keeps for it: the port is the one number the
   extension asks for, and it is not a step anybody can work out. */
.listen {
  @apply flex items-center gap-3.5 border border-border rounded-xl bg-bg-inset max-w-[1000px];
  padding: 16px 18px;
}

.listen-icon {
  @apply inline-flex items-center justify-center flex-none w-[34px] h-[34px] rounded-[9px]
         bg-bg-hover text-text-secondary;
}

.listen-text {
  @apply flex-1 min-w-0 text-[13.5px] text-text-secondary;
  line-height: 1.5;
}
</style>
