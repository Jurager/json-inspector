<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { EditorState, RangeSetBuilder, StateEffect, StateField, type Text } from '@codemirror/state'
import { Decoration, EditorView, lineNumbers, type DecorationSet } from '@codemirror/view'
import { HighlightStyle, codeFolding, foldGutter, foldKeymap, syntaxHighlighting } from '@codemirror/language'
import { json } from '@codemirror/lang-json'
import { useSettings } from '../../composables/useSettings'
import { useMessages } from '../../i18n'
import { keymap } from '@codemirror/view'
import { tags } from '@lezer/highlight'

// CodeMirror renders the text itself, so the toolbar's search has to be decorations
// here rather than markup on HTML.
const props = defineProps<{
  text: string
  query: string
}>()

const emit = defineEmits<{
  (e: 'stats', s: { count: number; index: number }): void
}>()

// The catalogue's `t`, not lezer's tags: the tags were renamed to their own name above so that these
// two can sit in one file without either being shortened.
const { t } = useMessages()

const host = ref<HTMLElement | null>(null)
const view = shallowRef<EditorView | null>(null)

// What the reader asked the viewer to do, from Go's settings: a missing value is the default, which
// is what the viewer did before either of them could be changed.
const { settings } = useSettings()
const wrap = computed(() => settings.value?.wrapLines ?? true)
const numbers = computed(() => settings.value?.lineNumbers ?? true)

// The app's own token colours, so Raw reads like the tree instead of introducing
// a second palette.
const appHighlight = HighlightStyle.define([
  { tag: tags.propertyName, color: 'var(--accent)' },
  { tag: tags.string, color: 'var(--tok-str)' },
  { tag: [tags.number, tags.bool, tags.null], color: 'var(--tok-num)' },
  { tag: [tags.punctuation, tags.separator], color: 'var(--text-tertiary)' },
])

const appTheme = EditorView.theme({
  '&': { backgroundColor: 'transparent', height: '100%', fontSize: '12px' },
  '&.cm-focused': { outline: 'none' },
  '.cm-scroller': { fontFamily: 'var(--mono)', lineHeight: '1.65' },
  // The app turns selection off globally and that inherits in here — a response
  // is text and has to stay selectable and copyable.
  '.cm-content': { padding: '10px 0', userSelect: 'text', WebkitUserSelect: 'text' },
  '.cm-line': { padding: '0 16px' },
  '.cm-gutters': {
    backgroundColor: 'transparent',
    border: 'none',
    color: 'var(--text-tertiary)',
    paddingLeft: '6px',
  },
  '.cm-foldGutter span': { color: 'var(--text-tertiary)', cursor: 'pointer' },
  '.cm-foldGutter span:hover': { color: 'var(--text)' },
  '.cm-foldPlaceholder': {
    backgroundColor: 'var(--bg-inset)',
    border: '1px solid var(--border)',
    borderRadius: '4px',
    color: 'var(--text-secondary)',
    padding: '0 6px',
    margin: '0 4px',
    cursor: 'pointer',
  },
  '.cm-searchMatch': { backgroundColor: 'color-mix(in srgb, var(--accent) 25%, transparent)' },
  '.cm-selectionBackground, ::selection': { backgroundColor: 'var(--accent-soft)' },
})

const folding = codeFolding({
  preparePlaceholder: (state, range) =>
    t('response.raw.foldedLines', state.doc.lineAt(range.to).number - state.doc.lineAt(range.from).number),
  placeholderDOM: (_view, onclick, prepared: string) => {
    const el = document.createElement('span')
    el.textContent = `⋯ ${prepared}`
    el.title = t('response.raw.expand')
    el.addEventListener('click', onclick)
    return el
  },
})

// Search is hand-written rather than @codemirror/search: that extension highlights
// only while its own panel is open, and the search UI here lives in the app toolbar.

interface Match {
  from: number
  to: number
}

// A 2 MB body can hold tens of thousands of matches, and decorating them all
// costs more than it tells anyone.
const MAX_MARKS = 2000

function findMatches(doc: Text, query: string): Match[] {
  const needle = query.trim().toLowerCase()
  if (!needle) return []
  const hay = doc.toString().toLowerCase()
  const out: Match[] = []
  let i = hay.indexOf(needle)
  while (i !== -1 && out.length < MAX_MARKS) {
    out.push({ from: i, to: i + needle.length })
    i = hay.indexOf(needle, i + needle.length)
  }
  return out
}

