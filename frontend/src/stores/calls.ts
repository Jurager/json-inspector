import { describeFailure, t } from '../i18n'
import { useToast } from '../composables/useToast'

const toast = useToast()

// The window's own voice for a call that came back as a refusal.
//
// Nearly everything the stores do is a call to Go, and Go refuses for ordinary reasons: the database
// said no, the server a request was for is away, a variable is missing. An unhandled rejection is not
// an answer to any of them — the store simply keeps drawing the state it had, and the person who
// asked for the change is told nothing at all. So every call that the window would otherwise be
// silent about goes through here: the refusal is said out loud, and nothing is landed.
//
// A read and an edit are the same thing to this function — both answer with something the store
// draws — and `undefined` is how a caller learns there is nothing to land. The wording is the
// caller's, because only the caller knows what it was trying to do.
export async function asked<T>(call: Promise<T>, key: string): Promise<T | undefined> {
  try {
    return await call
  } catch (error) {
    toast.show(t(key, { error: describeFailure(error) }), 'error')
    return undefined
  }
}
