import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { AccountService } from '../../bindings/json-inspector/internal/transport/wails'
import type { State } from '../../bindings/json-inspector/internal/usecase/account'
import type { Session } from '../../bindings/json-inspector/internal/domain'
import { describeFailure, refusalText } from '../i18n'

// The account, as the window sees it: whether anybody is signed in, what is happening right now, and
// the devices the server knows about. Go holds the truth — this is what is drawn, and it is drawn from
// what Go last said.
//
// Every window is told when the account moves, and the window that asked for the change is told as
// well: a sign-in finishes long after the call that started it returned, and the modal that shows a
// spinner is not the one that decides when to stop.
const state = ref<State | null>(null)
const sessions = ref<Session[]>([])
// The refusal in words, ready to be shown. Go sends a code and the values its sentence needs; the
// catalogue is where the sentence lives, so the wording happens here.
const failure = ref('')
const sessionsFailed = ref(false)
const busy = ref(false)
// Which of the account's two dialogs is up in this window. They are the account's own state rather
// than the rail's: the account menu opens them, the window root draws them — an overlay has to hang
// off the window and not off the rail, which has a material of its own and would hold a fixed child
// inside itself — and both halves read the flag here.
const signInOpen = ref(false)
const signOutOpen = ref(false)

Events.On('account:changed', (ev) => {
  const arriving = ev.data as State | null
  if (!arriving) return
  state.value = arriving
  failure.value = arriving.failure ? (refusalText(arriving.failure) ?? '') : ''
})

export function useAccount() {
  return {
    state,
    sessions,
    failure,
    sessionsFailed,
    busy,
    signInOpen,
    openSignIn,
    closeSignIn,
    signOutOpen,
    openSignOut,
    closeSignOut,
    loadAccount,
    beginSignIn,
    cancelSignIn,
    setServer,
    signOut,
    loadSessions,
    endSession,
    endOthers,
    deleteAccount,
  }
}

async function loadAccount(): Promise<void> {
  try {
    state.value = await AccountService.State()
  } catch {
    // Without an answer the pane draws "not signed in", which is the state the app is usable in.
  }
}

// openSignIn raises the dialog, reading the account first: it draws the address a sign-in would go
// to, and a window that opened the menu without asking Go would offer whatever it last saw.
function openSignIn(): void {
  signInOpen.value = true
  void loadAccount()
}

function closeSignIn(): void {
  signInOpen.value = false
}

// openSignOut raises the confirmation: leaving is not something to do on a stray click, and the list
// of what stays is what makes it a decision rather than an accident.
function openSignOut(): void {
  signOutOpen.value = true
}

function closeSignOut(): void {
  signOutOpen.value = false
}

// beginSignIn asks for a code and opens the browser. It answers as soon as there is a code: what
// happens after that arrives as an event, because the person is on the far side of a browser the app
// does not control.
async function beginSignIn(server: string): Promise<void> {
  busy.value = true
  failure.value = ''
  try {
    state.value = await AccountService.Begin(server)
  } catch (error) {
    failure.value = describeFailure(error)
  } finally {
    busy.value = false
  }
}

async function cancelSignIn(): Promise<void> {
  // Giving up closes the dialog as well: the wait is over, and a spinner over a sign-in nobody is
  // waiting for is a lie. Closing it with the cross is the other thing a person can do — that one
  // leaves the sign-in running, and confirming the code in the browser still signs the app in.
  signInOpen.value = false
  try {
    state.value = await AccountService.Cancel()
  } catch (error) {
    failure.value = describeFailure(error)
  }
}

async function setServer(server: string): Promise<boolean> {
  try {
    state.value = await AccountService.SetServer(server)
    failure.value = ''
    return true
  } catch (error) {
    failure.value = describeFailure(error)
    return false
  }
}

async function signOut(): Promise<void> {
  // The dialog that asked goes first: a sheet still standing over a window that has already left
  // would be asking about something that has happened.
  signOutOpen.value = false
  try {
    state.value = await AccountService.SignOut()
    sessions.value = []
  } catch (error) {
    failure.value = describeFailure(error)
  }
}

// loadSessions asks the server. It can fail with the server being away, and that is a sentence on the
// card rather than an empty list: an empty list would say nobody is signed in anywhere, which is a
// different thing from not knowing.
async function loadSessions(): Promise<void> {
  sessionsFailed.value = false
  try {
    sessions.value = (await AccountService.Sessions()) ?? []
  } catch {
    sessionsFailed.value = true
  }
}

async function endSession(sessionID: string): Promise<void> {
  try {
    sessions.value = (await AccountService.EndSession(sessionID)) ?? []
  } catch (error) {
    failure.value = describeFailure(error)
    await loadSessions()
  }
}

async function endOthers(): Promise<void> {
  try {
    sessions.value = (await AccountService.EndOthers()) ?? []
  } catch (error) {
    failure.value = describeFailure(error)
  }
}

async function deleteAccount(): Promise<void> {
  try {
    state.value = await AccountService.DeleteAccount()
    sessions.value = []
  } catch (error) {
    failure.value = describeFailure(error)
  }
}
