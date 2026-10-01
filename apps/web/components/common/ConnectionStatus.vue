<script setup lang="ts">
const sync = useSyncStore()

const view = computed(() => {
  if (sync.connection === 'offline') return { icon: 'cloudOff', text: 'Offline', cls: 'bg-warning-soft text-ink border-warning' }
  if (sync.connection === 'unreachable') return { icon: 'cloudOff', text: 'Server unreachable', cls: 'bg-warning-soft text-ink border-warning' }
  if (sync.syncing) return { icon: 'refresh', text: 'Syncing...', cls: 'bg-info-soft text-ink border-info' }
  return { icon: 'cloud', text: 'Online', cls: 'bg-success-soft text-ink border-success' }
})

const announcement = computed(() => {
  const parts = [view.value.text]
  if (sync.pending) parts.push(`${sync.pending} change(s) waiting to sync`)
  if (sync.problems) parts.push(`${sync.problems} change(s) need attention`)
  return parts.join('. ')
})
</script>

<template>
  <div class="flex flex-wrap items-center gap-2" data-testid="connection-status">
    <span class="inline-flex items-center gap-1.5 rounded-full border-2 px-3 py-1 text-sm font-semibold" :class="view.cls">
      <AppIcon :name="view.icon" :size="16" />
      <span data-testid="connection-label">{{ view.text }}</span>
    </span>
    <NuxtLink
      v-if="sync.pending || sync.problems"
      to="/sync"
      class="inline-flex items-center gap-1.5 rounded-full border-2 px-3 py-1 text-sm font-semibold no-underline"
      :class="sync.problems ? 'border-danger bg-danger-soft text-ink' : 'border-info bg-info-soft text-ink'"
      data-testid="pending-link"
    >
      <AppIcon :name="sync.problems ? 'alert' : 'clock'" :size="16" />
      <span v-if="sync.pending">{{ sync.pending }} waiting</span>
      <span v-if="sync.problems">{{ sync.problems }} need attention</span>
    </NuxtLink>
    <span class="sr-only" aria-live="polite">{{ announcement }}</span>
  </div>
</template>
