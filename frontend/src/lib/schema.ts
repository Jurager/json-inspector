import type { JsonApiDocument, Resource } from './jsonapi'
import { dataResources, linkHref, relIdentifiers } from './jsonapi'

export interface RelInfo {
  name: string
  targetType: string
  many: boolean
  inDoc: boolean
  relatedUrl: string
}

export interface IncomingInfo {
  fromType: string
  rel: string
}

export interface TypeInfo {
  type: string
  label: string
  count: number
  attributes: string[]
  rels: RelInfo[]
  incoming: IncomingInfo[]
}

// A JSON:API type is lowercase by convention ("articles"); the views label it capitalised.
export function capitalizeType(type: string): string {
  return type.charAt(0).toUpperCase() + type.slice(1)
}

export function buildTypeInfos(doc: JsonApiDocument): TypeInfo[] {
  const all: Resource[] = [...dataResources(doc), ...(doc.included ?? [])]
  const presentTypes = new Set(all.map((r) => r.type))

  const map = new Map<string, TypeInfo>()
  for (const r of all) {
    let t = map.get(r.type)
    if (!t) {
      t = { type: r.type, label: capitalizeType(r.type), count: 0, attributes: [], rels: [], incoming: [] }
      map.set(r.type, t)
    }
    t.count++
    if (r.attributes) {
      for (const k of Object.keys(r.attributes)) {
        if (!t.attributes.includes(k)) t.attributes.push(k)
      }
    }
  }

  for (const r of all) {
    const t = map.get(r.type)
    if (!t || !r.relationships) continue
    for (const [name, rel] of Object.entries(r.relationships)) {
      const many = Array.isArray(rel.data)
      const relatedUrl = linkHref(rel.links?.related)
      for (const ri of relIdentifiers(rel)) {
        const existing = t.rels.find((x) => x.name === name && x.targetType === ri.type)
        if (existing) {
          existing.many = existing.many || many
          if (!existing.relatedUrl && relatedUrl) existing.relatedUrl = relatedUrl
        } else {
          t.rels.push({ name, targetType: ri.type, many, inDoc: presentTypes.has(ri.type), relatedUrl })
        }
        const target = map.get(ri.type)
        if (target && !target.incoming.some((x) => x.fromType === r.type && x.rel === name)) {
          target.incoming.push({ fromType: r.type, rel: name })
        }
      }
    }
  }

  return Array.from(map.values()).sort((a, b) => a.label.localeCompare(b.label))
}
