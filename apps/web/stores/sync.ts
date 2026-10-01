import { defineStore } from 'pinia'
import { isApiError, isNetworkError } from '@reliefmesh/api-client'
import { type OutboxEntry, getDB, getMeta, metaKeys } from '~/utils/offline-db'
import { type EnqueueInput, type SubmitResult, SyncEngine } from '~/utils/sync-engine'

const HEALTH_INTERVAL_MS = 20_000
const FLUSH_INTERVAL_MS = 30_000
const PULL_INTERVAL_MS = 120_000
const FULL_PULL_AFTER_MS = 6 * 60 * 60 * 1000

let engine: SyncEngine | null = null
let timers: ReturnType<typeof setInterval>[] = []
let started = false

/**
 * Connection and synchronization state.
 *
 * "online" means the ReliefMesh server is reachable, not merely that the
 * device has a network connection (a shelter Wi-Fi without uplink to the
 * server is shown as offline).
 */
export const useSyncStore = defineStore('sync', {
  state: () => ({
    browserOnline: typeof navigator === 'undefined' ? true : navigator.onLine,
    serverReachable: true,
    syncing: false,
    pending: 0,
    problems: 0,
    foreignPending: 0,
    lastSyncAt: null as string | null,
    lastPullAt: null as string | null,
    authRequired: false,
  }),
  getters: {
    online: (s) => s.browserOnline && s.serverReachable,
    connection(): 'online' | 'offline' | 'unreachable' {
      if (!this.browserOnline) return 'offline'
      return this.serverReachable ? 'online' : 'unreachable'
    },
  },
  actions: {
    engine(): SyncEngine {
      if (!engine) {
        engine = new SyncEngine({
          db: getDB(),
          api: useApi(),
          getUserId: () => useAuthStore().user?.id ?? null,
          onChange: () => void this.refreshCounts(),
        })
      }
      return engine
    },
    markReachable(ok: boolean) {
      const wasReachable = this.serverReachable
      this.serverReachable = ok
      if (ok && !wasReachable) void this.onReconnect()
    },
    async refreshCounts() {
      const e = this.engine()
      const c = await e.counts()
      this.pending = c.pending
      this.problems = c.problems
      this.foreignPending = (await e.foreignEntries()).length
      const uid = useAuthStore().user?.id
      if (uid) this.lastSyncAt = (await getMeta<string>(getDB(), metaKeys.lastSync(uid))) ?? null
    },
    async checkServer() {
      this.browserOnline = navigator.onLine
      if (!this.browserOnline) {
        this.serverReachable = false
        return
      }
      try {
        await useApi().health()
        this.markReachable(true)
      } catch {
        this.markReachable(false)
      }
    },
    async onReconnect() {
      const auth = useAuthStore()
      if (auth.offlineSession) await auth.revalidate()
      await this.flushNow()
      await this.pullNow()
    },
    async flushNow() {
      const auth = useAuthStore()
      if (auth.status !== 'authenticated' || !auth.csrf || !this.online) return
      this.syncing = true
      try {
        const report = await this.engine().flush()
        this.authRequired = report.authRequired
        if (report.offline) this.serverReachable = false
        if (report.applied > 0) useToastStore().success(`${report.applied} offline change(s) synchronized.`)
        if (report.problems > 0) useToastStore().warning(`${report.problems} change(s) need your attention. Open "Pending changes".`)
      } finally {
        this.syncing = false
        await this.refreshCounts()
      }
    },
    async pullNow(forceFull = false) {
      const auth = useAuthStore()
      if (auth.status !== 'authenticated' || auth.offlineSession || !this.online) return
      const uid = auth.user!.id
      const last = await getMeta<string>(getDB(), metaKeys.lastPull(uid))
      const full = forceFull || !last || Date.now() - new Date(last).getTime() > FULL_PULL_AFTER_MS
      try {
        const res = await this.engine().pull(full)
        if (res) {
          useSettingsStore().apply(res.settings)
          this.lastPullAt = res.server_time
        }
      } catch (e) {
        if (isNetworkError(e)) this.serverReachable = false
        else if (!isApiError(e)) throw e
      }
      await this.refreshCounts()
    },
    /** Offline-capable mutation; see SyncEngine.submit. */
    async submit(input: EnqueueInput, opts: { dropOnReject?: boolean } = {}): Promise<SubmitResult> {
      const auth = useAuthStore()
      let res: SubmitResult
      if (!this.online || !auth.csrf) {
        const entry = await this.engine().enqueue(input)
        res = { status: 'queued', opId: entry.op_id }
      } else {
        res = await this.engine().submit(input, opts)
      }
      await this.refreshCounts()
      return res
    },
    async entries(): Promise<OutboxEntry[]> {
      return this.engine().entries()
    },
    async foreignEntries(): Promise<OutboxEntry[]> {
      return this.engine().foreignEntries()
    },
    async retry(opId: string) {
      await this.engine().retry(opId)
      await this.flushNow()
    },
    async discard(opId: string) {
      const removed = await this.engine().discard(opId)
      await this.refreshCounts()
      return removed
    },
    /** Removes outbox entries of other accounts (explicit user action). */
    async discardForeign() {
      const db = getDB()
      const list = await this.engine().foreignEntries()
      await db.outbox.bulkDelete(list.map((e) => e.op_id))
      await this.refreshCounts()
    },
    start() {
      if (started || typeof window === 'undefined') return
      started = true
      window.addEventListener('online', () => {
        this.browserOnline = true
        void this.checkServer()
      })
      window.addEventListener('offline', () => {
        this.browserOnline = false
        this.serverReachable = false
      })
      document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') void this.checkServer().then(() => this.flushNow())
      })
      timers.push(setInterval(() => void this.checkServer(), HEALTH_INTERVAL_MS))
      timers.push(setInterval(() => { if (this.pending > 0) void this.flushNow() }, FLUSH_INTERVAL_MS))
      timers.push(setInterval(() => { if (document.visibilityState === 'visible') void this.pullNow() }, PULL_INTERVAL_MS))
      void this.checkServer().then(async () => {
        await this.refreshCounts()
        await this.flushNow()
        await this.pullNow()
      })
    },
    stop() {
      timers.forEach(clearInterval)
      timers = []
      started = false
    },
  },
})
