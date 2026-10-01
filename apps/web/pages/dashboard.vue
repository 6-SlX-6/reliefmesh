<script setup lang="ts">
import type { Dashboard } from '@reliefmesh/shared-types'

definePageMeta({ requires: 'dashboard.view' })
useHead({ title: 'Dashboard' })
const settings = useSettingsStore()
const sync = useSyncStore()
const data = ref<Dashboard | null>(null)
const error = ref('')
const loading = ref(true)

async function load() {
  loading.value = true
  error.value = ''
  try {
    data.value = await useApi().dashboard()
  } catch {
    error.value = sync.online ? 'The dashboard could not be loaded.' : 'The dashboard needs a connection to the server. Lists of saved requests remain available offline.'
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(() => sync.online, (o) => { if (o) void load() })
const categories = computed(() => Object.entries(data.value?.requests.open_by_category ?? {}).sort((a, b) => b[1] - a[1]))
</script>

<template>
  <div class="flex flex-col gap-5">
    <PageHeader title="Operational overview">
      <template #meta><span v-if="data" class="text-sm text-ink-muted">Updated {{ formatRelative(data.generated_at) }}</span></template>
      <template #actions>
        <button type="button" class="btn-secondary" :disabled="loading" @click="load"><AppIcon name="refresh" :size="18" /> Refresh</button>
        <NuxtLink to="/requests/new" class="btn-primary"><AppIcon name="plus" :size="18" /> New request</NuxtLink>
      </template>
    </PageHeader>
    <LoadingState v-if="loading && !data" />
    <AlertBox v-else-if="error" tone="warning">{{ error }}</AlertBox>
    <template v-if="data">
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard label="Need review" :value="data.requests.needs_review" tone="info" to="/requests?status=submitted,under_review" />
        <StatCard label="Critical / high open" :value="(data.requests.open_by_urgency.critical ?? 0) + (data.requests.open_by_urgency.high ?? 0)" tone="danger" to="/requests?open=true&sort=priority" />
        <StatCard label="Overdue" :value="data.requests.overdue" tone="warning" help="Past the 'needed by' time" />
        <StatCard label="Delivered, awaiting confirmation" :value="data.requests.awaiting_confirmation" tone="success" help="Confirm whether the need is actually met" />
        <StatCard label="Open requests" :value="data.requests.open" to="/requests?open=true" />
        <StatCard label="Past review date" :value="data.requests.past_expiry" tone="warning" help="Review: still needed?" />
        <StatCard label="Active assignments" :value="data.assignments.active" to="/assignments" />
        <StatCard label="Volunteers available" :value="data.volunteers.available" :help="`${data.volunteers.limited} limited, ${data.volunteers.unavailable} unavailable`" :to="data.queues ? '/volunteers' : undefined" />
      </div>
      <div v-if="data.queues" class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <QueueList title="Waiting for review" :items="data.queues.needs_review" empty="Nothing waiting for review." link="/requests?status=submitted,under_review" />
        <QueueList title="Critical and high urgency" :items="data.queues.urgent_open" empty="No open critical or high-urgency requests." />
        <QueueList title="Delivered - confirm resolution" :items="data.queues.awaiting_confirmation" empty="Nothing to confirm." />
        <QueueList title="Overdue" :items="data.queues.overdue" empty="Nothing overdue." />
      </div>
      <AlertBox v-else tone="info">As an administrator you see aggregate numbers only. Request details require a coordinator role.</AlertBox>
      <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <section class="card">
          <h2 class="mb-3">Open requests by category</h2>
          <p v-if="!categories.length" class="text-ink-muted">No open requests.</p>
          <table v-else class="w-full text-left">
            <thead><tr><th class="py-1">Category</th><th class="py-1 text-right">Open</th></tr></thead>
            <tbody>
              <tr v-for="[cat, n] in categories" :key="cat" class="border-t border-surface-border"><td class="py-1.5">{{ settings.categoryLabel(cat) }}</td><td class="py-1.5 text-right font-semibold tabular-nums">{{ n }}</td></tr>
            </tbody>
          </table>
        </section>
        <section class="card">
          <h2 class="mb-3">Offers with remaining quantity</h2>
          <p v-if="!Object.keys(data.offers.remaining_by_category).length" class="text-ink-muted">No available offers.</p>
          <table v-else class="w-full text-left">
            <thead><tr><th class="py-1">Category</th><th class="py-1 text-right">Remaining units</th></tr></thead>
            <tbody>
              <tr v-for="(n, cat) in data.offers.remaining_by_category" :key="cat" class="border-t border-surface-border"><td class="py-1.5">{{ settings.categoryLabel(String(cat)) }}</td><td class="py-1.5 text-right font-semibold tabular-nums">{{ n }}</td></tr>
            </tbody>
          </table>
          <p class="mt-3 text-sm text-ink-muted">Allocation is always decided manually by coordinators.</p>
        </section>
      </div>
    </template>
  </div>
</template>
