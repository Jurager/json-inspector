import { createI18n, useI18n } from 'vue-i18n'
import en, { type Messages } from './locales/en'
import ru from './locales/ru'

// Typed message keys. The augmentation has to live in a module that imports vue-i18n itself: declared
// from the catalogue it is read as a plain interface and never reaches the composer's `t`, which then
// accepts any string and renders a typo as its own key.
declare module 'vue-i18n' {
  export interface DefineLocaleMessage extends Messages {}
}

// The languages the app has catalogues for. Which of them the settings screen offers, and in what
// order, is Go's answer — a list here would be a second one, and the two would drift.
export type Locale = 'ru' | 'en'

// English is both the source of truth for the message keys and the fallback: a key missing from the
// chosen language has to come out as words rather than as its own name.
export const FALLBACK_LOCALE: Locale = 'en'

// The slots a message's plural forms are picked from, named the way Intl.PluralRules names them.
// Russian has no `zero` or `two` category; a number that somehow takes one lands on the slot for
// everything else rather than on one that does not exist.
const PLURAL_SLOTS: Partial<Record<Intl.LDMLPluralRule, number>> = { one: 0, few: 1, many: 2, other: 2 }

// vue-i18n's own rule is written for English and does not stretch to Russian: with three forms it
// collapses `few` and `many` onto the same slot (`Math.min(choice, 2)`), so '2 запроса' and '5
// запросов' would come out of one form. The platform is asked which form a number takes instead, and
// the answer is mapped onto the slots the message actually has — which is what lets one key serve
// both languages: '{n} request | {n} requests' against '{n} запрос | {n} запроса | {n} запросов'.
//
// This is the composition API's option. The legacy API's `pluralizationRules` is a different name for
// a different table and is ignored here — a rule registered under it is never called, and the window
// silently falls back to the English one.
const russianPlurals = new Intl.PluralRules('ru')

function russianPluralRule(choice: number, choicesLength: number): number {
  const slot = PLURAL_SLOTS[russianPlurals.select(choice)] ?? 2
  // A message may hold fewer forms than Russian has categories — one that counts two things needs
  // two — and the last form is what the rest fall back to.
  return Math.min(slot, choicesLength - 1)
}

// What "system" means, asked of the webview: the only side of the app that can ask, and the reason the
// setting travels to a window unresolved. An unreadable value falls back like an absent one — the
// webview knows better than a language from a build that no longer exists.
export function systemLocale(): Locale {
  return navigator.language.toLowerCase().startsWith('ru') ? 'ru' : FALLBACK_LOCALE
}

/** A stored or URL-supplied choice, resolved. */
function resolveLocale(choice: string | null | undefined): Locale {
  return choice === 'ru' || choice === 'en' ? choice : systemLocale()
}

// The choice Go put on the window's URL. Read here rather than over IPC for the same reason the theme
// is: this runs before the first frame, and both catalogues are in the bundle either way.
export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: resolveLocale(new URLSearchParams(location.search).get('lang')),
  fallbackLocale: FALLBACK_LOCALE,
  messages: { en, ru },
  pluralRules: { ru: russianPluralRule },
})

/**
 * The message functions a component's script needs. The scope is global on purpose: there is one
 * catalogue per language for the whole app and one language for the whole process, so a component
 * scope would only be a second place for the same answer to live.
 */
export function useMessages() {
  return useI18n({ useScope: 'global' })
}

// The message function for callers outside a component — a formatter, a helper. The composer's own
// methods are bound where they are made, so one taken off the object stays whole.
export const { t } = i18n.global

// `Intl` formatters are the expensive part of formatting and the same one is asked for on every row of
// a table, so one per locale per shape is all the app can use.
const numberFormats = new Map<string, Intl.NumberFormat>()
const dateFormats = new Map<string, Intl.DateTimeFormat>()

function numberFormat(digits: number): Intl.NumberFormat {
  const key = `${i18n.global.locale.value}:${digits}`
  let format = numberFormats.get(key)
  if (!format) {
    format = new Intl.NumberFormat(i18n.global.locale.value, {
      minimumFractionDigits: digits,
      maximumFractionDigits: digits,
    })
    numberFormats.set(key, format)
  }
  return format
}

function dateFormat(options: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
  const key = `${i18n.global.locale.value}:${JSON.stringify(options)}`
  let format = dateFormats.get(key)
  if (!format) {
    format = new Intl.DateTimeFormat(i18n.global.locale.value, options)
    dateFormats.set(key, format)
  }
  return format
}

/**
 * A number written the way the language writes numbers. The decimal separator and the thousands
 * grouping are the one thing a catalogue cannot hold. Reading the locale here is also what makes a
 * caller inside a `computed` redraw when the language changes.
 */
export function formatNumber(value: number, digits = 0): string {
  return numberFormat(digits).format(value)
}

