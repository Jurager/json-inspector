import type { Collection, CollectionNode } from '../../bindings/json-inspector/internal/domain'

// The tree as the window walks it: a collection's level, finding a row in it, naming the path to one,
// and narrowing the whole thing by a search. All of it is pure — the order and the nesting come from
// Go, and this only reads them.

/**
 * One row of a level: a request, or a collection inside it. Exactly one side is set, and the order of
 * the two is the order the tree draws them in.
 */
export type Child =
  | { kind: 'request'; node: CollectionNode }
  | { kind: 'collection'; collection: Collection }

/**
 * The rows inside a collection, in the order the tree draws them.
 *
 * A collection's own requests and the collections inside it are one number line in Go — the position a
 * nested collection was dropped at is a position among the requests — so the two lists the model keeps
 * apart merge back into one by that number. Both arrive in position order, which is what makes a single
 * pass over the pair enough.
 */
export function childrenOf(collection: Collection): Child[] {
  const requests = collection.items ?? []
  const nested = collection.children ?? []

  const out: Child[] = []
  let request = 0
  let child = 0
  while (request < requests.length || child < nested.length) {
    if (child >= nested.length || (request < requests.length && requests[request].position <= nested[child].position)) {
      out.push({ kind: 'request', node: requests[request] })
      request += 1
      continue
    }
    out.push({ kind: 'collection', collection: nested[child] })
    child += 1
  }
  return out
}

// Where a row sits: the collections above it, outermost first, and the row itself. Exactly one of
// `collection` and `node` is set, because a row is one or the other.
export interface Trail {
  ancestors: Collection[]
  collection: Collection | null
  node: CollectionNode | null
}

/** The collection the id sits in: the one it is, or the one that holds it. */
export function holderOf(trail: Trail | null): Collection | null {
  if (!trail) return null
  return trail.collection ?? trail.ancestors[trail.ancestors.length - 1] ?? null
}

export function findNode(tree: Collection[], id: string | null): CollectionNode | null {
  if (!id) return null
  for (const collection of tree) {
    const inLevel = (collection.items ?? []).find((node) => node.id === id)
    if (inLevel) return inLevel
    const below = findNode(collection.children ?? [], id)
    if (below) return below
  }
  return null
}

export function findCollection(tree: Collection[], id: string | null): Collection | null {
  if (!id) return null
  for (const collection of tree) {
    if (collection.id === id) return collection
    const below = findCollection(collection.children ?? [], id)
    if (below) return below
  }
  return null
}

// The path to a row, whatever it names: a collection, or a request inside one. The ancestors are the
// collections above it — a request never holds anything, so nothing else can be on the way down.
export function trailOf(tree: Collection[], id: string | null): Trail | null {
  if (!id) return null

  const walk = (collections: Collection[], ancestors: Collection[]): Trail | null => {
    for (const collection of collections) {
      if (collection.id === id) return { ancestors, collection, node: null }

      const node = (collection.items ?? []).find((row) => row.id === id)
      if (node) return { ancestors: [...ancestors, collection], collection: null, node }

      const found = walk(collection.children ?? [], [...ancestors, collection])
      if (found) return found
    }
    return null
  }
  return walk(tree, [])
}

// How many requests a collection holds, the collections inside it included: the number on a row is
// what running it would send.
export function requestCount(collection: Collection): number {
  return childrenOf(collection).reduce(
    (total, child) => total + (child.kind === 'request' ? 1 : requestCount(child.collection)),
    0
  )
}

// What a filtered tree keeps: a row whose name matches, and every row above it, so a match deep in the
// tree is reachable rather than hidden inside a collapsed parent.
export function filterTree(tree: Collection[], query: string): Collection[] {
  const needle = query.trim().toLowerCase()
  if (!needle) return tree

  // A collection survives on its own name or on anything kept under it — a level with nothing left in
  // it is a level the search has nothing to say about.
  const kept = (collection: Collection): boolean =>
    collection.name.toLowerCase().includes(needle) ||
    (collection.items ?? []).length > 0 ||
    (collection.children ?? []).length > 0

  const keep = (collection: Collection): Collection => {
    const items = (collection.items ?? []).filter((node) => node.name.toLowerCase().includes(needle))
    const children = (collection.children ?? []).map(keep).filter(kept)
    return { ...collection, items, children }
  }

  return tree.map(keep).filter(kept)
}
