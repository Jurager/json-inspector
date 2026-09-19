export interface ResourceIdentifier {
  type: string
  id: string
  meta?: unknown
}

export type LinkValue = string | { href?: string; meta?: unknown }

export interface Links {
  [name: string]: LinkValue | null | undefined
}

export interface Relationship {
  data?: ResourceIdentifier | ResourceIdentifier[] | null
  links?: Links
  meta?: unknown
}

export interface Resource {
  type: string
  id: string
  attributes?: Record<string, unknown>
  relationships?: Record<string, Relationship>
  links?: Links
  meta?: unknown
}

export interface JsonApiDocument {
  jsonapi?: unknown
  meta?: unknown
  links?: Links
  data?: Resource | Resource[] | null
  included?: Resource[]
  errors?: unknown[]
}

export function resourceKey(type: string, id: string): string {
  return `${type}/${id}`
}

export function dataResources(doc: JsonApiDocument): Resource[] {
  const d = doc.data
  if (d == null) return []
  if (Array.isArray(d)) return d
  return [d]
}

// Keyed by `type/id` — the same key `resourceKey` builds, so a relationship can look up its
// target without scanning the document.
export function buildResourceIndex(doc: JsonApiDocument): Map<string, Resource> {
  const index = new Map<string, Resource>()
  for (const r of dataResources(doc)) index.set(resourceKey(r.type, r.id), r)
  for (const r of doc.included ?? []) index.set(resourceKey(r.type, r.id), r)
  return index
}

export function relIdentifiers(rel: Relationship): ResourceIdentifier[] {
  const d = rel.data
  if (d == null) return []
  if (Array.isArray(d)) return d
  return [d]
}

// A link is either the URL itself or an object carrying one; a missing link is an empty
// string, so callers can test the result rather than the shape.
export function linkHref(link: LinkValue | null | undefined): string {
  if (typeof link === 'string') return link
  if (link && typeof link.href === 'string') return link.href
  return ''
}

export function resourceLabel(r: Resource): string {
  const attrs = r.attributes
  if (!attrs) return `${r.type}/${r.id}`
  for (const key of ['title', 'name', 'label', 'username', 'email', 'slug', 'code', 'key']) {
    const v = attrs[key]
    if (typeof v === 'string' && v) return v
  }
  for (const v of Object.values(attrs)) {
    if (typeof v === 'string' && v) return v
  }
  return `${r.type}/${r.id}`
}

// Expects an already-trimmed, lowercased query. Shared by the "Тело" search so
// the toolbar count and the tree's filtering agree with each other.
export function resourceMatchesQuery(r: Resource, q: string): boolean {
  if (resourceLabel(r).toLowerCase().includes(q)) return true
  if (r.type.toLowerCase().includes(q) || String(r.id).toLowerCase().includes(q)) return true
  for (const [k, v] of Object.entries(r.attributes ?? {})) {
    if (k.toLowerCase().includes(q)) return true
    const vs = v === null ? 'null' : typeof v === 'object' ? JSON.stringify(v) : String(v)
    if (vs.toLowerCase().includes(q)) return true
  }
  return false
}

/**
 * The version a document declares about itself, and nothing when it declares none this window has a
 * word for. The member is read as unknown because a response is whatever the server sent: a version
 * that is not a string is not a version to print.
 *
 * Three places draw it — the status bar, the response's own tag, the inspector — and each had read
 * it for itself, which is three answers to one question.
 */
export function documentVersion(doc: JsonApiDocument | null): string {
  const declared = (doc?.jsonapi ?? null) as { version?: unknown } | null
  return typeof declared?.version === 'string' ? declared.version : ''
}

/**
 * How many relationships point at a resource the document does not carry. It is the one defect a
 * JSON:API reader can see on its own: the link is there, and the thing it names is not, so a tree
 * that followed it would have nothing to open.
 */
export function missingLinkCount(doc: JsonApiDocument): number {
  const index = buildResourceIndex(doc)
  let missing = 0
  for (const resource of [...dataResources(doc), ...(doc.included ?? [])]) {
    for (const rel of Object.values(resource.relationships ?? {})) {
      for (const id of relIdentifiers(rel)) {
        if (!index.has(resourceKey(id.type, id.id))) missing += 1
      }
    }
  }
  return missing
}

export function isJsonApi(obj: unknown): boolean {
  if (typeof obj !== 'object' || obj === null || Array.isArray(obj)) return false
  const d = obj as Record<string, unknown>
  if ('jsonapi' in d) return true
  if ('included' in d) return true
  const data = d.data
  if (data == null) return false
  const arr = Array.isArray(data) ? data : [data]
  if (arr.length === 0) return false
  return arr.every((r) => isResourceLike(r))
}

function isResourceLike(r: unknown): boolean {
  return (
    typeof r === 'object' &&
    r !== null &&
    typeof (r as Record<string, unknown>).type === 'string' &&
    typeof (r as Record<string, unknown>).id === 'string'
  )
}
