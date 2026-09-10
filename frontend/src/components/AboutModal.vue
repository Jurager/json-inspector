<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Version } from '../../wailsjs/go/main/App'
import logoUrl from '../assets/logo.svg'

const emit = defineEmits<{ (e: 'close'): void }>()

const version = ref('')

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

onMounted(async () => {
  try {
    version.value = await Version()
  } catch {
    version.value = ''
  }
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="about-overlay" @click.self="emit('close')">
    <div class="about">
      <img class="about-icon" :src="logoUrl" alt="JSON Inspector" draggable="false" />
      <div class="about-name">JSON Inspector</div>
      <div class="about-version">Версия {{ version || '…' }}</div>
      <div class="about-desc">Инструмент для удобной работы с JSON:API-ответами.</div>
      <div class="about-tech">Go · Wails · Vue</div>
      <div class="about-copy">© {{ new Date().getFullYear() }} Yuri Gerasimov</div>
      <button class="btn" @click="emit('close')">Закрыть</button>
    </div>
  </div>
</template>

<style scoped>
.about-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 3000;
}

.about {
  width: 320px;
  max-width: 90%;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 14px;
  box-shadow: var(--shadow);
  padding: 28px 24px 20px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.about-icon {
  width: 88px;
  height: 88px;
  margin-bottom: 10px;
  -webkit-user-drag: none;
  user-drag: none;
}

.about-name {
  font-size: 17px;
  font-weight: 700;
}

.about-version {
  font-size: 13px;
  color: var(--text-secondary);
}

.about-desc {
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 6px;
  line-height: 1.4;
}

.about-tech {
  font-size: 11px;
  color: var(--text-tertiary);
}

.about-copy {
  font-size: 11px;
  color: var(--text-tertiary);
  margin-top: 8px;
}

.about .btn {
  margin-top: 14px;
}
</style>
