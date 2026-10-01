// Offline synchronization engine.
//
// All offline-capable mutations (creating requests, offers and notes; status
// changes; handover updates) are written to the IndexedDB outbox first and
// then pushed to /api/v1/sync/push in their original order. The engine:
//
//   * never discards a change silently: conflicts and rejections stay in the
//     outbox until the user explicitly retries or discards them;
//   * is idempotent: every operation has a stable op_id and every create a
//     client_id, so retries after timeouts cannot create duplicates;
//   * keeps operations that depend on a not-yet-synchronized record (e.g. a
//     note on an offline-created request) pending instead of failing them;
//   * only pushes operations of the signed-in user.
import type {
  AidRequest,
  ApiErrorDetail,
  Assignment,
  Offer,
  SyncOperation,
  SyncOperationType,
  SyncOpResult,
  SyncPullResult,
  SyncPushResult,
} from '@reliefmesh/shared-types'
import { isApiError, isNetworkError } from '@reliefmesh/api-client'
import {
  cacheEntities,
  type OutboxEntry,
  type ReliefMeshDB,
  metaKeys,
  replaceEntities,
  setMeta,
} from './offline-db'
import { uuid } from './uuid'

export interface SyncApi {
  sync: {
    push: (ops: SyncOperation[]) => Promise<SyncPushResult>
    pull: (since?: string) => Promise<SyncPullResult>
  }
}

export interface EnqueueInput {
  type: SyncOperationType
  entity_id?: string
  entity_client_id?: string
  payload: unknown
  summary: string
}

export interface FlushReport {
  applied: number
  problems: number
  offline: boolean
  authRequired: boolean
}

export type SubmitResult =
  | { status: 'applied'; entity?: unknown; entityId?: string; reference?: string }
  | { status: 'queued'; opId: string }
  | { status: 'rejected' | 'conflict'; error?: ApiErrorDetail; entity?: unknown; opId: string }

export interface SyncEngineOptions {
  db: ReliefMeshDB
  api: SyncApi
  getUserId: () => string | null
  /** Notified whenever outbox or cache contents change. */
  onChange?: () => void
  batchSize?: number
}

const CREATE_TYPES: SyncOperationType[] = ['request.create', 'offer.create']

export class SyncEngine {
  private flushing: Promise<FlushReport> | null = null
  private readonly batchSize: number

  constructor(private readonly opts: SyncEngineOptions) {
    this.batchSize = opts.batchSize ?? 50
  }

  private changed() {
    this.opts.onChange?.()
  }

  /** Adds an operation to the outbox. */
  async enqueue(input: EnqueueInput): Promise<OutboxEntry> {
    const userId = this.opts.getUserId()
    if (!userId) throw new Error('Not signed in')
    const entry: OutboxEntry = {
      op_id: uuid(),
      user_id: userId,
      type: input.type,
      entity_id: input.entity_id,
      entity_client_id: input.entity_client_id,
      payload: input.payload,
      created_at: new Date().toISOString(),
      state: 'pending',
      attempts: 0,
      summary: input.summary,
    }
    await this.opts.db.outbox.put(entry)
    this.changed()
    return entry
  }

  /** Outbox entries of the current user, oldest first. */
  async entries(): Promise<OutboxEntry[]> {
    const userId = this.opts.getUserId()
    if (!userId) return []
    return this.opts.db.outbox.where('user_id').equals(userId).sortBy('created_at')
  }

  /** Outbox entries that belong to other accounts on this device. */
  async foreignEntries(): Promise<OutboxEntry[]> {
    const userId = this.opts.getUserId()
    return (await this.opts.db.outbox.toArray()).filter((e) => e.user_id !== userId)
  }

  async counts(): Promise<{ pending: number; problems: number }> {
    const list = await this.entries()
    return {
      pending: list.filter((e) => e.state === 'pending').length,
      problems: list.filter((e) => e.state !== 'pending').length,
    }
  }

  /** Pushes pending operations. Concurrent calls share one run. */
  flush(): Promise<FlushReport> {
    if (!this.flushing) {
      this.flushing = this.doFlush().finally(() => {
        this.flushing = null
      })
    }
    return this.flushing
  }

