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
@reference "../style.css";

.about-overlay {
  @apply fixed inset-0 flex items-center justify-center z-3000;
  background: rgba(0, 0, 0, 0.4);
}

.about {
  @apply w-80 max-w-[90%] rounded-2xl pt-7 px-6 pb-5 text-center flex flex-col items-center gap-1.5;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  box-shadow: var(--shadow);
}

.about-icon {
  @apply w-22 h-22 mb-2.5;
  -webkit-user-drag: none;
  user-drag: none;
}

.about-name {
  @apply text-[17px] font-bold;
}

.about-version {
  @apply text-[13px] text-text-secondary;
}

.about-desc {
  @apply text-xs text-text-secondary mt-1.5 leading-normal;
}

.about-tech {
  @apply text-[11px] text-text-tertiary;
}

.about-copy {
  @apply text-[11px] text-text-tertiary mt-2;
}

.about .btn {
  @apply mt-3.5;
}
</style>
