import { ref } from 'vue'
import { DraftService } from '../../bindings/json-inspector/internal/transport/wails'
import type { AuthType, Scheme } from '../../bindings/json-inspector/internal/domain'

// Every way a request can authorize itself, asked for once. Go is the side that holds this list:
// what a scheme asks for, how each field is drawn and which of them are secrets all come from there,
// so a scheme added on that side appears here without the window learning anything new.
const schemes = ref<Scheme[]>([])

DraftService.AuthSchemes()
  .then((list) => {
    schemes.value = list ?? []
  })
  .catch(() => {})

export function useAuthSchemes() {
  return {
    // A scheme that needs a level above it is offered only where there is one: a command line has
    // nothing to inherit from, and a choice that means nothing there is a choice nobody should make.
    offered: (canInherit: boolean) => schemes.value.filter((s) => canInherit || !s.needsParent),
    schemeOf: (type: AuthType) => schemes.value.find((s) => s.type === type) ?? null,
  }
}