  private async doFlush(): Promise<FlushReport> {
    const report: FlushReport = { applied: 0, problems: 0, offline: false, authRequired: false }
    const { db, api } = this.opts
    // Guards against endless loops if the server keeps answering "retry".
    for (let round = 0; round < 20; round++) {
      const all = await this.entries()
      const pending = all.filter((e) => e.state === 'pending').slice(0, this.batchSize)
      if (!pending.length) break
      let res: SyncPushResult
      try {
        res = await api.sync.push(pending.map(toOperation))
      } catch (e) {
        if (isNetworkError(e)) report.offline = true
        else if (isApiError(e) && e.status === 401) report.authRequired = true
        else if (isApiError(e) && e.code === 'password_change_required') report.authRequired = true
        await this.touch(pending)
        break
      }
      const unsyncedCreates = new Set(
        all.filter((e) => CREATE_TYPES.includes(e.type)).map((e) => clientIdOf(e)).filter(Boolean) as string[],
      )
      let retryLater = false
      for (const r of res.results) {
        const entry = pending.find((p) => p.op_id === r.op_id)
        if (!entry) continue
        const outcome = await this.applyResult(entry, r, unsyncedCreates)
        if (outcome === 'applied') {
          report.applied++
          const cid = clientIdOf(entry)
          if (cid) unsyncedCreates.delete(cid)
        } else if (outcome === 'problem') report.problems++
        else retryLater = true
      }
      this.changed()
      if (retryLater) break
    }
    const userId = this.opts.getUserId()
    if (userId && !report.offline) await setMeta(db, metaKeys.lastSync(userId), new Date().toISOString())
    return report
  }

  private async touch(entries: OutboxEntry[]) {
    const now = new Date().toISOString()
    await this.opts.db.outbox.bulkPut(entries.map((e) => ({ ...e, attempts: e.attempts + 1, last_attempt_at: now })))
  }

  private async applyResult(entry: OutboxEntry, r: SyncOpResult, unsyncedCreates: Set<string>): Promise<'applied' | 'problem' | 'retry'> {
    const { db } = this.opts
    const now = new Date().toISOString()
    switch (r.status) {
      case 'applied':
      case 'duplicate':
        await db.outbox.delete(entry.op_id)
        await this.storeEntity(entry, r)
        return 'applied'
      case 'retry':
        await db.outbox.put({ ...entry, attempts: entry.attempts + 1, last_attempt_at: now, last_error: r.error })
        return 'retry'
      case 'rejected':
        // Depends on a record that is still waiting in the outbox: keep it.
        if (r.error?.code === 'not_found' && entry.entity_client_id && unsyncedCreates.has(entry.entity_client_id)) {
          await db.outbox.put({ ...entry, attempts: entry.attempts + 1, last_attempt_at: now })
          return 'retry'
        }
        await db.outbox.put({ ...entry, state: 'rejected', attempts: entry.attempts + 1, last_attempt_at: now, last_error: r.error })
        return 'problem'
      case 'conflict':
        await db.outbox.put({
          ...entry, state: 'conflict', attempts: entry.attempts + 1, last_attempt_at: now,
          last_error: r.error, server_entity: r.entity,
        })
        await this.storeEntity(entry, r)
        return 'problem'
    }
  }

  private async storeEntity(entry: OutboxEntry, r: SyncOpResult) {
    const userId = entry.user_id
    const { db } = this.opts
    if (!r.entity) return
    if (r.entity_type === 'aid_request') {
      await cacheEntities(db, 'requests', userId, [r.entity as AidRequest])
      if (entry.type === 'request.create') await db.requests.delete(`local:${clientIdOf(entry)}`)
    } else if (r.entity_type === 'offer') {
      await cacheEntities(db, 'offers', userId, [r.entity as Offer])
      if (entry.type === 'offer.create') await db.offers.delete(`local:${clientIdOf(entry)}`)
    } else if (r.entity_type === 'assignment') {
      await cacheEntities(db, 'assignments', userId, [r.entity as Assignment])
    }
  }

