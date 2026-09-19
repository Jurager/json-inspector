import type {
  BodyRef,
  CookieRow,
  HeaderPair,
  Record,
  ResponseCookie,
} from '../../bindings/json-inspector/internal/domain'

// A record as the window draws it: the one Go keeps, with its two bodies flattened to text. Storing
// a record is the record itself; drawing it is this.
export type RecordView = Omit<
  Record,
  | 'requestBody'
  | 'responseBody'
  | 'requestHeaders'
  | 'responseHeaders'
  | 'requestCookies'
  | 'responseCookies'
> & {
  // The wire marks a list as possibly null because Go can marshal a nil slice that way; the app
  // never means that, so the shift from Go to view settles it once here instead of at every use.
  requestHeaders: HeaderPair[]
  responseHeaders: HeaderPair[]
  requestCookies: CookieRow[]
  responseCookies: ResponseCookie[]
  requestBody: string
  responseBody: string
}

// Both sides of a record once they have been read, by side. What a fetch for a body answers with.
export interface RecordBodies {
  request?: string
  response?: string
}

export function recordView(record: Record, bodies?: RecordBodies): RecordView {
  return {
    ...record,
    requestHeaders: record.requestHeaders ?? [],
    responseHeaders: record.responseHeaders ?? [],
    requestCookies: record.requestCookies ?? [],
    responseCookies: record.responseCookies ?? [],
    requestBody: bodies?.request ?? record.requestBody?.inline ?? '',
    responseBody: bodies?.response ?? record.responseBody?.inline ?? '',
  }
}

// Whether a body still has to be asked for. `inline` is present exactly when it is the whole text,
// so its absence next to a non-zero size is the one case where a call is owed — and a body that is
// merely empty is not one of them.
export function bodyMissing(ref: BodyRef | null | undefined): boolean {
  return !!ref && ref.size > 0 && !ref.inline
}
