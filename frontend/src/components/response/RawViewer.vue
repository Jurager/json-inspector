<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { EditorState, RangeSetBuilder, StateEffect, StateField, type Text } from '@codemirror/state'
import { Decoration, EditorView, lineNumbers, type DecorationSet } from '@codemirror/view'
import { HighlightStyle, codeFolding, foldGutter, foldKeymap, syntaxHighlighting } from '@codemirror/language'
import { json } from '@codemirror/lang-json'
import { keymap } from '@codemirror/view'
import { tags as t } from '@lezer/highlight'

// CodeMirror renders the text itself, so the toolbar's search has to be decorations
// here rather than markup on HTML.
const props = defineProps<{
  text: string
  query: string
}>()

const emit = defineEmits<{
  (e: 'stats', s: { count: number; index: number }): void
}>()

const host = ref<HTMLElement | null>(null)
const view = shallowRef<EditorView | null>(null)

// The app's own token colours, so Raw reads like the tree instead of introducing
// a second palette.
const appHighlight = HighlightStyle.define([
  { tag: t.propertyName, color: 'var(--accent)' },
  { tag: t.string, color: 'var(--tok-str)' },
  { tag: [t.number, t.bool, t.null], color: 'var(--tok-num)' },
  { tag: [t.punctuation, t.separator], color: 'var(--text-tertiary)' },
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

function plural(n: number, forms: [string, string, string]): string {
  const m10 = n % 10
  const m100 = n % 100
  const word =
    m10 === 1 && m100 !== 11
      ? forms[0]
      : m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)
        ? forms[1]
        : forms[2]
  return `${n} ${word}`
}

const folding = codeFolding({
  preparePlaceholder: (state, range) =>
    plural(
      state.doc.lineAt(range.to).number - state.doc.lineAt(range.from).number,
      ['строка', 'строки', 'строк']
    ),
  placeholderDOM: (_view, onclick, prepared: string) => {
    const el = document.createElement('span')
    el.textContent = `⋯ ${prepared}`
    el.title = 'Развернуть'
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

function matches(): Match[] {
  const v = view.value
  if (!v) return []
  return findMatches(v.state.doc, v.state.field(queryField))
}

function stats(): { count: number; index: number } {
  const v = view.value
  const all = matches()
  if (!v || all.length === 0) return { count: 0, index: 0 }
  const pos = v.state.selection.main.from
  const at = all.findIndex((m) => m.from <= pos && pos <= m.to)
  return { count: all.length, index: at === -1 ? 0 : at }
}

function report() {
  emit('stats', stats())
}

// Selecting a match is what makes it visible — the caret sits on it.
function step(direction: 1 | -1) {
  const v = view.value
  if (!v) return
  const all = matches()
  if (all.length === 0) {
    report()
    return
  }
  const pos = v.state.selection.main.from
  const after = all.findIndex((m) => m.from > pos)
  const target =
    direction === 1
      ? all[after === -1 ? 0 : after]
      : all[(after === -1 ? all.length : after) - 1] ?? all[all.length - 1]

  v.dispatch({
    selection: { anchor: target.from, head: target.to },
    scrollIntoView: true,
  })
  report()
}

function applyQuery(query: string) {
  const v = view.value
  if (!v) return
  v.dispatch({ effects: setQuery.of(query) })
  report()
}

function buildState(doc: string): EditorState {
  return EditorState.create({
    doc,
    extensions: [
      lineNumbers(),
      foldGutter(),
      folding,
      keymap.of(foldKeymap),
      json(),
      syntaxHighlighting(appHighlight),
      EditorState.readOnly.of(true),
      EditorView.editable.of(false),
      EditorView.lineWrapping,
      queryField,
      matchField,
      appTheme,
      EditorView.updateListener.of((u) => {
        if (u.selectionSet || u.docChanged) report()
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

watch(() => props.query, applyQuery)

defineExpose({
  next: () => step(1),
  prev: () => step(-1),
})
</script>

<template>
  <div ref="host" class="raw-viewer"></div>
</template>

<style scoped>
@reference "../../style.css";

/* Fills its column and scrolls inside it, so the toolbar above never moves. */
.raw-viewer {
  @apply flex-1 min-h-0 overflow-hidden;
}
</style>
