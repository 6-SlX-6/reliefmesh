<script setup lang="ts">
import type { AidRequest } from '@reliefmesh/shared-types'

const props = defineProps<{ request: AidRequest; pending?: boolean }>()
const settings = useSettingsStore()
const overdue = computed(() => !['resolved', 'cancelled', 'expired', 'duplicate'].includes(props.request.status) && isPast(props.request.requested_by_time))
</script>

<template>
  <li class="card flex flex-col gap-2 hover:border-brand" data-testid="request-card">
    <div class="flex flex-wrap items-center gap-2">
      <UrgencyBadge :urgency="request.urgency" />
      <StatusBadge kind="request" :status="request.status" :pending="pending" />
      <span v-if="request.sensitive_data_flag" class="inline-flex items-center gap-1 text-sm font-semibold text-ink-muted"><AppIcon name="lock" :size="14" /> Sensitive</span>
      <span v-if="overdue" class="inline-flex items-center gap-1 text-sm font-bold text-danger"><AppIcon name="clock" :size="14" /> Overdue</span>
    </div>
    <NuxtLink :to="`/requests/${request.id}`" class="text-lg font-bold text-ink no-underline hover:underline">
      {{ request.title }}
    </NuxtLink>
    <p class="flex flex-wrap gap-x-4 gap-y-1 text-sm text-ink-muted">
      <span class="font-mono">{{ request.reference }}</span>
      <span>{{ settings.categoryLabel(request.category) }}</span>
      <span v-if="request.requested_quantity != null">{{ formatQuantity(request.requested_quantity, request.requested_unit) }}</span>
      <span v-if="request.location.area_label" class="inline-flex items-center gap-1"><AppIcon name="pin" :size="14" />{{ request.location.area_label }}</span>
      <span v-if="request.requested_by_time" class="inline-flex items-center gap-1"><AppIcon name="clock" :size="14" />Needed {{ formatRelative(request.requested_by_time) }}</span>
    </p>
  </li>
</template>
