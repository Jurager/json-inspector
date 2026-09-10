<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { CheckForUpdates, UpdateNow, Version, ShowAbout } from '../../wailsjs/go/main/App'

interface UpdateInfo {
  available: boolean
  current: string
  latest: string
}

const currentVersion = ref('')
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

onMounted(async () => {
  try {
    currentVersion.value = await Version()
  } catch {
    currentVersion.value = ''
  }
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
    <button class="nav-item" @click="check">
      <span class="nav-icon"><Icon name="arrow-down" /></span> {{ checking ? 'Проверка…' : 'Проверить обновления' }}
    </button>
    <button v-if="currentVersion" class="updater-version" title="О программе" @click="ShowAbout()">v{{ currentVersion }}</button>

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
.updater {
  margin-top: auto;
  padding-top: 8px;
  border-top: 1px solid var(--border);
}

.updater-version {
  display: block;
  width: 100%;
  text-align: left;
  padding: 2px 10px;
  font-size: 11px;
  color: var(--text-tertiary);
  border: none;
  background: transparent;
  cursor: pointer;
  border-radius: 6px;
  font-family: inherit;
}

.updater-version:hover {
  color: var(--text-secondary);
  background: var(--bg-hover);
}

.toast {
  position: fixed;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%);
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 12px;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  box-shadow: var(--shadow);
  z-index: 2000;
  max-width: 80%;
}

.toast.error {
  color: var(--red);
}

.toast.info {
  color: var(--text);
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1500;
}

.modal {
  width: 360px;
  max-width: 90%;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 12px;
  box-shadow: var(--shadow);
  padding: 18px;
}

.modal-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 8px;
}

.modal-body {
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 16px;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
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
