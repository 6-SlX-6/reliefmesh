<script setup lang="ts">
import type { Assignment, HandoverStatus } from '@reliefmesh/shared-types'

const route = useRoute()
const settings = useSettingsStore()
const sync = useSyncStore()
const toasts = useToastStore()
const id = computed(() => String(route.params.id))
const entity = useCachedEntity<Assignment>('assignments', id, (aid) => useApi().assignments.get(aid))
useHead({ title: 'Assignment' })
onMounted(entity.load)

const eta = ref('')
const handover = ref<HandoverStatus>('not_started')
const handoverNotes = ref('')
watch(() => entity.item.value, (a) => {
  if (a) {
    eta.value = a.eta_text
    handover.value = a.handover_status
    handoverNotes.value = a.handover_notes
  }
}, { immediate: true })

async function saveHandover() {
  const a = entity.item.value
  if (!a) return
  const res = await sync.submit({ type: 'assignment.update', entity_id: a.id,
    payload: { eta_text: eta.value, handover_status: handover.value, handover_notes: handoverNotes.value, version: a.version },
    summary: `Update task ${a.request?.reference ?? ''}` }, { dropOnReject: true })
  if (res.status === 'applied') {
    await entity.setFresh(res.entity as Assignment)
    toasts.success('Saved.')
  } else if (res.status === 'queued') toasts.info('Saved on this device. It will be sent when the connection returns.')
  else toasts.error(res.error?.message ?? 'Could not save.')
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <LoadingState v-if="entity.loading.value && !entity.item.value" />
    <template v-else-if="entity.notFound.value">
      <PageHeader title="Assignment not found" back="/assignments" />
      <AlertBox tone="warning">This assignment does not exist or is not assigned to you.</AlertBox>
    </template>
    <AlertBox v-else-if="entity.error.value" tone="danger">{{ entity.error.value }}</AlertBox>
    <template v-else-if="entity.item.value">
      <PageHeader :title="entity.item.value.request?.title ?? 'Assignment'" back="/assignments" back-label="Back to tasks">
        <template #meta>
          <StatusBadge kind="assignment" :status="entity.item.value.status" />
          <UrgencyBadge v-if="entity.item.value.request" :urgency="entity.item.value.request.urgency" />
          <span class="font-mono text-ink-muted">{{ entity.item.value.request?.reference }}</span>
        </template>
      </PageHeader>
      <OfflineDataNotice :show="entity.fromCache.value" :cached-at="entity.cachedAt.value" />
      <PendingForEntity :entity-id="entity.item.value.id" />
      <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <div class="flex flex-col gap-4 xl:col-span-2">
          <section class="card flex flex-col gap-3">
            <h2>What to do</h2>
            <p v-if="entity.item.value.instructions" class="whitespace-pre-line text-lg">{{ entity.item.value.instructions }}</p>
            <p v-else class="text-ink-muted">No instructions were added.</p>
            <dl class="kv">
              <div v-if="entity.item.value.request"><dt>Need</dt><dd>{{ settings.categoryLabel(entity.item.value.request.category) }}</dd></div>
              <div v-if="entity.item.value.quantity_assigned"><dt>Quantity</dt><dd class="font-semibold">{{ formatQuantity(entity.item.value.quantity_assigned, entity.item.value.unit) }}</dd></div>
              <div v-if="entity.item.value.offer"><dt>Pick up from</dt><dd>{{ entity.item.value.offer.title }} ({{ deliveryModeLabel[entity.item.value.offer.pickup_or_delivery_mode] }})<span v-if="entity.item.value.offer.location.area_label"> - {{ entity.item.value.offer.location.area_label }}</span></dd></div>
              <div v-if="entity.item.value.request?.location.area_label"><dt>Destination area</dt><dd>{{ entity.item.value.request.location.area_label }}</dd></div>
              <div v-if="entity.item.value.request?.requested_by_time"><dt>Needed by</dt><dd>{{ formatDateTime(entity.item.value.request.requested_by_time) }}</dd></div>
              <div v-if="entity.item.value.request?.accessibility_notes"><dt>Accessibility needs</dt><dd>{{ entity.item.value.request.accessibility_notes }}</dd></div>
              <div v-if="entity.item.value.request?.requires_formal_authorization"><dt>Authorization</dt><dd>Formal authorization is required for this pickup. Check with the coordinator.</dd></div>
              <div v-if="entity.item.value.offer?.restrictions"><dt>Restrictions</dt><dd>{{ entity.item.value.offer.restrictions }}</dd></div>
            </dl>
            <AssignmentActions :assignment="entity.item.value" @updated="entity.setFresh" @queued="entity.load()" />
          </section>
          <ProtectedDetails kind="assignment" :id="entity.item.value.id" :can-reveal="entity.item.value.permissions.can_reveal_protected" />
          <section v-if="entity.item.value.permissions.can_update_handover" class="card flex flex-col gap-3">
            <h2>Progress and handover</h2>
            <FormField v-slot="f" label="Expected time"><input :id="f.id" v-model="eta" class="field-input" maxlength="120"></FormField>
            <FormField v-slot="f" label="Handover">
              <select :id="f.id" v-model="handover" class="field-input">
                <option v-for="(label, key) in handoverLabel" :key="key" :value="key">{{ label }}</option>
              </select>
            </FormField>
            <FormField v-slot="f" label="Handover notes" help="E.g. 'left with shelter desk'. No personal or health details.">
              <textarea :id="f.id" v-model="handoverNotes" class="field-input" rows="2" maxlength="2000" />
            </FormField>
            <button type="button" class="btn-secondary w-fit" @click="saveHandover">Save progress</button>
          </section>
        </div>
        <NotesPanel v-if="entity.item.value.request" kind="request" :id="entity.item.value.request.id" :visibilities="['responders']" />
      </div>
    </template>
  </div>
</template>
