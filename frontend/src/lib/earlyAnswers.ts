import type { Record } from '../../bindings/json-inspector/internal/domain'

// What one attempt can become before the pane that started it knows its id: a record, or a failure
// that happened before there was anything to keep.
export type EarlyAnswer = { record: Record } | { failed: true }

// An answer can outrun the news of the id it answers. Go names an attempt and starts it in the same
// breath, and one that fails on the spot — an address with no host in it, a script that throws — is
// over before that id has crossed the bridge. The pane that asked would then be waiting for an
// answer it has already been handed, which is the one way this window can spin forever.
//
// So an answer no pane recognizes yet waits here, under its own id, and the claim that comes a
// moment later picks it up. Keyed by id, because ids are unique: whatever order the two messages
// arrive in, nothing can be handed to the wrong pane.
//
// Bounded, because an answer nobody will ever claim — a run's own request, say — is not worth
// keeping.
const LIMIT = 8

const held = new Map<string, EarlyAnswer>()

export function holdAnswer(id: string, answer: EarlyAnswer) {
  // The same id twice is the same answer: it is moved to the end, where it is the last to be dropped.
  held.delete(id)
  held.set(id, answer)
  while (held.size > LIMIT) {
    const oldest = held.keys().next().value
    if (oldest === undefined) break
    held.delete(oldest)
  }
}

export function takeAnswer(id: string): EarlyAnswer | undefined {
  const answer = held.get(id)
  held.delete(id)
  return answer
}