export function formatDate(value: number | Date, options: Intl.DateTimeFormatOptions): string {
  return dateFormat(options).format(value)
}

/** The codes the catalogue has a sentence for — the shape Go names a refusal with. */
type FailureCode = keyof Messages['errors']['codes']

const CODES: Record<string, string> = en.errors.codes

/** A code this build knows: the catalogue is the list, so a newer build's code is not one. */
function isFailureCode(value: unknown): value is FailureCode {
  return typeof value === 'string' && CODES[value] !== undefined
}

/** A failure as it crosses: the code Go named and the values its sentence needs. The values are
 * optional as well as nullable — a Go map's keys reach the window as an index signature. */
type Refusal = { code?: unknown; args?: Record<string, string | undefined> | null }

/**
 * The window's words for a refusal, or nothing when the code is not one this build words — a newer
 * build's code, or a failure nobody wrote a sentence for.
 */
export function refusalText(refusal: Refusal | null | undefined): string | null {
  if (!isFailureCode(refusal?.code)) return null
  return t(`errors.codes.${refusal.code}`, countable(refusal.args))
}

/**
 * A count crosses as the string every value of a Go map does, and a plural form is chosen by a
 * number: a sentence about one variable and many of them would otherwise always take its first form,
 * whatever the count was. Only `n` is read this way, because `n` is what the rule is asked about.
 */
function countable(args: Record<string, string | undefined> | null | undefined) {
  const count = args?.n
  if (count === undefined || count === '' || Number.isNaN(Number(count))) return args ?? {}
  return { ...args, n: Number(count) }
}

/**
 * What went wrong, in the window's own words.
 *
 * A call Go refused carries the refusal's code and the values its sentence needs, and the sentence is
 * read from the catalogue — which is what makes a failure the app is responsible for read in the
 * language the window is in. A failure with no code belongs to the machine: the network, the disk, a
 * socket. Its message is shown as it is, because nobody has written a sentence for it and inventing
 * one would say less than the machine does.
 */
export function describeFailure(error: unknown): string {
  const cause = (error as { cause?: Refusal } | null)?.cause
  return refusalText(cause) ?? String(error)
}

// Every time in the app is microseconds, because a millisecond is too coarse to say anything about a
// warm connection: its phases are over before the second millisecond ticks, and rounding them to zero
// made a measured request look like one that failed to be measured. A phase under a millisecond gets a
// decimal for the same reason — written with the separator the language writes decimals with. The unit
// is a word from the catalogue: `Intl` spells it out ('1.2 sec') or abbreviates it the way a byte count
// would ('8.4 MB' for 8 digits of megabytes) and neither is what the design writes.
export function formatMicros(us: number): string {
  if (us < 1000) return `${formatNumber(us / 1000, 1)} ${t('units.ms')}`
  if (us < 1_000_000) return `${formatNumber(Math.round(us / 1000), 0)} ${t('units.ms')}`
  return `${formatNumber(us / 1_000_000, 1)} ${t('units.s')}`
}

const BYTE_UNITS = ['units.b', 'units.kb', 'units.mb', 'units.gb'] as const

export function formatBytes(bytes: number): string {
  if (!bytes) return `0 ${t('units.b')}`
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < BYTE_UNITS.length - 1) {
    value /= 1024
    unit++
  }
  // Below a kilobyte and above a hundred of anything the fraction says nothing, so it is dropped —
  // the same rule the readout has always used.
  const digits = value >= 100 || unit === 0 ? 0 : 1
  return `${formatNumber(value, digits)} ${t(BYTE_UNITS[unit])}`
}

const sameDay = (a: Date, b: Date) => a.toDateString() === b.toDateString()

// How long ago something happened, for the line next to a run: minutes while it is fresh, then the
// step that reads better, and the date once "{n} days ago" has stopped saying anything useful.
export function formatAgo(ms: number): string {
  if (!ms) return t('common.none')
  const minutes = Math.floor((Date.now() - ms) / 60_000)
  if (minutes < 1) return t('time.justNow')
  if (minutes < 60) return t('time.minutesAgo', minutes)
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return t('time.hoursAgo', hours)
  const days = Math.floor(hours / 24)
  if (days < 7) return t('time.daysAgo', days)
  return formatDate(ms, { day: 'numeric', month: 'long' })
}

export function formatCheckedAt(ms: number): string {
  if (!ms) return t('common.none')
  const at = new Date(ms)
  const time = formatDate(at, { hour: '2-digit', minute: '2-digit' })
  const now = new Date()
  if (sameDay(at, now)) return t('time.todayAt', { time })
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (sameDay(at, yesterday)) return t('time.yesterdayAt', { time })
  return `${formatDate(at, { day: 'numeric', month: 'long' })}, ${time}`
}
