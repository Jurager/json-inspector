import { SearchKind, SearchNoteKind, type SearchNote } from '../../bindings/json-inspector/internal/domain'
import { t } from '../i18n'

// What the window knows about a kind that Go does not: the glyph on a row and the words of its
// heading. Adding an area is a case in each of the three functions below — and the reason the words
// are here at all is that Go has no catalogue.

/** The glyph in front of a group and on a row that has no method badge of its own. */
export function kindIcon(kind: SearchKind): string {
  switch (kind) {
    case SearchKind.SearchRequest:
      return 'arrow-up-right'
    case SearchKind.SearchCollection:
      return 'folder'
    case SearchKind.SearchEnvironment:
      return 'globe'
    case SearchKind.SearchHistory:
      return 'clock'
    case SearchKind.SearchAction:
      return 'sparkles'
    default:
      return 'search'
  }
}

/** What a group is called. */
export function kindLabel(kind: SearchKind): string {
  switch (kind) {
    case SearchKind.SearchRequest:
      return t('search.kinds.request')
    case SearchKind.SearchCollection:
      return t('search.kinds.collection')
    case SearchKind.SearchEnvironment:
      return t('search.kinds.environment')
    case SearchKind.SearchHistory:
      return t('search.kinds.history')
    case SearchKind.SearchAction:
      return t('search.kinds.action')
    default:
      return ''
  }
}

/**
 * The line on the right of a row. An empty answer is nothing to draw, which is how a note that was
 * never set comes back — the shape is a form, and a form with no kind is a form with nothing in it.
 */
export function noteText(note: SearchNote | null | undefined): string {
  switch (note?.kind) {
    case SearchNoteKind.NoteActive:
      return t('search.notes.active')
    case SearchNoteKind.NoteRequests:
      return t('counts.requests', note.count ?? 0)
    default:
      return ''
  }
}
