<script setup lang="ts">
import type { Assignment } from '@reliefmesh/shared-types'

const props = defineProps<{ assignment: Assignment; showRequest?: boolean }>()
const emit = defineEmits<{ updated: [assignment: Assignment]; queued: [] }>()
const settings = useSettingsStore()
const a = computed(() => props.assignment)
</script>

<template>
  <li class="card flex flex-col gap-3" data-testid="assignment-card">
    <div class="flex flex-wrap items-center gap-2">
      <StatusBadge kind="assignment" :status="a.status" />
      <UrgencyBadge v-if="a.request" :urgency="a.request.urgency" />
    </div>
    <div v-if="showRequest && a.request">
      <NuxtLink :to="`/assignments/${a.id}`" class="text-lg font-bold text-ink no-underline hover:underline">{{ a.request.title }}</NuxtLink>
      <p class="text-sm text-ink-muted"><span class="font-mono">{{ a.request.reference }}</span> - {{ settings.categoryLabel(a.request.category) }}</p>
    </div>
    <dl class="kv text-sm">
      <div v-if="a.volunteer"><dt>Volunteer</dt><dd>{{ a.volunteer.display_name }}</dd></div>
      <div v-if="a.team_label"><dt>Team</dt><dd>{{ a.team_label }}</dd></div>
      <div v-if="a.offer"><dt>From offer</dt><dd>{{ a.offer.reference }} - {{ a.offer.title }}</dd></div>
      <div v-if="a.quantity_assigned"><dt>Quantity</dt><dd>{{ formatQuantity(a.quantity_assigned, a.unit) }}</dd></div>
      <div v-if="a.request?.location.area_label"><dt>Area</dt><dd>{{ a.request.location.area_label }}</dd></div>
      <div v-if="a.request?.requested_by_time"><dt>Needed by</dt><dd>{{ formatDateTime(a.request.requested_by_time) }}</dd></div>
      <div v-if="a.eta_text"><dt>Expected</dt><dd>{{ a.eta_text }}</dd></div>
      <div v-if="a.assigned_by"><dt>Assigned by</dt><dd>{{ a.assigned_by.display_name }}, {{ formatDateTime(a.assigned_at) }}</dd></div>
      <div v-if="a.completion_evidence_type !== 'no_evidence' && a.completed_at"><dt>Confirmation</dt><dd>{{ evidenceLabel[a.completion_evidence_type] }}</dd></div>
      <div v-if="a.cancellation_reason"><dt>Reason</dt><dd>{{ a.cancellation_reason }}</dd></div>
    </dl>
    <p v-if="a.instructions" class="whitespace-pre-line rounded-lg bg-surface-sunken p-3"><strong>Instructions:</strong> {{ a.instructions }}</p>
    <div class="flex flex-wrap items-center gap-2">
      <AssignmentActions :assignment="a" @updated="emit('updated', $event)" @queued="emit('queued')" />
      <NuxtLink v-if="showRequest" :to="`/assignments/${a.id}`" class="btn-ghost">Open <AppIcon name="chevronRight" :size="18" /></NuxtLink>
    </div>
  </li>
</template>
