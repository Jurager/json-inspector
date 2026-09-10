// Lightweight JSON:API (https://jsonapi.org/) helpers used by the tree view.

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

export function buildIndex(doc: JsonApiDocument): Map<string, Resource> {
  const idx = new Map<string, Resource>()
  for (const r of dataResources(doc)) idx.set(resourceKey(r.type, r.id), r)
  for (const r of doc.included ?? []) idx.set(resourceKey(r.type, r.id), r)
  return idx
}

export function relIdentifiers(rel: Relationship): ResourceIdentifier[] {
  const d = rel.data
  if (d == null) return []
  if (Array.isArray(d)) return d
  return [d]
}

export function href(link: LinkValue | null | undefined): string {
  if (typeof link === 'string') return link
  if (link && typeof link.href === 'string') return link.href
  return ''
}

// resourceLabel picks a short human-readable label for a resource from its
// attributes, preferring common "name-like" fields.
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
