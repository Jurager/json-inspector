import { defineStore } from 'pinia'
import {
  RecordSource,
  SearchKind,
  SearchTarget,
  type Collection,
  type SearchGroup,
  type SearchHit,
} from '../../bindings/json-inspector/internal/domain'
import { SearchService, SystemService } from '../../bindings/json-inspector/internal/transport/wails'
import { kindLabel } from '../lib/searchKinds'
import { asked } from './calls'
import { trailOf } from '../lib/collectionTree'
import { t as tr } from '../i18n'
import { useCollectionsStore } from './collections'
import { useEnvironmentsStore } from './environments'
import { useRequestsStore } from './requests'

// How long the field waits before the question goes over. A palette is typed into a letter at a time,
// and a question per letter would be a query per letter on the other side of the boundary.
const QUERY_PAUSE_MS = 120

/**
 * A command the palette offers besides what is in the database. Its label comes from the catalogue,
 * which is the whole reason the commands are the window's and not Go's: Go has no language to write
 * one in.
 */
export interface Command {
  id: string
  label: string
  icon: string
}

/**
 * One row of the palette. Exactly one side is set: a row from Go, or one of the window's own
 * commands. They are drawn the same and chosen the same, so they are one list with one selection.
 */
export interface PaletteRow {
  hit: SearchHit | null
  command: Command | null
}

/** One heading and the rows under it, ready to draw. */
export interface Section {
  kind: SearchKind
  label: string
  /** How many the area found, which is not the number of rows drawn. */
  total: number
  rows: PaletteRow[]
}

// The timer of the question not yet asked, and which question the answer belongs to. Both are the
// store's own and neither belongs in state: a pending timer is not something the window draws, and a
// number that only says "this answer is stale" is not state either.
let waiting: ReturnType<typeof setTimeout> | null = null
let generation = 0

