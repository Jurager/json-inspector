import type { Collection } from '../../bindings/json-inspector/internal/domain'

// One level of the tree, flat: a place a request can be saved into. A folder and a collection are the
// same thing at different depths, and both are places, which is why they come out of one walk.
export interface Place {
  id: string
  // The names from the top down to this level, outermost first. A folder called «Articles» says
  // nothing about which collection holds it, and the sheet's list is flat.
  path: string[]
  // What the level holds right now. The meta line under a name says how big the place is, and this is
  // the only number it says.
  requests: number
}

// Every level of the tree, deepest ones included, in the order the tree draws them.
export function collectionPlaces(tree: Collection[]): Place[] {
  const out: Place[] = []
  const walk = (collections: Collection[], path: string[]) => {
    for (const collection of collections) {
      const trail = [...path, collection.name]
      out.push({ id: collection.id, path: trail, requests: (collection.items ?? []).length })
      walk(collection.children ?? [], trail)
    }
  }
  walk(tree, [])
  return out
}
