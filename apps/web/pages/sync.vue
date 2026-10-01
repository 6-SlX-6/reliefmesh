<script setup lang="ts">
import type { OutboxEntry } from '~/utils/offline-db'

useHead({ title: 'Pending changes' })
const sync = useSyncStore()
const toasts = useToastStore()
const entries = ref<OutboxEntry[]>([])
const foreign = ref<OutboxEntry[]>([])
const discardTarget = ref<OutboxEntry | null>(null)
const discardForeignOpen = ref(false)

async function load() {
  entries.value = await sync.entries()
  foreign.value = await sync.foreignEntries()
}
onMounted(load)
watch(() => [sync.pending, sync.problems, sync.foreignPending], load)

async function copy(e: OutboxEntry) {
  try {
    await navigator.clipboard.writeText(JSON.stringify({ type: e.type, created_at: e.created_at, payload: e.payload }, null, 2))
    toasts.success('Copied to the clipboard.')
  } catch {
    toasts.error('Copying is not available on this device.')
  }
}
async function confirmDiscard() {
  if (!discardTarget.value) return
  const removed = await sync.discard(discardTarget.value.op_id)
  toasts.info(`${removed.length} change(s) discarded.`)
  discardTarget.value = null
  await load()
}
async function confirmDiscardForeign() {
  await sync.discardForeign()
  discardForeignOpen.value = false
  await load()
}
const stateLabel = { pending: 'Waiting to sync', conflict: 'Conflict', rejected: 'Rejected by server' }
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="Pending changes">
      <template #actions>
        <button type="button" class="btn-primary" :disabled="!sync.online || sync.syncing || !sync.pending" data-testid="sync-now" @click="sync.flushNow()">
          <AppIcon name="refresh" :size="18" /> {{ sync.syncing ? 'Syncing...' : 'Sync now' }}
        </button>
      </template>
    </PageHeader>
    <AlertBox tone="info">
      Changes made without a connection are stored on this device and sent automatically in the order they were made.
      Nothing is discarded unless you choose to. <span v-if="sync.lastSyncAt">Last successful sync: {{ formatRelative(sync.lastSyncAt) }}.</span>
    </AlertBox>
    <AlertBox v-if="sync.authRequired" tone="warning">Your session has ended. Sign in again to send these changes.</AlertBox>
    <EmptyState v-if="!entries.length" title="Everything is synchronized" icon="check" data-testid="sync-empty">There are no changes waiting on this device.</EmptyState>
    <ul v-else class="flex flex-col gap-3" data-testid="outbox-list">
      <li v-for="e in entries" :key="e.op_id" class="card flex flex-col gap-2" :class="e.state !== 'pending' ? 'border-2 border-danger' : ''" data-testid="outbox-entry">
        <div class="flex flex-wrap items-center gap-2">
          <span class="rounded-md border-2 px-2 py-0.5 text-sm font-semibold" :class="e.state === 'pending' ? 'border-info bg-info-soft' : 'border-danger bg-danger-soft'">{{ stateLabel[e.state] }}</span>
          <span class="text-sm text-ink-muted">Created {{ formatDateTime(e.created_at) }}<span v-if="e.attempts"> - {{ e.attempts }} attempt(s)</span></span>
        </div>
        <p class="font-semibold">{{ e.summary }}</p>
        <AlertBox v-if="e.last_error && e.state !== 'pending'" tone="danger">
          {{ e.last_error.message }}
          <ul v-if="e.last_error.fields" class="mt-1 list-disc pl-5 text-sm"><li v-for="(m, f) in e.last_error.fields" :key="f">{{ f }}: {{ m }}</li></ul>
        </AlertBox>
        <p v-if="e.state === 'conflict'" class="text-sm">The record was changed on the server in the meantime. The current server version has been loaded. You can try again or discard your change.</p>
        <div class="flex flex-wrap gap-2">
          <button v-if="e.state !== 'pending'" type="button" class="btn-secondary btn-sm" :disabled="!sync.online" @click="sync.retry(e.op_id)">Try again</button>
          <button type="button" class="btn-ghost btn-sm" @click="copy(e)">Copy details</button>
          <button type="button" class="btn-ghost btn-sm text-danger" data-testid="discard" @click="discardTarget = e">Discard...</button>
        </div>
      </li>
    </ul>
    <section v-if="foreign.length" class="card flex flex-col gap-2 border-2 border-warning">
      <h2>Changes from another account on this device</h2>
      <p>{{ foreign.length }} change(s) were made by a different account and have not been sent. Sign in with that account to send them.</p>
      <button type="button" class="btn-ghost w-fit text-danger" @click="discardForeignOpen = true">Discard them...</button>
    </section>
    <ConfirmDialog :open="discardTarget !== null" title="Discard this change?" confirm-label="Discard permanently" danger @confirm="confirmDiscard" @cancel="discardTarget = null">
      <p>"{{ discardTarget?.summary }}" will be removed from this device and never sent. This cannot be undone. Changes that depend on it are discarded too.</p>
    </ConfirmDialog>
    <ConfirmDialog :open="discardForeignOpen" title="Discard changes of another account?" confirm-label="Discard permanently" danger @confirm="confirmDiscardForeign" @cancel="discardForeignOpen = false">
      <p>These changes will never be sent. This cannot be undone.</p>
    </ConfirmDialog>
  </div>
</template>
