<script setup lang="ts">
import type { Offer } from '@reliefmesh/shared-types'

const route = useRoute()
const settings = useSettingsStore()
const id = computed(() => String(route.params.id))
const entity = useCachedEntity<Offer>('offers', id, (oid) => useApi().offers.get(oid))
useHead(() => ({ title: entity.item.value?.reference ?? 'Offer' }))
onMounted(entity.load)
async function refresh(o?: Offer) {
  if (o) await entity.setFresh(o)
  else await entity.load()
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <LoadingState v-if="entity.loading.value && !entity.item.value" />
    <template v-else-if="entity.notFound.value">
      <PageHeader title="Offer not found" back="/offers" />
      <AlertBox tone="warning">This offer does not exist or you are not allowed to see it.</AlertBox>
    </template>
    <AlertBox v-else-if="entity.error.value" tone="danger">{{ entity.error.value }}</AlertBox>
    <template v-else-if="entity.item.value">
      <PageHeader :title="entity.item.value.title" back="/offers" back-label="Back to offers">
        <template #meta>
          <StatusBadge kind="offer" :status="entity.item.value.status" :pending="entity.pending.value" />
          <span class="font-mono text-ink-muted">{{ entity.item.value.reference }}</span>
        </template>
      </PageHeader>
      <OfflineDataNotice :show="entity.fromCache.value && !entity.pending.value" :cached-at="entity.cachedAt.value" />
      <PendingForEntity :entity-id="entity.item.value.id.startsWith('local:') ? undefined : entity.item.value.id" :client-id="entity.item.value.client_id" />
      <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <div class="flex flex-col gap-4 xl:col-span-2">
          <section class="card flex flex-col gap-4">
            <div class="grid grid-cols-3 gap-3 text-center">
              <div class="rounded-lg bg-surface-sunken p-3"><p class="text-2xl font-bold tabular-nums">{{ entity.item.value.quantity_available }}</p><p class="text-sm">Total {{ entity.item.value.unit }}</p></div>
              <div class="rounded-lg bg-surface-sunken p-3"><p class="text-2xl font-bold tabular-nums">{{ entity.item.value.assigned_quantity }}</p><p class="text-sm">Allocated</p></div>
              <div class="rounded-lg bg-success-soft p-3"><p class="text-2xl font-bold tabular-nums" data-testid="offer-remaining">{{ entity.item.value.remaining_quantity }}</p><p class="text-sm">Remaining</p></div>
            </div>
            <OfferStatusActions :offer="entity.item.value" @updated="refresh" @queued="refresh()" />
          </section>
          <section class="card">
            <h2 class="mb-3">Details</h2>
            <p v-if="entity.item.value.description" class="mb-4 whitespace-pre-line">{{ entity.item.value.description }}</p>
            <dl class="kv">
              <div><dt>Category</dt><dd>{{ settings.categoryLabel(entity.item.value.category) }}</dd></div>
              <div><dt>Pickup / delivery</dt><dd>{{ deliveryModeLabel[entity.item.value.pickup_or_delivery_mode] }}</dd></div>
              <div v-if="entity.item.value.availability_start"><dt>Available from</dt><dd>{{ formatDateTime(entity.item.value.availability_start) }}</dd></div>
              <div v-if="entity.item.value.availability_end"><dt>Available until</dt><dd>{{ formatDateTime(entity.item.value.availability_end) }}</dd></div>
              <div><dt>Location</dt><dd>{{ locationModeLabel[entity.item.value.location.mode] }}<span v-if="entity.item.value.location.area_label">: {{ entity.item.value.location.area_label }}</span></dd></div>
              <div v-if="entity.item.value.restrictions"><dt>Restrictions</dt><dd>{{ entity.item.value.restrictions }}</dd></div>
              <div v-if="entity.item.value.accessibility_notes"><dt>Accessibility</dt><dd>{{ entity.item.value.accessibility_notes }}</dd></div>
              <div v-if="entity.item.value.verification_level"><dt>Verification</dt><dd>{{ verificationLabel[entity.item.value.verification_level] }}</dd></div>
              <div v-if="entity.item.value.created_by"><dt>Offered by</dt><dd>{{ entity.item.value.created_by.display_name }}</dd></div>
              <div><dt>Created</dt><dd>{{ formatDateTime(entity.item.value.created_at) }}</dd></div>
            </dl>
          </section>
          <ProtectedDetails kind="offer" :id="entity.item.value.id" :can-reveal="entity.item.value.permissions.can_reveal_protected" />
        </div>
        <NotesPanel kind="offer" :id="entity.item.value.id" :client-id="entity.item.value.client_id" :visibilities="entity.pending.value ? ['shared'] : entity.item.value.permissions.note_visibilities" />
      </div>
    </template>
  </div>
</template>
