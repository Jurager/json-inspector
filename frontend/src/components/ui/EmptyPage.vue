<script setup lang="ts">
import Icon from './Icon.vue'

// The page a section shows before there is anything in it: what the section is for, the one thing to
// start it with, the three steps that get there, and the fact somebody would otherwise have to look up.
//
// One component for two sections because the drawing gives them one shape — the browser's page and the
// collections' are the same page with different words — and because a third would otherwise be a third
// copy of numbers that have to agree.
defineProps<{
  /** The state, in a chip: what there is none of, and where. */
  badge: string
  /** The chip's tone: `ok` is for the one state on such a page that is good news. */
  tone?: 'flat' | 'ok'
  title: string
  body: string
  steps: { n: string; title: string; body: string }[]
}>()

// The page's actions are the caller's — a section starts in its own way — and so is the note: the
// strip draws the mark and the text slot, and a page with a button to hang there can fill `note-action`.
</script>

<template>
  <div class="page">
    <div class="head">
      <span class="chip" :class="{ ok: tone === 'ok' }">
        <span class="dot"></span>{{ badge }}
      </span>
      <span class="title">{{ title }}</span>
      <span class="body">{{ body }}</span>
    </div>

    <div class="actions">
      <slot name="actions" />
    </div>

    <div class="steps">
      <div v-for="step in steps" :key="step.n" class="step">
        <span class="step-num">{{ step.n }}</span>
        <span class="step-title">{{ step.title }}</span>
        <span class="step-body">{{ step.body }}</span>
      </div>
    </div>

    <div class="note">
      <span class="note-icon"><Icon name="info" :size="18" :stroke-width="1.8" /></span>
      <span class="note-text"><slot name="note" /></span>
      <slot name="note-action" />
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.page {
  /* The drawing's own lines: the window's base is Tailwind's 1.5, which leaves a 26px title taller
     than the handoff's. */
  line-height: normal;
  @apply flex-1 min-h-0 overflow-y-auto flex flex-col gap-7;
  padding: 40px 44px;
}

.head {
  @apply flex flex-col gap-2.5 max-w-[620px];
}

/* The window's own vocabulary for the state, in the badge's clothes. */
.chip {
  @apply inline-flex items-center gap-[9px] self-start py-[5px] px-2.5 rounded-[7px]
         text-[12px] font-semibold text-text-secondary bg-bg-hover;
}

.chip.ok {
  @apply bg-green-soft;
  color: var(--green-text);
}

.dot {
  @apply w-2 h-2 rounded-full bg-current flex-none;
}

.title {
  @apply text-[26px] font-semibold;
  letter-spacing: -0.02em;
}

.body {
  @apply text-[14px] text-text-secondary;
  line-height: 1.55;
}

.actions {
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
         text-[13px] font-bold text-accent bg-accent-soft;
}

.step-title {
  @apply text-[14px] font-semibold;
}

.step-body {
  @apply text-[13px] text-text-secondary;
  line-height: 1.5;
}

/* The one fact the page keeps a line for: what the section takes, or where it is listening — the thing
   nobody can work out from the steps. */
.note {
  @apply flex items-center gap-3.5 border border-border rounded-xl bg-bg-inset max-w-[1000px];
  padding: 16px 18px;
}

.note-icon {
  @apply inline-flex items-center justify-center flex-none w-[34px] h-[34px] rounded-[8px]
         bg-bg-hover text-text-secondary;
}

.note-text {
  @apply flex-1 min-w-0 text-[13px] text-text-secondary;
  line-height: 1.5;
}
</style>
