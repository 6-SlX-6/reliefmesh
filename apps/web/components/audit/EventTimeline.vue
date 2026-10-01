<script setup lang="ts">
import type { TimelineEvent } from '@reliefmesh/shared-types'

const props = defineProps<{ requestId: string }>()
const sync = useSyncStore()
const events = ref<TimelineEvent[]>([])
const error = ref('')
const loading = ref(false)

async function load() {
  if (props.requestId.startsWith('local:')) return
  if (!sync.online) {
    error.value = 'The history is only available while connected.'
    return
  }
  loading.value = true
  error.value = ''
  try {
    events.value = (await useApi().requests.events(props.requestId)).items
  } catch {
    error.value = 'The history could not be loaded.'
  } finally {
    loading.value = false
  }
}

function describe(e: TimelineEvent): string {
  if (e.from_status || e.to_status) {
    const from = statusLabelAny(e.from_status)
    const to = statusLabelAny(e.to_status)
    return from ? `${from} -> ${to}` : to
  }
  if (e.action.endsWith('urgency_changed')) return `${e.metadata.from} -> ${e.metadata.to}`
  if (e.action.endsWith('protected_viewed')) return `Opened: ${(e.metadata.fields as string[] | undefined)?.join(', ') ?? ''}`
  if (e.action.endsWith('.updated') && Array.isArray(e.metadata.changed_fields)) return `Changed: ${(e.metadata.changed_fields as string[]).join(', ')}`
  return ''
}

defineExpose({ load })
onMounted(load)
watch(() => sync.online, (o) => { if (o) void load() })
</script>

<template>
  <section class="card" aria-labelledby="tl-h">
    <h2 id="tl-h" class="mb-1 flex items-center gap-2"><AppIcon name="clock" /> History</h2>
    <p class="mb-3 text-sm text-ink-muted">Every change is recorded and cannot be edited or deleted.</p>
    <LoadingState v-if="loading" />
    <p v-else-if="error" class="text-ink-muted">{{ error }}</p>
    <ol v-else class="relative flex flex-col gap-3 border-l-4 border-surface-border pl-4" data-testid="timeline">
      <li v-for="e in events" :key="e.id" class="relative">
        <span class="absolute -left-[1.6rem] top-1.5 h-3 w-3 rounded-full border-2 border-surface bg-brand" aria-hidden="true" />
        <p class="font-semibold">{{ actionLabel(e.action) }} <span v-if="describe(e)" class="font-normal">- {{ describe(e) }}</span></p>
        <p class="text-sm text-ink-muted">
          {{ formatDateTime(e.occurred_at) }} - {{ e.actor.display_name || e.actor.label }}
          <span v-if="e.metadata.cause"> (automatic consequence: {{ String(e.metadata.cause).replaceAll('_', ' ') }})</span>
          <span v-if="e.metadata.created_offline"> (created offline)</span>
        </p>
        <p v-if="e.reason" class="text-sm">Reason: {{ e.reason }}</p>
      </li>
    </ol>
  </section>
</template>
