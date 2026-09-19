import { t } from '../i18n'
import type { Record } from '../../bindings/json-inspector/internal/domain'

// One tab of captured traffic: what the browser list draws, and what the tab's own page is about.
// The records arrive newest-first and stay that way, so the first of a group is the last thing the
// page asked for.
export interface TabGroup {
  key: string
  title: string
  url: string
  favIconUrl: string
  items: Record[]
}

// What a tab is called in a key. The extension's tab id when there is one, and the page's own address
// when there is not: a record forwarded without a tab is still the record of a page, and grouping
// those together is the only thing that can be said about them.
//
// The id counts only when it is a real one: `tabId` is `omitempty` on the wire, so the first tab of a
// browser — the id zero — arrives as absent, and a group keyed on it would be a group that changes
// its mind about which tab it is.
function tabKeyOf(record: Record): string {
  return record.tabId ? String(record.tabId) : record.tabURL || 'unknown'
}

// The captured records, grouped by the tab they came from, newest tab first.
export function tabGroups(records: Record[]): TabGroup[] {
  const map = new Map<string, TabGroup>()
  for (const record of records) {
    const key = tabKeyOf(record)
    let group = map.get(key)
    if (!group) {
      group = {
        key,
        title: record.tabTitle || '',
        url: record.tabURL || '',
        favIconUrl: record.favIconUrl || '',
        items: [],
      }
      map.set(key, group)
    }
    group.items.push(record)
    // A tab is named by the first record that knew: the title and the icon are the page's, and a
    // request forwarded before the page had one carries nothing.
    if (!group.title && record.tabTitle) group.title = record.tabTitle
    if (!group.favIconUrl && record.favIconUrl) group.favIconUrl = record.favIconUrl
    if (!group.url && record.tabURL) group.url = record.tabURL
  }
  return Array.from(map.values()).sort((a, b) => b.items[0].startedAt - a.items[0].startedAt)
}

// What a tab is called where there is no room for its address: the page's title, then the host, then
// the bare word — a row has to say something.
export function groupLabel(group: TabGroup): string {
  return group.title || hostnameOf(group.url) || t('history.tab')
}

function hostnameOf(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return ''
  }
}

// The hue of the letter a tab is drawn with when it has no icon. Derived from the key so that the
// same tab is the same colour on every drawing of it.
export function groupHue(key: string): number {
  let hue = 0
  for (let i = 0; i < key.length; i++) hue = (hue * 31 + key.charCodeAt(i)) >>> 0
  return hue % 360
}

// Which tab the window is looking at. The browser section is a list of tabs, so one of them is
// always the one on screen: the chosen one, and the newest when nobody has chosen — a page that drew
// the onboarding beside a list of captured traffic would be a page about nothing.
export function effectiveTab(groups: TabGroup[], key: string | null): TabGroup | null {
  if (!groups.length) return null
  return groups.find((group) => group.key === key) ?? groups[0]
}

// What the tab's page says about the traffic it holds, counted once so the four numbers on it and
// the table under them cannot disagree. Failures are the answers the browser got and did not want —
// a status of four hundred and up — and a record that never had an answer is one too.
export interface TabStats {
  requests: number
  failed: number
  transferred: number
  medianUs: number
  fastestUs: number
  firstFailure: Record | null
}

export function tabStats(records: Record[]): TabStats {
  const times = records.map((r) => r.durationUs ?? 0).filter((us) => us > 0)
  times.sort((a, b) => a - b)

  return {
    requests: records.length,
    failed: records.filter(failedRecord).length,
    transferred: records.reduce((sum, r) => sum + (r.responseBytes ?? 0), 0),
    medianUs: median(times),
    fastestUs: times.length ? times[0] : 0,
    firstFailure: records.find(failedRecord) ?? null,
  }
}

// A failure is the answer being wrong, not the request being unwise: an error the extension reported,
// or a status the server has no reason to be proud of.
function failedRecord(record: Record): boolean {
  return Boolean(record.error) || (record.status ?? 0) >= 400
}

// The middle of a list of times. An even count has two middles and takes their average, which is what
// a median is — picking one of them would make the number jump between two requests.
function median(sorted: number[]): number {
  if (!sorted.length) return 0
  const middle = Math.floor(sorted.length / 2)
  if (sorted.length % 2) return sorted[middle]
  return Math.round((sorted[middle - 1] + sorted[middle]) / 2)
}
