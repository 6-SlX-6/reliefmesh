<script setup lang="ts">
import type { OutboxEntry } from '~/utils/offline-db'

// Lists outbox entries that concern one record, so people can see that a
// change they made offline has not reached the server yet.
const props = defineProps<{ entityId?: string; clientId?: string }>()
const sync = useSyncStore()
const entries = ref<OutboxEntry[]>([])

async function load() {
  const all = await sync.entries()
  entries.value = all.filter((e) =>
    (props.entityId && e.entity_id === props.entityId) ||
    (props.clientId && (e.entity_client_id === props.clientId || (e.payload as { client_id?: string })?.client_id === props.clientId)))
}
onMounted(load)
watch(() => [sync.pending, sync.problems], load)
</script>

<template>
  <AlertBox v-if="entries.length" :tone="entries.some((e) => e.state !== 'pending') ? 'danger' : 'info'" title="Changes not yet on the server" data-testid="pending-for-entity">
    <ul class="list-disc pl-5">
      <li v-for="e in entries" :key="e.op_id">
        {{ e.summary }} -
        <strong>{{ e.state === 'pending' ? 'waiting to sync' : e.state === 'conflict' ? 'conflict' : 'rejected' }}</strong>
        <span v-if="e.last_error">: {{ e.last_error.message }}</span>
      </li>
    </ul>
    <NuxtLink v-if="entries.some((e) => e.state !== 'pending')" to="/sync" class="mt-2 inline-block font-semibold">Resolve in "Pending changes"</NuxtLink>
  </AlertBox>
</template>
