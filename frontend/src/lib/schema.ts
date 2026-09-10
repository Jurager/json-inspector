import type { JsonApiDocument, Resource } from './jsonapi'
import { dataResources, href, relIdentifiers } from './jsonapi'

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

export function humanize(type: string): string {
  return type.charAt(0).toUpperCase() + type.slice(1)
}

// buildSchema aggregates all resources of a document into a type-level schema:
// per type, the union of attribute keys, relationships and incoming references.
export function buildSchema(doc: JsonApiDocument): TypeInfo[] {
  const all: Resource[] = [...dataResources(doc), ...(doc.included ?? [])]
  const presentTypes = new Set(all.map((r) => r.type))

  const map = new Map<string, TypeInfo>()
  for (const r of all) {
    let t = map.get(r.type)
    if (!t) {
      t = { type: r.type, label: humanize(r.type), count: 0, attributes: [], rels: [], incoming: [] }
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
      const relatedUrl = href(rel.links?.related)
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

export interface TypeDiff {
  type: string
  label: string
  status: 'added' | 'removed' | 'changed'
  addedAttrs: string[]
  removedAttrs: string[]
  addedRels: string[]
  removedRels: string[]
}

function relLabel(r: RelInfo): string {
  return `${r.name} → ${humanize(r.targetType)}`
}

// diffSchemas compares two schemas. `base` is the current response, `target`
// the one being compared against: "added" means present in target but not base,
// "removed" means present in base but not target.
export function diffSchemas(base: TypeInfo[], target: TypeInfo[]): TypeDiff[] {
  const bm = new Map(base.map((t) => [t.type, t]))
  const tm = new Map(target.map((t) => [t.type, t]))
  const keys = new Set([...bm.keys(), ...tm.keys()])

  const diffs: TypeDiff[] = []
  for (const key of keys) {
    const b = bm.get(key)
    const t = tm.get(key)
    if (!b && t) {
      diffs.push({
        type: key,
        label: t.label,
        status: 'added',
        addedAttrs: t.attributes,
        removedAttrs: [],
        addedRels: t.rels.map(relLabel),
        removedRels: [],
      })
    } else if (b && !t) {
      diffs.push({
        type: key,
        label: b.label,
        status: 'removed',
        addedAttrs: [],
        removedAttrs: b.attributes,
        addedRels: [],
        removedRels: b.rels.map(relLabel),
      })
    } else if (b && t) {
      const bA = new Set(b.attributes)
      const tA = new Set(t.attributes)
      const bR = new Set(b.rels.map(relLabel))
      const tR = new Set(t.rels.map(relLabel))
      const addedAttrs = t.attributes.filter((x) => !bA.has(x))
      const removedAttrs = b.attributes.filter((x) => !tA.has(x))
      const addedRels = t.rels.map(relLabel).filter((x) => !bR.has(x))
      const removedRels = b.rels.map(relLabel).filter((x) => !tR.has(x))
      if (addedAttrs.length || removedAttrs.length || addedRels.length || removedRels.length) {
        diffs.push({ type: key, label: t.label, status: 'changed', addedAttrs, removedAttrs, addedRels, removedRels })
      }
    }
  }
  return diffs.sort((a, b) => a.label.localeCompare(b.label))
}