export const useSearchStore = defineStore('search', {
  state: () => ({
    open: false,
    text: '',
    // Which area the user narrowed to, or nothing for all of them at once.
    kind: null as SearchKind | null,
    groups: [] as SearchGroup[],
    // Which row is selected. The list is walked with the arrows while the caret stays in the field,
    // so this is not the focus.
    activeIndex: 0,
  }),

  getters: {
    /**
     * The words the search runs on. A `>` in front narrows to the commands, as the design draws it,
     * and the sign itself is not a word to search for.
     */
    query(state): string {
      return state.text.replace(/^\s*>/, '').trim()
    },

    commandsMode(state): boolean {
      return state.text.trimStart().startsWith('>')
    },

    /** An empty field over the history is the palette offering what the user was doing last. */
    recent(): boolean {
      return this.query === '' && (this.kind === null || this.kind === SearchKind.SearchHistory)
    },

    commands(): Command[] {
      const all: Command[] = [
        { id: 'new-request', label: tr('search.commands.newRequest'), icon: 'plus' },
        { id: 'import-collection', label: tr('search.commands.importCollection'), icon: 'upload' },
        { id: 'environments', label: tr('search.commands.environments'), icon: 'globe' },
        { id: 'settings', label: tr('search.commands.settings'), icon: 'settings-2' },
      ]
      const needle = this.query.toLowerCase()
      if (!needle) return all
      return all.filter((command) => command.label.toLowerCase().includes(needle))
    },

    /**
     * Every heading and its rows, in the order the design draws them: the areas Go answered with, and
     * the commands last.
     *
     * The commands are merged in here rather than asked for, and this is the one place the mirror
     * adds anything to what Go said.
     */
    sections(): Section[] {
      const sections: Section[] = []

      if (!this.commandsMode) {
        for (const group of this.groups) {
          if (this.kind !== null && group.kind !== this.kind) continue
          const hits = group.hits ?? []
          if (hits.length === 0) continue
          sections.push({
            kind: group.kind,
            // An empty field answering with times is "Недавнее"; the same rows once something is
            // typed are the history being searched.
            label: this.recent && group.kind === SearchKind.SearchHistory ? tr('search.recent') : kindLabel(group.kind),
            total: group.total,
            rows: hits.map((hit) => ({ hit, command: null })),
          })
        }
      }

      const commands = this.commands
      const wanted = this.kind === null || this.kind === SearchKind.SearchAction
      if (wanted && commands.length > 0) {
        sections.push({
          kind: SearchKind.SearchAction,
          label: kindLabel(SearchKind.SearchAction),
          total: commands.length,
          rows: commands.map((command) => ({ hit: null, command })),
        })
      }

      return sections
    },

    /** Every drawn row in one list, which is what the arrows walk. */
    rows(): PaletteRow[] {
      return this.sections.flatMap((section) => section.rows)
    },

    active(): PaletteRow | null {
      return this.rows[this.activeIndex] ?? null
    },
  },

  actions: {
    // ---- the field -------------------------------------------------------

    openPalette() {
      this.open = true
      this.text = ''
      this.kind = null
      this.activeIndex = 0
      void this.ask()
    },

    close() {
      this.open = false
      if (waiting) clearTimeout(waiting)
      waiting = null
    },

    /**
     * A letter typed. The question waits for a pause, and an answer that arrives after the field has
     * moved on is dropped — the same remedy the command line uses for its text, and for the same
     * reason: a late answer describes words the user has already typed past.
     */
    setText(text: string) {
      this.text = text
      if (waiting) clearTimeout(waiting)
      waiting = setTimeout(() => void this.ask(), QUERY_PAUSE_MS)
    },

    setKind(kind: SearchKind | null) {
      // The chip that is already on is the one that turns the narrowing off again.
      this.kind = this.kind === kind ? null : kind
      void this.ask()
    },

    async ask() {
      waiting = null
      const mine = ++generation
      // A refusal is said out loud like every other one: a palette that silently kept the groups it
      // was showing would be answering a question nobody asked.
      const result = await asked(
        SearchService.Find({ text: this.query, kind: this.kind }),
        'search.failed'
      )
      if (mine !== generation || result === undefined) return
      this.groups = result.groups ?? []
      this.activeIndex = 0
    },

    // ---- the list --------------------------------------------------------

    move(step: number) {
      const count = this.rows.length
      if (count === 0) return
      // The walk wraps: the top is one key away from the bottom, and a palette is not scrolled by
      // hand.
      this.activeIndex = (this.activeIndex + step + count) % count
    },

    /**
     * What a row does when it is chosen. `background` is ⌘↵: the row is reached without the palette
     * closing, so that several things can be picked up in one visit.
     */
    async activate(row: PaletteRow | null, background = false) {
      if (!row) return
      if (!background) this.close()

      if (row.command) {
        this.run(row.command)
        return
      }
      const hit = row.hit
      if (!hit) return

      const requests = useRequestsStore()
      switch (hit.open.target) {
        case SearchTarget.TargetRequest:
        case SearchTarget.TargetCollection: {
          const collections = useCollectionsStore()
          requests.activeView = 'collections'
          reveal(collections.tree, hit.open.id)
          await collections.select(hit.open.id)
          break
        }
        case SearchTarget.TargetEnvironment:
          await useEnvironmentsStore().setActive(hit.open.id)
          break
        case SearchTarget.TargetVariable:
          useEnvironmentsStore().openSheet({ envId: hit.open.scope || null, varName: hit.title })
          break
        case SearchTarget.TargetHistory: {
          const record = requests.records.find((r) => r.id === hit.open.id)
          if (!record) break
          // A capture is read in the Browser view and the app's own call in the Request view: those
          // are the two places a record is drawn, and each loads its own body.
          const captured = record.source === RecordSource.SourceBrowser
          requests.activeView = captured ? 'browser' : 'request'
          if (captured) await requests.selectBrowser(record.id)
          else await requests.selectManual(record.id)
          break
        }
      }
    },

    run(command: Command) {
      switch (command.id) {
        case 'new-request':
          useRequestsStore().focusSearch()
          break
        case 'import-collection':
          void useCollectionsStore().importFile()
          break
        case 'environments':
          useEnvironmentsStore().openSheet()
          break
        case 'settings':
          // An empty category: the window opens where it was, which is what a search result about
          // the settings as a whole means.
          void SystemService.ShowSettings('')
          break
      }
    },
  },
})

// Opens the levels above a row so that a row the palette reached is a row on screen — the same thing
// the tree does for its own walk, and for the same reason: the selection moved, and what is open is
// the user's own answer.
function reveal(tree: Collection[], id: string) {
  const trail = trailOf(tree, id)
  if (!trail) return
  const collections = useCollectionsStore()
  for (const ancestor of trail.ancestors) collections.expanded[ancestor.id] = true
}
