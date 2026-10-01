// Local IndexedDB storage for offline use (Dexie).
//
// Privacy rules for this cache:
//   * Only data the signed-in user is already allowed to see is cached (the
//     server's role projections). Sensitive descriptions are not part of list
//     responses and therefore only cached for records the user opened.
//   * Protected values (contact details, exact locations) revealed through the
//     audited endpoints are NEVER written here; they live in memory only.
//   * Cached records are scoped to the user who fetched them and are cleared on
//     logout. The outbox of unsynchronized changes is only cleared after the
//     user explicitly confirms (never silently).
import Dexie, { type Table } from 'dexie'
import type {
  AidRequest,
  ApiErrorDetail,
  Assignment,
  ClientSettings,
  Me,
  Offer,
  SyncOperationType,
} from '@reliefmesh/shared-types'

export type OutboxState = 'pending' | 'conflict' | 'rejected'

export interface OutboxEntry {
  op_id: string
  user_id: string
  type: SyncOperationType
  entity_id?: string
  entity_client_id?: string
  payload: unknown
  created_at: string
  state: OutboxState
  attempts: number
  /** Short human description shown in the sync screen. */
  summary: string
  last_error?: ApiErrorDetail
  /** Current server version of the entity (for conflicts). */
  server_entity?: unknown
  last_attempt_at?: string
}

export interface CachedEntity<T> {
  /** Server id, or `local:<client_id>` for records created offline. */
  id: string
  user_id: string
  client_id?: string
  /** True for local placeholders not yet confirmed by the server. */
  pending?: boolean
  data: T
  cached_at: string
}

export interface MetaEntry {
  key: string
  value: unknown
}

export class ReliefMeshDB extends Dexie {
  outbox!: Table<OutboxEntry, string>
  requests!: Table<CachedEntity<AidRequest>, string>
  offers!: Table<CachedEntity<Offer>, string>
  assignments!: Table<CachedEntity<Assignment>, string>
  meta!: Table<MetaEntry, string>

  constructor(name = 'reliefmesh') {
    super(name)
    this.version(1).stores({
      outbox: 'op_id, user_id, state, created_at',
      requests: 'id, user_id, client_id',
      offers: 'id, user_id, client_id',
      assignments: 'id, user_id',
      meta: 'key',
    })
  }
}

let instance: ReliefMeshDB | null = null

/** Shared database instance (browser only). */
export function getDB(): ReliefMeshDB {
  if (!instance) instance = new ReliefMeshDB()
  return instance
}

export type EntityKind = 'requests' | 'offers' | 'assignments'
type EntityOf<K extends EntityKind> = K extends 'requests' ? AidRequest : K extends 'offers' ? Offer : Assignment

/** Upserts server entities for a user. */
export async function cacheEntities<K extends EntityKind>(db: ReliefMeshDB, kind: K, userId: string, items: EntityOf<K>[]): Promise<void> {
  if (!items.length) return
  const now = new Date().toISOString()
  const table = db[kind] as unknown as Table<CachedEntity<EntityOf<K>>, string>
  const plain = JSON.parse(JSON.stringify(items)) as EntityOf<K>[]
  await table.bulkPut(plain.map((data) => ({
    id: (data as { id: string }).id,
    user_id: userId,
    client_id: (data as { client_id?: string }).client_id,
    data,
    cached_at: now,
  })))
}

/** Replaces all non-pending cached entities of a kind for a user. */
export async function replaceEntities<K extends EntityKind>(db: ReliefMeshDB, kind: K, userId: string, items: EntityOf<K>[]): Promise<void> {
  const table = db[kind] as unknown as Table<CachedEntity<EntityOf<K>>, string>
  await db.transaction('rw', table, async () => {
    const stale = await table.where('user_id').equals(userId).filter((e) => !e.pending).primaryKeys()
    await table.bulkDelete(stale)
    await cacheEntities(db, kind, userId, items)
  })
}

export async function cachedEntities<K extends EntityKind>(db: ReliefMeshDB, kind: K, userId: string): Promise<CachedEntity<EntityOf<K>>[]> {
  const table = db[kind] as unknown as Table<CachedEntity<EntityOf<K>>, string>
  return table.where('user_id').equals(userId).toArray()
}

export async function cachedEntity<K extends EntityKind>(db: ReliefMeshDB, kind: K, userId: string, id: string): Promise<CachedEntity<EntityOf<K>> | undefined> {
  const table = db[kind] as unknown as Table<CachedEntity<EntityOf<K>>, string>
  const e = await table.get(id)
  return e && e.user_id === userId ? e : undefined
}

export async function getMeta<T>(db: ReliefMeshDB, key: string): Promise<T | undefined> {
  return (await db.meta.get(key))?.value as T | undefined
}

export async function setMeta(db: ReliefMeshDB, key: string, value: unknown): Promise<void> {
  // Values often come from reactive store state; store a plain copy.
  await db.meta.put({ key, value: value === undefined ? undefined : JSON.parse(JSON.stringify(value)) })
}

export interface CachedSession {
  user: Me
  saved_at: string
}

export const metaKeys = {
  session: 'session',
  settings: 'settings',
  publicNotice: 'public_notice',
  lastPull: (userId: string) => `last_pull:${userId}`,
  lastSync: (userId: string) => `last_sync:${userId}`,
}

export async function cachedSettings(db: ReliefMeshDB): Promise<ClientSettings | undefined> {
  return getMeta<ClientSettings>(db, metaKeys.settings)
}

/** Clears cached records of all users, keeping the outbox. */
export async function clearCaches(db: ReliefMeshDB): Promise<void> {
  await db.transaction('rw', [db.requests, db.offers, db.assignments, db.meta], async () => {
    await db.requests.clear()
    await db.offers.clear()
    await db.assignments.clear()
    const keep = [metaKeys.publicNotice, metaKeys.settings]
    const keys = (await db.meta.toCollection().primaryKeys()).filter((k) => !keep.includes(k))
    await db.meta.bulkDelete(keys)
  })
}

/** Stores a local placeholder for a record created offline. */
export async function storePlaceholder<K extends 'requests' | 'offers'>(db: ReliefMeshDB, kind: K, userId: string, clientId: string, data: EntityOf<K>): Promise<void> {
  const table = db[kind] as unknown as Table<CachedEntity<EntityOf<K>>, string>
  await table.put({
    id: `local:${clientId}`,
    user_id: userId,
    client_id: clientId,
    pending: true,
    data: JSON.parse(JSON.stringify(data)),
    cached_at: new Date().toISOString(),
  })
}
