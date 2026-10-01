<script setup lang="ts">
import type { AidRequest, Assignment } from '@reliefmesh/shared-types'

const route = useRoute()
const auth = useAuthStore()
const id = computed(() => String(route.params.id))
const entity = useCachedEntity<AidRequest>('requests', id, (rid) => useApi().requests.get(rid))
const assignments = ref<Assignment[]>([])
const timeline = ref<{ load: () => Promise<void> } | null>(null)
useHead(() => ({ title: entity.item.value?.reference ?? 'Request' }))

async function loadAssignments() {
  const r = entity.item.value
  if (!r || r.id.startsWith('local:') || !auth.can('assignment.manage') || !useSyncStore().online) return
  try {
    assignments.value = (await useApi().assignments.list({ request_id: r.id })).items
  } catch {
    assignments.value = []
  }
}

async function refresh(updated?: AidRequest) {
  if (updated) await entity.setFresh(updated)
  else await entity.load()
  await loadAssignments()
  await timeline.value?.load()
}

onMounted(async () => {
  await entity.load()
  await loadAssignments()
})
watch(id, () => void refresh())
</script>

<template>
  <div class="flex flex-col gap-4">
    <LoadingState v-if="entity.loading.value && !entity.item.value" />
    <template v-else-if="entity.notFound.value">
      <PageHeader title="Request not found" back="/requests" back-label="Back to requests" />
      <AlertBox tone="warning">This request does not exist or you are not allowed to see it.</AlertBox>
    </template>
    <AlertBox v-else-if="entity.error.value" tone="danger">{{ entity.error.value }}</AlertBox>
    <template v-else-if="entity.item.value">
      <PageHeader :title="entity.item.value.title" back="/requests" back-label="Back to requests">
        <template #meta>
          <UrgencyBadge :urgency="entity.item.value.urgency" />
          <StatusBadge kind="request" :status="entity.item.value.status" :pending="entity.pending.value" />
          <span class="font-mono text-ink-muted" data-testid="request-reference">{{ entity.item.value.reference }}</span>
          <span v-if="entity.item.value.sensitive_data_flag" class="inline-flex items-center gap-1 text-sm font-semibold"><AppIcon name="lock" :size="14" /> Sensitive</span>
        </template>
      </PageHeader>
      <OfflineDataNotice :show="entity.fromCache.value && !entity.pending.value" :cached-at="entity.cachedAt.value" />
      <PendingForEntity :entity-id="entity.item.value.id.startsWith('local:') ? undefined : entity.item.value.id" :client-id="entity.item.value.client_id" />
      <AlertBox v-if="entity.pending.value" tone="info" title="Waiting to sync">
        This request is saved on this device and will be sent automatically when the server is reachable. It gets its reference number then.
      </AlertBox>
      <AlertBox v-if="entity.item.value.redacted" tone="info">Personal details of this request were removed under the data retention policy.</AlertBox>
      <AlertBox tone="info" class="!border-l-4">
        <strong>{{ requestStatusLabel[entity.item.value.status] }}:</strong> {{ requestStatusHelp[entity.item.value.status] }}
      </AlertBox>

      <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <div class="flex flex-col gap-4 xl:col-span-2">
          <RequestStatusActions :request="entity.item.value" @updated="refresh" @queued="refresh()" />
          <RequestFacts :request="entity.item.value" />
          <ProtectedDetails kind="request" :id="entity.item.value.id" :can-reveal="entity.item.value.permissions.can_reveal_protected" />
          <section v-if="auth.can('assignment.manage') && !entity.pending.value" class="card flex flex-col gap-3" aria-labelledby="asg-h">
            <h2 id="asg-h">Assignments</h2>
            <AssignDialog v-if="entity.item.value.permissions.can_assign" :request="entity.item.value" @created="refresh()" />
            <p v-else class="text-ink-muted">Verify the request before assigning help.</p>
            <ul v-if="assignments.length" class="flex flex-col gap-3">
              <AssignmentCard v-for="a in assignments" :key="a.id" :assignment="a" @updated="refresh()" @queued="refresh()" />
            </ul>
          </section>
          <RequestEditPanel :request="entity.item.value" @updated="refresh" @queued="refresh()" />
        </div>
        <div class="flex flex-col gap-4">
          <NotesPanel kind="request" :id="entity.item.value.id" :client-id="entity.item.value.client_id"
            :visibilities="entity.pending.value ? ['shared'] : entity.item.value.permissions.note_visibilities"
            :sensitive-context="entity.item.value.category === 'medicine_pickup'" />
          <EventTimeline ref="timeline" :request-id="entity.item.value.id" />
        </div>
      </div>
    </template>
  </div>
</template>
