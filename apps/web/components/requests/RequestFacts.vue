<script setup lang="ts">
import type { AidRequest } from '@reliefmesh/shared-types'

const props = defineProps<{ request: AidRequest }>()
const settings = useSettingsStore()
const r = computed(() => props.request)
const approx = computed(() => r.value.location.approx_lat != null ? `${r.value.location.approx_lat}, ${r.value.location.approx_lon} (rounded)` : '')
</script>

<template>
  <section class="card" aria-labelledby="facts-h">
    <h2 id="facts-h" class="mb-3">Details</h2>
    <p v-if="r.details_redacted" class="mb-3 text-sm text-ink-muted">Sensitive details are hidden in lists. Open the request online to see them.</p>
    <p v-if="r.description" class="mb-4 whitespace-pre-line">{{ r.description }}</p>
    <dl class="kv">
      <div><dt>Category</dt><dd>{{ settings.categoryLabel(r.category) }}</dd></div>
      <div v-if="r.requested_quantity != null"><dt>Quantity</dt><dd>{{ formatQuantity(r.requested_quantity, r.requested_unit) }}</dd></div>
      <div v-if="r.estimated_people_affected != null"><dt>People affected</dt><dd>{{ r.estimated_people_affected }}</dd></div>
      <div v-if="r.requested_by_time"><dt>Needed by</dt><dd>{{ formatDateTime(r.requested_by_time) }} ({{ formatRelative(r.requested_by_time) }})</dd></div>
      <div><dt>Location</dt><dd>
        {{ locationModeLabel[r.location.mode] }}<span v-if="r.location.area_label">: {{ r.location.area_label }}</span>
        <span v-if="approx" class="block text-sm text-ink-muted">{{ approx }}</span>
        <span v-if="r.location.has_exact" class="mt-1 flex items-center gap-1 text-sm"><AppIcon name="lock" :size="14" /> Exact location stored (protected)</span>
      </dd></div>
      <div><dt>Contact</dt><dd>
        {{ contactVisibilityLabel[r.contact_visibility] }}<span v-if="r.contact_method !== 'none'"> - {{ contactMethodLabel[r.contact_method] }}</span>
        <span v-if="r.has_contact_details" class="mt-1 flex items-center gap-1 text-sm"><AppIcon name="lock" :size="14" /> Contact details stored (protected)</span>
      </dd></div>
      <div v-if="r.accessibility_notes"><dt>Accessibility needs</dt><dd class="whitespace-pre-line">{{ r.accessibility_notes }}</dd></div>
      <div v-if="r.requires_formal_authorization"><dt>Formal authorization</dt><dd>Required for pickup</dd></div>
      <div v-if="r.review_status"><dt>Review</dt><dd>{{ reviewStatusLabel[r.review_status] }}</dd></div>
      <div v-if="r.verification_level"><dt>Verification</dt><dd>{{ verificationLabel[r.verification_level] }}</dd></div>
      <div v-if="r.assigned_team"><dt>Responsible team</dt><dd>{{ r.assigned_team }}</dd></div>
      <div v-if="r.assigned_user"><dt>Lead responder</dt><dd>{{ r.assigned_user.display_name }}</dd></div>
      <div v-if="r.responders_assigned !== undefined"><dt>Help assigned</dt><dd>{{ r.responders_assigned ? 'Yes' : 'Not yet' }}</dd></div>
      <div v-if="r.created_by"><dt>Created by</dt><dd>{{ r.created_by.display_name }}</dd></div>
      <div><dt>Created</dt><dd>{{ formatDateTime(r.created_at) }}</dd></div>
      <div v-if="r.expiry_at"><dt>Review by (expiry)</dt><dd>{{ formatDateTime(r.expiry_at) }}</dd></div>
      <div v-if="r.tags?.length"><dt>Tags</dt><dd>{{ r.tags.join(', ') }}</dd></div>
      <div v-if="r.duplicate_of"><dt>Duplicate of</dt><dd><NuxtLink :to="`/requests/${r.duplicate_of.id}`">{{ r.duplicate_of.reference }}</NuxtLink></dd></div>
      <div v-if="r.resolution_summary" class="sm:col-span-2"><dt>Resolution</dt><dd class="whitespace-pre-line">{{ r.resolution_summary }}</dd></div>
    </dl>
  </section>
</template>
