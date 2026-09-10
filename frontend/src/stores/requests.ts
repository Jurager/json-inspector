import { defineStore } from 'pinia'
import type { RequestRecord } from '../lib/types'

let counter = 0
function nextId(): string {
  counter += 1
  return `req-${Date.now()}-${counter}`
}

export const useRequestsStore = defineStore('requests', {
  state: () => ({
    requests: [] as RequestRecord[],
    selectedId: null as string | null,
    activeView: 'request' as 'request' | 'browser',
    capturing: false,
    unreadCount: 0,
    loading: false,
  }),
  getters: {
    selected(state): RequestRecord | null {
      return state.requests.find((r) => r.id === state.selectedId) ?? null
    },
  },
  actions: {
    add(record: Omit<RequestRecord, 'id' | 'startedAt'>) {
      const full: RequestRecord = {
        ...record,
        id: nextId(),
        startedAt: Date.now(),
      }
      this.requests.unshift(full)
      this.selectedId = full.id
    },
    addCaptured(record: Omit<RequestRecord, 'id' | 'source' | 'startedAt'>) {
      const full: RequestRecord = {
        ...record,
        id: nextId(),
        startedAt: Date.now(),
        source: 'browser',
      }
      this.requests.unshift(full)
      if (this.activeView !== 'browser') {
        this.unreadCount += 1
      }
    },
    select(id: string) {
      this.selectedId = id
    },
    markBrowserRead() {
      this.unreadCount = 0
    },
    clear() {
      this.requests = []
      this.selectedId = null
      this.unreadCount = 0
    },
  },
})
