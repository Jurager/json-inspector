import { onBeforeUnmount, onMounted } from 'vue'
import { Events } from '@wailsio/runtime'
import { RecordSource } from '../../bindings/json-inspector/internal/domain'
import { useRequestsStore } from '../stores/requests'
import { useCollectionsStore } from '../stores/collections'
import { useToast } from './useToast'
import { refusalText, t as tr } from '../i18n'
import { holdAnswer } from '../lib/earlyAnswers'

// What Go publishes about history and about the attempts that fill it. The payload types come from
// the generated bindings, so a field renamed on that side is a compile error here.
//
// An answer belongs to whoever asked: the records this app sends carry the id of the attempt that
// produced them, and both panes keep the ids they started. A run's requests go through the same
// path, and without that check every one of them would take over the command line's pane.
export function useRecordEvents(
  store: ReturnType<typeof useRequestsStore>,
  collections: ReturnType<typeof useCollectionsStore>
) {
  const offs: (() => void)[] = []
  const toast = useToast()

  onMounted(() => {
    offs.push(
      // A record appeared: one the browser made, or the sample the window asked for. A capture
      // arriving is itself proof the extension is connected and recording — a belt-and-suspenders
      // check against a stray `capture-disconnected` for a since-replaced socket (see server.go).
      Events.On('record:added', (ev) => {
        if (ev.data.source === RecordSource.SourceBrowser) {
          store.setCaptureState({ connected: true, recording: true, paused: false })
        }
        store.addIngested(ev.data)
      }),

      // The attempt a pane started has come back, and it carries the record history keeps. A pane
      // that does not know this id yet is one whose send is still on its way back from the call that
      // started it: the answer waits under its id until that pane claims it (see lib/earlyAnswers).
      Events.On('request:finished', (ev) => {
        const { id, record } = ev.data
        if (store.mine.includes(id)) void store.finishSend(record)
        else if (collections.mine.includes(id)) void collections.finishSend(record)
        else holdAnswer(id, { record })
      }),

      // No record at all: this side failed before the request became history — a body the database
      // would not take, an engine that could not start. A request that never reached the server
      // still has a record, so there is nothing to show but the reason.
      //
      // The reason is the window's, not a pane's: whichever one asked, the attempt failed. Which
      // spinner it stops is the pane's own, and an id neither pane knows yet waits like any answer.
      Events.On('request:failed', (ev) => {
        const { id } = ev.data
        toast.show(tr('errors.requestFailed', { error: refusalText(ev.data.failure) ?? ev.data.error }), 'error')
        if (collections.mine.includes(id)) collections.failSend()
        else if (store.mine.includes(id)) store.failSend()
        else holdAnswer(id, { failed: true })
      }),

      // A history thrown away — the settings window's "clear history now". Nothing else reloads this
      // list, so a window that ignored the news would go on drawing rows the database no longer has.
      //
      // A list that knows it is another space's is left alone: its rows are still there, and reading
      // the database again would only cost. Both sides have to be known for that — a window whose own
      // space is not named yet reads anyway, because the read is about the workspace Go has on
      // screen, which is the one that was just cleared.
      Events.On('history:cleared', (ev) => {
        if (store.workspaceId && ev.data.workspaceId && ev.data.workspaceId !== store.workspaceId) {
          return
        }
        void store.load()
      }),

      // A run of a collection: one event per request it reaches, and the finished run with its
      // counters — which is what the overview draws, and what the status bar counts.
      Events.On('collection:run-progress', (ev) => {
        collections.applyRunProgress(ev.data)
      }),
      Events.On('collection:run-finished', (ev) => {
        collections.applyRunFinished(ev.data)
      })
    )
  })

  onBeforeUnmount(() => offs.forEach((off) => off()))
}