  /**
   * Enqueues an operation and, when online, pushes it immediately and returns
   * its result. With `dropOnReject`, a rejected operation is removed from the
   * outbox because the caller shows the error in the still-filled form.
   */
  async submit(input: EnqueueInput, opts: { dropOnReject?: boolean } = {}): Promise<SubmitResult> {
    const entry = await this.enqueue(input)
    let result: SyncOpResult | undefined
    try {
      const res = await this.opts.api.sync.push([toOperation(entry)])
      result = res.results[0]
    } catch (e) {
      if (isNetworkError(e) || (isApiError(e) && e.status === 401)) return { status: 'queued', opId: entry.op_id }
      throw e
    }
    if (!result) return { status: 'queued', opId: entry.op_id }
    const outcome = await this.applyResult(entry, result, new Set())
    this.changed()
    if (outcome === 'applied') {
      void this.flush()
      return { status: 'applied', entity: result.entity, entityId: result.entity_id, reference: result.reference }
    }
    if (outcome === 'retry') return { status: 'queued', opId: entry.op_id }
    if (opts.dropOnReject && result.status === 'rejected') {
      await this.opts.db.outbox.delete(entry.op_id)
      this.changed()
    }
    return { status: result.status as 'rejected' | 'conflict', error: result.error, entity: result.entity, opId: entry.op_id }
  }

  /** Marks a problematic operation for another attempt. */
  async retry(opId: string): Promise<void> {
    const e = await this.opts.db.outbox.get(opId)
    if (!e) return
    await this.opts.db.outbox.put({ ...e, state: 'pending', last_error: undefined, server_entity: undefined })
    this.changed()
  }

  /**
   * Explicitly discards an operation (after user confirmation). Operations
   * that depend on a discarded offline-created record are discarded too, and
   * so is the local placeholder. Returns the discarded entries.
   */
  async discard(opId: string): Promise<OutboxEntry[]> {
    const { db } = this.opts
    const e = await db.outbox.get(opId)
    if (!e) return []
    const removed: OutboxEntry[] = [e]
    const cid = clientIdOf(e)
    if (cid && CREATE_TYPES.includes(e.type)) {
      const dependents = (await db.outbox.toArray()).filter((x) => x.entity_client_id === cid)
      removed.push(...dependents)
      await db.requests.delete(`local:${cid}`)
      await db.offers.delete(`local:${cid}`)
    }
    await db.outbox.bulkDelete(removed.map((x) => x.op_id))
    this.changed()
    return removed
  }

  /** Pulls server data into the local cache. */
  async pull(full = false): Promise<SyncPullResult | null> {
    const userId = this.opts.getUserId()
    if (!userId) return null
    const { db, api } = this.opts
    const since = full ? undefined : ((await db.meta.get(metaKeys.lastPull(userId)))?.value as string | undefined)
    const res = await api.sync.pull(since)
    if (res.full) {
      await replaceEntities(db, 'requests', userId, res.requests)
      await replaceEntities(db, 'offers', userId, res.offers)
      await replaceEntities(db, 'assignments', userId, res.assignments)
    } else {
      await cacheEntities(db, 'requests', userId, res.requests)
      await cacheEntities(db, 'offers', userId, res.offers)
      await cacheEntities(db, 'assignments', userId, res.assignments)
    }
    await setMeta(db, metaKeys.settings, res.settings)
    // A full refresh is needed periodically so records that became invisible
    // (e.g. a cancelled assignment) disappear from the device.
    await setMeta(db, metaKeys.lastPull(userId), res.server_time)
    await setMeta(db, metaKeys.lastSync(userId), new Date().toISOString())
    this.changed()
    return res
  }
}

export function clientIdOf(e: OutboxEntry): string | undefined {
  const p = e.payload as { client_id?: string } | null
  return p?.client_id
}

export function toOperation(e: OutboxEntry): SyncOperation {
  return {
    op_id: e.op_id,
    type: e.type,
    entity_id: e.entity_id,
    entity_client_id: e.entity_client_id,
    payload: e.payload,
    client_created_at: e.created_at,
  }
}
