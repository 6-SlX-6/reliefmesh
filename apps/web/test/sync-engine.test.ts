import { beforeEach, describe, expect, it, vi } from 'vitest'
import { NetworkError } from '@reliefmesh/api-client'
import type { SyncOperation, SyncPushResult } from '@reliefmesh/shared-types'
import { ReliefMeshDB } from '../utils/offline-db'
import { SyncEngine } from '../utils/sync-engine'

let db: ReliefMeshDB
let userId: string | null
let push: ReturnType<typeof vi.fn>
let pull: ReturnType<typeof vi.fn>
let engine: SyncEngine

function result(ops: SyncOperation[], fn: (op: SyncOperation) => Partial<SyncPushResult['results'][number]>): SyncPushResult {
  return { server_time: new Date().toISOString(), results: ops.map((op) => ({ op_id: op.op_id, status: 'applied', ...fn(op) })) }
}

beforeEach(async () => {
  db = new ReliefMeshDB('test-' + Math.random())
  userId = 'user-1'
  push = vi.fn()
  pull = vi.fn()
  engine = new SyncEngine({ db, api: { sync: { push: push as never, pull: pull as never } }, getUserId: () => userId })
})

describe('SyncEngine', () => {
  it('pushes pending operations, caches the result and removes local placeholders', async () => {
    await db.requests.put({ id: 'local:c1', user_id: 'user-1', client_id: 'c1', pending: true, data: {} as never, cached_at: '' })
    await engine.enqueue({ type: 'request.create', payload: { client_id: 'c1', title: 'Water' }, summary: 'New request' })
    push.mockImplementation(async (ops: SyncOperation[]) =>
      result(ops, () => ({ entity_type: 'aid_request', entity_id: 'srv-1', reference: 'RM-2026-0000001', entity: { id: 'srv-1', client_id: 'c1' } })))
    const report = await engine.flush()
    expect(report.applied).toBe(1)
    expect(await db.outbox.count()).toBe(0)
    expect(await db.requests.get('local:c1')).toBeUndefined()
    expect((await db.requests.get('srv-1'))?.data).toMatchObject({ id: 'srv-1' })
  })

  it('keeps operations when the server is unreachable', async () => {
    await engine.enqueue({ type: 'request.note', entity_id: 'r1', payload: { client_id: 'n1', body: 'hi' }, summary: 'Note' })
    push.mockRejectedValue(new NetworkError())
    const report = await engine.flush()
    expect(report.offline).toBe(true)
    const [entry] = await db.outbox.toArray()
    expect(entry?.state).toBe('pending')
    expect(entry?.attempts).toBe(1)
  })

  it('never discards conflicts silently', async () => {
    await engine.enqueue({ type: 'request.status', entity_id: 'r1', payload: { status: 'verified' }, summary: 'Status' })
    push.mockImplementation(async (ops: SyncOperation[]) =>
      result(ops, () => ({ status: 'conflict', entity_type: 'aid_request', error: { code: 'invalid_transition', message: 'x' }, entity: { id: 'r1', status: 'cancelled' } })))
    const report = await engine.flush()
    expect(report.problems).toBe(1)
    const [entry] = await db.outbox.toArray()
    expect(entry?.state).toBe('conflict')
    expect(entry?.server_entity).toMatchObject({ status: 'cancelled' })
    expect((await db.requests.get('r1'))?.data).toMatchObject({ status: 'cancelled' })
    // Explicit retry puts it back into the queue.
    await engine.retry(entry!.op_id)
    expect((await db.outbox.get(entry!.op_id))?.state).toBe('pending')
  })

  it('keeps dependent operations pending while their record is not synchronized', async () => {
    await engine.enqueue({ type: 'request.create', payload: { client_id: 'c2' }, summary: 'Create' })
    await engine.enqueue({ type: 'request.note', entity_client_id: 'c2', payload: { client_id: 'n2', body: 'x' }, summary: 'Note' })
    push.mockImplementation(async (ops: SyncOperation[]) =>
      result(ops, (op) => op.type === 'request.create'
        ? { status: 'rejected', error: { code: 'validation_failed', message: 'bad' } }
        : { status: 'rejected', error: { code: 'not_found', message: 'missing' } }))
    await engine.flush()
    const entries = await engine.entries()
    expect(entries.find((e) => e.type === 'request.create')?.state).toBe('rejected')
    expect(entries.find((e) => e.type === 'request.note')?.state).toBe('pending')
  })

  it('discarding a create also discards dependent operations and the placeholder', async () => {
    await db.requests.put({ id: 'local:c3', user_id: 'user-1', client_id: 'c3', pending: true, data: {} as never, cached_at: '' })
    const create = await engine.enqueue({ type: 'request.create', payload: { client_id: 'c3' }, summary: 'Create' })
    await engine.enqueue({ type: 'request.note', entity_client_id: 'c3', payload: { client_id: 'n3', body: 'x' }, summary: 'Note' })
    const removed = await engine.discard(create.op_id)
    expect(removed).toHaveLength(2)
    expect(await db.outbox.count()).toBe(0)
    expect(await db.requests.get('local:c3')).toBeUndefined()
  })

  it('only pushes operations of the signed-in user', async () => {
    await engine.enqueue({ type: 'request.note', entity_id: 'r1', payload: { body: 'a' }, summary: 'A' })
    userId = 'user-2'
    await engine.enqueue({ type: 'request.note', entity_id: 'r1', payload: { body: 'b' }, summary: 'B' })
    push.mockImplementation(async (ops: SyncOperation[]) => result(ops, () => ({})))
    await engine.flush()
    expect(push).toHaveBeenCalledTimes(1)
    expect((push.mock.calls[0]![0] as SyncOperation[]).map((o) => (o.payload as { body: string }).body)).toEqual(['b'])
    expect((await engine.foreignEntries()).map((e) => e.summary)).toEqual(['A'])
  })

  it('submit drops rejected operations when the form shows the error', async () => {
    push.mockImplementation(async (ops: SyncOperation[]) =>
      result(ops, () => ({ status: 'rejected', error: { code: 'validation_failed', message: 'bad', fields: { title: 'Required' } } })))
    const res = await engine.submit({ type: 'request.create', payload: { client_id: 'c4' }, summary: 'x' }, { dropOnReject: true })
    expect(res.status).toBe('rejected')
    expect(await db.outbox.count()).toBe(0)
  })

  it('submit queues the operation when offline', async () => {
    push.mockRejectedValue(new NetworkError())
    const res = await engine.submit({ type: 'request.create', payload: { client_id: 'c5' }, summary: 'x' }, { dropOnReject: true })
    expect(res.status).toBe('queued')
    expect(await db.outbox.count()).toBe(1)
  })

  it('full pull replaces cached records but keeps local placeholders', async () => {
    await db.requests.bulkPut([
      { id: 'old', user_id: 'user-1', data: { id: 'old' } as never, cached_at: '' },
      { id: 'local:c6', user_id: 'user-1', pending: true, data: {} as never, cached_at: '' },
    ])
    pull.mockResolvedValue({ server_time: '2026-10-01T00:00:00Z', full: true, has_more: false,
      settings: { settings: {}, categories: [] }, requests: [{ id: 'new' }], offers: [], assignments: [] })
    await engine.pull(true)
    expect((await db.requests.toArray()).map((r) => r.id).sort()).toEqual(['local:c6', 'new'])
    expect((await db.meta.get('last_pull:user-1'))?.value).toBe('2026-10-01T00:00:00Z')
  })
})
