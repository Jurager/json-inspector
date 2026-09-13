import { onBeforeUnmount, onMounted } from 'vue'
import { Events } from '@wailsio/runtime'
import { RecordSource } from '../../bindings/json-inspector/internal/domain'
import { useRequestsStore } from '../stores/requests'
import { useToast } from './useToast'

// What Go publishes about history and about the attempts that fill it. The payload types come from
// the generated bindings, so a field renamed on that side is a compile error here.
export function useRecordEvents(store: ReturnType<typeof useRequestsStore>) {
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

      // The attempt the window started has come back, and it carries the record history keeps.
      Events.On('request:finished', (ev) => {
        void store.finishSend(ev.data.record)
      }),

      // No record at all: this side failed before the request became history — a body the database
      // would not take, an engine that could not start. A request that never reached the server
      // still has a record, so there is nothing to show but the reason.
      Events.On('request:failed', (ev) => {
        store.failSend()
        toast.show(`Запрос не выполнен: ${ev.data.error}`, 'error')
      })
    )
  })

  onBeforeUnmount(() => offs.forEach((off) => off()))
}
