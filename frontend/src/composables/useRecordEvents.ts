import { onBeforeUnmount, onMounted } from 'vue'
import { Events } from '@wailsio/runtime'
import { RecordSource } from '../../bindings/json-inspector/internal/domain'
import { useRequestsStore } from '../stores/requests'
import { useCollectionsStore } from '../stores/collections'
import { useToast } from './useToast'

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
          store.setCaptureState({ connected: true, recording: true })
        }
        store.addIngested(ev.data)
      }),

      // The attempt a pane started has come back, and it carries the record history keeps.
      Events.On('request:finished', (ev) => {
        const { id, record } = ev.data
        if (store.mine.includes(id)) void store.finishSend(record)
        else if (collections.mine.includes(id)) void collections.finishSend(record)
      }),

      // No record at all: this side failed before the request became history — a body the database
      // would not take, an engine that could not start. A request that never reached the server
      // still has a record, so there is nothing to show but the reason.
      Events.On('request:failed', (ev) => {
        if (collections.mine.includes(ev.data.id)) {
          collections.failSend()
          toast.show(`Запрос не выполнен: ${ev.data.error}`, 'error')
          return
        }
        store.failSend()
        toast.show(`Запрос не выполнен: ${ev.data.error}`, 'error')
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
