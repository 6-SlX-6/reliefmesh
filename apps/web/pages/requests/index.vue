<script setup lang="ts">
import { REQUEST_STATUSES, URGENCIES, type AidRequest } from '@reliefmesh/shared-types'

definePageMeta({ requires: ['request.create', 'request.read_all', 'assignment.work_own'] })
const auth = useAuthStore()
const settings = useSettingsStore()
const route = useRoute()
const router = useRouter()
const coordinator = computed(() => auth.can('request.read_all'))
useHead({ title: coordinator.value ? 'Requests' : 'My requests' })

const filters = reactive({
  status: (route.query.status as string) ?? '',
  urgency: (route.query.urgency as string) ?? '',
  category: (route.query.category as string) ?? '',
  q: (route.query.q as string) ?? '',
  open: route.query.open === 'true',
  sort: ((route.query.sort as string) ?? 'priority') as 'priority' | 'newest' | 'updated',
})

function matches(r: AidRequest): boolean {
  if (filters.status && !filters.status.split(',').includes(r.status)) return false
  if (filters.urgency && r.urgency !== filters.urgency) return false
  if (filters.category && r.category !== filters.category) return false
  if (filters.open && ['draft', 'resolved', 'cancelled', 'expired', 'duplicate'].includes(r.status)) return false
  if (filters.q && !(`${r.reference} ${r.title} ${r.location.area_label}`.toLowerCase().includes(filters.q.toLowerCase()))) return false
  return true
}

const list = useCachedList<AidRequest>('requests', async () =>
  (await useApi().requests.list({ ...filters, open: filters.open || undefined, limit: 200 })).items, matches)

const urgencyRank = { critical: 0, high: 1, normal: 2, low: 3 }
const sorted = computed(() => {
  const items = [...list.items.value]
  if (list.fromCache.value) {
    items.sort((a, b) => filters.sort === 'priority'
      ? urgencyRank[a.urgency] - urgencyRank[b.urgency] || a.created_at.localeCompare(b.created_at)
      : b.updated_at.localeCompare(a.updated_at))
  }
  return items
})

function apply() {
  const query: Record<string, string> = {}
  for (const [k, v] of Object.entries(filters)) if (v) query[k] = String(v)
  void router.replace({ query })
  void list.load()
}
onMounted(list.load)
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader :title="coordinator ? 'Requests' : 'My requests'">
      <template #actions>
        <NuxtLink v-if="auth.can('request.create')" to="/requests/new" class="btn-primary text-lg" data-testid="new-request"><AppIcon name="plus" /> New request</NuxtLink>
      </template>
    </PageHeader>

    <form class="card grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5" role="search" @submit.prevent="apply">
      <FormField v-if="coordinator" v-slot="f" label="Search">
        <input :id="f.id" v-model="filters.q" class="field-input" type="search" placeholder="Reference, title, area">
      </FormField>
      <FormField v-slot="f" label="Status">
        <select :id="f.id" v-model="filters.status" class="field-input">
          <option value="">Any</option>
          <option value="submitted,under_review">Waiting for review</option>
          <option v-for="s in REQUEST_STATUSES" :key="s" :value="s">{{ requestStatusLabel[s] }}</option>
        </select>
      </FormField>
      <FormField v-slot="f" label="Urgency">
        <select :id="f.id" v-model="filters.urgency" class="field-input"><option value="">Any</option><option v-for="u in URGENCIES" :key="u" :value="u">{{ urgencyLabel[u] }}</option></select>
      </FormField>
      <FormField v-slot="f" label="Category">
        <select :id="f.id" v-model="filters.category" class="field-input"><option value="">Any</option><option v-for="c in settings.categories" :key="c.code" :value="c.code">{{ c.label }}</option></select>
      </FormField>
      <FormField v-slot="f" label="Sort">
        <select :id="f.id" v-model="filters.sort" class="field-input">
          <option value="priority">Urgency, then oldest</option><option value="newest">Newest first</option><option value="updated">Recently updated</option>
        </select>
      </FormField>
      <label class="flex items-center gap-2 font-normal"><input v-model="filters.open" type="checkbox" class="h-5 w-5"> Open only</label>
      <div class="flex items-end"><button type="submit" class="btn-secondary w-full">Apply filters</button></div>
    </form>

    <OfflineDataNotice :show="list.fromCache.value" :cached-at="list.cachedAt.value" />
    <AlertBox v-if="list.error.value" tone="danger">{{ list.error.value }}</AlertBox>

    <ul v-if="list.pendingItems.value.length" class="flex flex-col gap-3" aria-label="Waiting to sync">
      <RequestCard v-for="p in list.pendingItems.value" :key="p.id" :request="p.data" pending />
    </ul>
    <LoadingState v-if="list.loading.value && !sorted.length" />
    <EmptyState v-else-if="!sorted.length && !list.pendingItems.value.length" title="No requests found">
      <span v-if="auth.can('request.create')">Use "New request" to ask for help.</span>
    </EmptyState>
    <ul v-else class="flex flex-col gap-3" data-testid="request-list">
      <RequestCard v-for="r in sorted" :key="r.id" :request="r" />
    </ul>
  </div>
</template>
