import type { Ref } from 'vue'
import { NetworkError, isApiError, isNetworkError } from '@reliefmesh/api-client'
import { type CachedEntity, type EntityKind, cacheEntities, cachedEntities, cachedEntity, getDB } from '~/utils/offline-db'

function maxCachedAt(list: { cached_at: string }[]): string | null {
  return list.reduce<string | null>((m, e) => (!m || e.cached_at > m ? e.cached_at : m), null)
}

/**
 * Loads a list from the API and falls back to the device cache when the
 * server is unreachable. Local placeholders for records created offline are
 * exposed separately so pages can show them as "waiting to sync".
 */
export function useCachedList<T extends { id: string }>(
  kind: EntityKind,
  fetcher: () => Promise<T[]>,
  filter: (item: T) => boolean = () => true,
) {
  const auth = useAuthStore()
  const sync = useSyncStore()
  const items = ref([]) as Ref<T[]>
  const pendingItems = ref([]) as Ref<CachedEntity<T>[]>
  const loading = ref(true)
  const fromCache = ref(false)
  const cachedAt = ref<string | null>(null)
  const error = ref('')

  async function loadPlaceholders() {
    const uid = auth.user?.id
    if (!uid) return
    const all = (await cachedEntities(getDB(), kind, uid)) as unknown as CachedEntity<T>[]
    pendingItems.value = all.filter((e) => e.pending)
  }

  async function load() {
    const uid = auth.user?.id
    if (!uid) return
    loading.value = true
    error.value = ''
    try {
      if (!sync.online) throw new NetworkError()
      const list = await fetcher()
      items.value = list
      fromCache.value = false
      await cacheEntities(getDB(), kind, uid, list as never)
    } catch (e) {
      if (isNetworkError(e)) {
        const all = (await cachedEntities(getDB(), kind, uid)) as unknown as CachedEntity<T>[]
        const stored = all.filter((x) => !x.pending)
        items.value = stored.map((x) => x.data).filter(filter)
        cachedAt.value = maxCachedAt(stored)
        fromCache.value = true
      } else {
        error.value = isApiError(e) ? e.message : 'Could not load data.'
      }
    } finally {
      await loadPlaceholders()
      loading.value = false
    }
  }

  watch(() => [sync.pending, sync.problems], () => void loadPlaceholders())
  return { items, pendingItems, loading, fromCache, cachedAt, error, load }
}

/** Loads a single entity, falling back to the device cache when offline. */
export function useCachedEntity<T extends { id: string }>(kind: EntityKind, id: Ref<string>, fetcher: (id: string) => Promise<T>) {
  const auth = useAuthStore()
  const sync = useSyncStore()
  const item = ref(null) as Ref<T | null>
  const pending = ref(false)
  const loading = ref(true)
  const fromCache = ref(false)
  const cachedAt = ref<string | null>(null)
  const error = ref('')
  const notFound = ref(false)

  async function fromDevice(uid: string): Promise<boolean> {
    const e = (await cachedEntity(getDB(), kind, uid, id.value)) as unknown as CachedEntity<T> | undefined
    if (!e) return false
    item.value = e.data
    pending.value = Boolean(e.pending)
    cachedAt.value = e.cached_at
    fromCache.value = true
    return true
  }

  async function load() {
    const uid = auth.user?.id
    if (!uid) return
    loading.value = true
    error.value = ''
    notFound.value = false
    try {
      if (id.value.startsWith('local:')) {
        if (!(await fromDevice(uid))) notFound.value = true
        return
      }
      if (!sync.online) {
        if (!(await fromDevice(uid))) error.value = 'This record is not saved on this device and the server cannot be reached.'
        return
      }
      const v = await fetcher(id.value)
      item.value = v
      pending.value = false
      fromCache.value = false
      await cacheEntities(getDB(), kind, uid, [v] as never)
    } catch (e) {
      if (isNetworkError(e)) {
        if (!(await fromDevice(uid))) error.value = 'This record is not saved on this device and the server cannot be reached.'
      } else if (isApiError(e) && e.status === 404) {
        notFound.value = true
      } else {
        error.value = isApiError(e) ? e.message : 'Could not load data.'
      }
    } finally {
      loading.value = false
    }
  }

  /** Replaces the current item with a fresh server version. */
  async function setFresh(v: T) {
    item.value = v
    pending.value = false
    fromCache.value = false
    const uid = auth.user?.id
    if (uid) await cacheEntities(getDB(), kind, uid, [v] as never)
  }

  return { item, pending, loading, fromCache, cachedAt, error, notFound, load, setFresh }
}
