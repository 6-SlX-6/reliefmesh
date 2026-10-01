<script setup lang="ts">
import { assignmentStatusTone, offerStatusTone, requestStatusTone, tone as tones } from '@reliefmesh/ui-tokens'

const props = defineProps<{ kind: 'request' | 'offer' | 'assignment'; status: string; pending?: boolean }>()

const icons: Record<string, string> = {
  neutral: 'circle', info: 'info', progress: 'activity', success: 'check', warning: 'alert', danger: 'x',
}

const info = computed(() => {
  const map = props.kind === 'request' ? requestStatusTone : props.kind === 'offer' ? offerStatusTone : assignmentStatusTone
  const t = map[props.status] ?? 'neutral'
  const label = props.kind === 'request'
    ? (requestStatusLabel as Record<string, string>)[props.status]
    : props.kind === 'offer'
      ? (offerStatusLabel as Record<string, string>)[props.status]
      : (assignmentStatusLabel as Record<string, string>)[props.status]
  return { tone: tones[t], icon: icons[t] ?? 'circle', label: label ?? props.status }
})
</script>

<template>
  <span class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-md border-2 px-2 py-0.5 text-sm font-semibold" :style="{ background: info.tone.bg, color: info.tone.fg, borderColor: info.tone.border }" data-testid="status-badge">
    <AppIcon :name="info.icon" :size="14" />
    <span><span class="sr-only">Status: </span>{{ info.label }}</span>
  </span>
  <span v-if="pending" class="ml-1 inline-flex items-center gap-1 whitespace-nowrap rounded-md border-2 border-dashed border-info px-2 py-0.5 text-sm font-semibold text-ink">
    <AppIcon name="clock" :size="14" /> Waiting to sync
  </span>
</template>
