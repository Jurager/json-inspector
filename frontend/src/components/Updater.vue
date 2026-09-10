<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { CheckForUpdates, UpdateNow } from '../../wailsjs/go/main/App'

interface UpdateInfo {
  available: boolean
  current: string
  latest: string
}

const checking = ref(false)
const updating = ref(false)
const update = ref<UpdateInfo | null>(null)
const toast = ref('')
const toastType = ref<'info' | 'error'>('info')

let toastTimer: ReturnType<typeof setTimeout> | null = null
const offs: (() => void)[] = []

function showToast(msg: string, type: 'info' | 'error' = 'info') {
  toast.value = msg
  toastType.value = type
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (toast.value = ''), 5000)
}

async function check() {
  if (checking.value) return
  checking.value = true
  try {
    const u = await CheckForUpdates()
    if (u.available) update.value = u
    else showToast(`У вас последняя версия (${u.latest})`)
  } catch (e) {
    showToast(`Не удалось проверить обновления`, 'error')
  } finally {
    checking.value = false
  }
}

async function doUpdate() {
  if (!update.value || updating.value) return
  updating.value = true
  const v = update.value.latest
  try {
    await UpdateNow(v)
  } catch (e) {
    showToast(`Не удалось обновиться: ${e}`, 'error')
  } finally {
    updating.value = false
  }
}

onMounted(() => {
  offs.push(
    EventsOn('update-available', (u: UpdateInfo) => {
      update.value = u
    })
  )
  offs.push(
    EventsOn('update-up-to-date', (u: UpdateInfo) => {
      showToast(`У вас последняя версия (${u.latest})`)
    })
  )
  offs.push(
    EventsOn('update-error', (msg: string) => {
      showToast(msg, 'error')
    })
  )
})

onBeforeUnmount(() => {
  offs.forEach((off) => off())
  if (toastTimer) clearTimeout(toastTimer)
})
</script>

<template>
  <div class="updater">
    <button class="menu-item" @click="check">
      <Icon name="arrow-down" :size="14" /> {{ checking ? 'Проверка…' : 'Проверить обновления' }}
    </button>

    <transition name="fade">
      <div v-if="toast" class="toast" :class="toastType">{{ toast }}</div>
    </transition>

    <div v-if="update" class="modal-overlay" @click.self="update = null">
      <div class="modal">
        <div class="modal-title">Доступна новая версия</div>
        <div class="modal-body">
          Версия <b>{{ update.latest }}</b> (у вас {{ update.current }}).<br />
          Обновить сейчас? Приложение перезапустится.
        </div>
        <div class="modal-actions">
          <button class="btn" @click="update = null">Позже</button>
          <button class="btn btn-primary" :disabled="updating" @click="doUpdate">
            {{ updating ? 'Обновление…' : 'Обновить' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

.toast {
  @apply fixed bottom-4 left-1/2 px-4 py-2 rounded-lg text-xs z-2000 max-w-[80%];
  transform: translateX(-50%);
  background: var(--bg-panel);
  border: 1px solid var(--border);
  box-shadow: var(--shadow);
}

.toast.error {
  @apply text-red;
}

.toast.info {
  @apply text-text;
}

.modal-overlay {
  @apply fixed inset-0 flex items-center justify-center z-1500;
  background: rgba(0, 0, 0, 0.4);
}

.modal {
  @apply w-90 max-w-[90%] rounded-xl p-4.5;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  box-shadow: var(--shadow);
}

.modal-title {
  @apply text-[15px] font-semibold mb-2;
}

.modal-body {
  @apply text-[13px] text-text-secondary mb-4;
}

.modal-actions {
  @apply flex justify-end gap-2;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
