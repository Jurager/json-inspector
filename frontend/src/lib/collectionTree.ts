import type { Collection, CollectionNode } from '../../bindings/json-inspector/internal/domain'

// The tree as the window walks it: finding a node, naming the path to it, and narrowing it by a
// search. All of it is pure — the order and the nesting come from Go, and this only reads them.

// Where a node sits: the collection it belongs to and the rows above it, outermost first. The card
// draws this as breadcrumbs, so the last entry is the node itself.
export interface Trail {
  collection: Collection
  ancestors: CollectionNode[]
  node: CollectionNode | null
}

export function findNode(tree: Collection[], id: string | null): CollectionNode | null {
  if (!id) return null
  const walk = (nodes: CollectionNode[]): CollectionNode | null => {
    for (const node of nodes) {
      if (node.id === id) return node
      const found = walk(node.items ?? [])
      if (found) return found
    }
    return null
  }
  for (const collection of tree) {
    const found = walk(collection.items ?? [])
    if (found) return found
  }
  return null
}

export function findCollection(tree: Collection[], id: string | null): Collection | null {
  if (!id) return null
  return tree.find((c) => c.id === id) ?? null
}

// The path to a node, whatever it names: a collection on its own, a folder, or a request.
export function trailOf(tree: Collection[], id: string | null): Trail | null {
  if (!id) return null

  for (const collection of tree) {
    if (collection.id === id) return { collection, ancestors: [], node: null }

    const walk = (nodes: CollectionNode[], ancestors: CollectionNode[]): Trail | null => {
      for (const node of nodes) {
        if (node.id === id) return { collection, ancestors, node }
        const found = walk(node.items ?? [], [...ancestors, node])
        if (found) return found
      }
      return null
    }
    const found = walk(collection.items ?? [], [])
    if (found) return found
  }
  return null
}

// How many requests a subtree holds. A folder counts its own; the collection counts everything —
// the number on the row is what running it would send.
export function requestCount(node: { items?: CollectionNode[] | null }): number {
  return (node.items ?? []).reduce(
    (total, child) => total + (child.kind === 'request' ? 1 : requestCount(child)),
    0
  )
}

// What a filtered tree keeps: a row whose name matches, and every row above it, so a match deep in
// the tree is reachable rather than hidden inside a collapsed parent.
export function filterTree(tree: Collection[], query: string): Collection[] {
  const needle = query.trim().toLowerCase()
  if (!needle) return tree

  const keep = (nodes: CollectionNode[]): CollectionNode[] => {
    const out: CollectionNode[] = []
    for (const node of nodes) {
      const children = keep(node.items ?? [])
      if (node.name.toLowerCase().includes(needle) || children.length > 0) {
        out.push({ ...node, items: children })
      }
    }
    return out
  }

  return tree
    .map((collection) => ({ ...collection, items: keep(collection.items ?? []) }))
    .filter((collection) => collection.name.toLowerCase().includes(needle) || (collection.items ?? []).length > 0)
}