const setQuery = StateEffect.define<string>()

const queryField = StateField.define<string>({
  create: () => '',
  update(value, tr) {
    for (const e of tr.effects) if (e.is(setQuery)) return e.value
    return value
  },
})

const matchMark = Decoration.mark({ class: 'cm-searchMatch' })

const matchField = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(deco, tr) {
    // Recompute only on query/doc change — scrolling must not rebuild a set per match.
    const touched = tr.docChanged || tr.effects.some((e) => e.is(setQuery))
    if (!touched) return deco.map(tr.changes)
    const builder = new RangeSetBuilder<Decoration>()
    for (const m of findMatches(tr.state.doc, tr.state.field(queryField))) {
      builder.add(m.from, m.to, matchMark)
    }
    return builder.finish()
  },
  provide: (f) => EditorView.decorations.from(f),
})

function foundMatches(): Match[] {
  const v = view.value
  if (!v) return []
  return findMatches(v.state.doc, v.state.field(queryField))
}

function matchStats(): { count: number; index: number } {
  const v = view.value
  const all = foundMatches()
  if (!v || all.length === 0) return { count: 0, index: 0 }
  const pos = v.state.selection.main.from
  const at = all.findIndex((m) => m.from <= pos && pos <= m.to)
  return { count: all.length, index: at === -1 ? 0 : at }
}

function emitMatchStats() {
  emit('stats', matchStats())
}

// Selecting a match is what makes it visible — the caret sits on it.
function moveToMatch(direction: 'next' | 'prev') {
  const v = view.value
  if (!v) return
  const all = foundMatches()
  if (all.length === 0) {
    emitMatchStats()
    return
  }
  const pos = v.state.selection.main.from
  const after = all.findIndex((m) => m.from > pos)
  const target =
    direction === 'next'
      ? all[after === -1 ? 0 : after]
      : all[(after === -1 ? all.length : after) - 1] ?? all[all.length - 1]

  v.dispatch({
    selection: { anchor: target.from, head: target.to },
    scrollIntoView: true,
  })
  emitMatchStats()
}

function applyQuery(query: string) {
  const v = view.value
  if (!v) return
  v.dispatch({ effects: setQuery.of(query) })
  emitMatchStats()
}

function buildState(doc: string): EditorState {
  return EditorState.create({
    doc,
    extensions: [
      // Two of these are the user's: a gutter can be turned off, and so can wrapping. A long response
      // is read either way — down, or across — and which one is the reader's call.
      ...(numbers.value ? [lineNumbers()] : []),
      foldGutter(),
      folding,
      keymap.of(foldKeymap),
      json(),
      syntaxHighlighting(appHighlight),
      EditorState.readOnly.of(true),
      EditorView.editable.of(false),
      ...(wrap.value ? [EditorView.lineWrapping] : []),
      queryField,
      matchField,
      appTheme,
      EditorView.updateListener.of((u) => {
        if (u.selectionSet || u.docChanged) emitMatchStats()
      }),
    ],
  })
}

onMounted(() => {
  if (!host.value) return
  view.value = new EditorView({ state: buildState(props.text), parent: host.value })
  applyQuery(props.query)
})

onBeforeUnmount(() => {
  view.value?.destroy()
  view.value = null
})

// Rebuilding the state (not patching the doc) also drops folding from the previous
// response, which would otherwise hide lines that no longer exist.
watch(
  () => props.text,
  (text) => {
    const v = view.value
    if (!v) return
    v.setState(buildState(text))
    applyQuery(props.query)
  }
)

// The two the user can change while this is on screen: the settings window writes them, Go says so,
// and the editor is rebuilt the same way it is rebuilt for another response. Folding goes with the
// rebuild, which is the price the text watcher above already pays.
watch([wrap, numbers], () => {
  const v = view.value
  if (!v) return
  v.setState(buildState(props.text))
  applyQuery(props.query)
})

watch(() => props.query, applyQuery)

defineExpose({
  next: () => moveToMatch('next'),
  prev: () => moveToMatch('prev'),
})
</script>

<template>
  <div ref="host" class="raw-viewer"></div>
</template>

<style scoped>
@reference "../../style.css";

/* Fills its column and scrolls inside it, so the toolbar above never moves. CodeMirror's own
   theme paints no background of its own (see `appTheme` above) — this is what shows through it. */
.raw-viewer {
  @apply flex-1 min-h-0 overflow-hidden bg-bg-panel;
}
</style>
