<script setup lang="ts">
import type { QueueItem } from '@reliefmesh/shared-types'

defineProps<{ title: string; items: QueueItem[]; empty: string; link?: string }>()
const settings = useSettingsStore()
</script>

<template>
  <section class="card" :aria-label="title">
    <div class="mb-3 flex items-center justify-between gap-2">
      <h2>{{ title }}</h2>
      <NuxtLink v-if="link" :to="link" class="text-sm font-semibold">Show all</NuxtLink>
    </div>
    <p v-if="!items.length" class="text-ink-muted">{{ empty }}</p>
    <ul v-else class="flex flex-col divide-y divide-surface-border">
      <li v-for="q in items" :key="q.id" class="flex flex-col gap-1 py-2">
        <div class="flex flex-wrap items-center gap-2">
          <UrgencyBadge :urgency="q.urgency" />
          <StatusBadge kind="request" :status="q.status" />
        </div>
        <NuxtLink :to="`/requests/${q.id}`" class="font-semibold text-ink no-underline hover:underline">{{ q.title }}</NuxtLink>
        <p class="text-sm text-ink-muted">
          <span class="font-mono">{{ q.reference }}</span> - {{ settings.categoryLabel(q.category) }}
          <span v-if="q.area_label"> - {{ q.area_label }}</span>
          <span v-if="q.requested_by_time"> - needed {{ formatRelative(q.requested_by_time) }}</span>
        </p>
      </li>
    </ul>
  </section>
</template>
