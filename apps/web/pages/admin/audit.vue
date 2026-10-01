<script setup lang="ts">
import type { AuditEvent, AuditVerifyResult } from '@reliefmesh/shared-types'

definePageMeta({ layout: 'admin', requires: 'audit.view_system' })
useHead({ title: 'Audit log' })
const events = ref<AuditEvent[]>([])
const action = ref('')
const verify = ref<AuditVerifyResult | null>(null)
const verifying = ref(false)
const hasMore = ref(false)

async function load(more = false) {
  const before = more ? events.value[events.value.length - 1]?.id : undefined
  const res = await useApi().admin.audit({ action: action.value || undefined, before_id: before, limit: 100 })
  events.value = more ? [...events.value, ...res.items] : res.items
  hasMore.value = res.items.length === 100
}
async function runVerify() {
  verifying.value = true
  try {
    verify.value = await useApi().admin.verifyAudit()
  } finally {
    verifying.value = false
  }
}
onMounted(() => load())
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="Audit log">
      <template #actions><button type="button" class="btn-secondary" :disabled="verifying" data-testid="verify-audit" @click="runVerify"><AppIcon name="shield" :size="18" /> Verify integrity</button></template>
    </PageHeader>
    <AlertBox v-if="verify" :tone="verify.valid ? 'success' : 'danger'" :title="verify.valid ? 'Audit log intact' : 'Audit log integrity problem'" data-testid="verify-result">
      {{ verify.events_checked }} events checked.
      <span v-if="!verify.valid">First problem at event {{ verify.first_broken_id }}: {{ verify.problem }}. Preserve a database backup and investigate.</span>
    </AlertBox>
    <p class="text-ink-muted">Append-only and hash-chained. Events contain identifiers, actions and statuses - no request contents or personal data.</p>
    <form class="flex flex-wrap items-end gap-2" @submit.prevent="load()">
      <FormField v-slot="f" label="Filter by action"><input :id="f.id" v-model="action" class="field-input" placeholder="e.g. auth.login_failed"></FormField>
      <button type="submit" class="btn-secondary">Filter</button>
    </form>
    <div class="card overflow-x-auto">
      <table class="w-full min-w-[48rem] text-left text-sm">
        <thead><tr class="border-b-2 border-surface-strong"><th class="py-2 pr-3">#</th><th class="py-2 pr-3">Time</th><th class="py-2 pr-3">Action</th><th class="py-2 pr-3">Actor roles</th><th class="py-2 pr-3">Entity</th><th class="py-2 pr-3">Change</th><th class="py-2">Details</th></tr></thead>
        <tbody>
          <tr v-for="e in events" :key="e.id" class="border-b border-surface-border align-top">
            <td class="py-1.5 pr-3 tabular-nums">{{ e.id }}</td>
            <td class="py-1.5 pr-3 whitespace-nowrap">{{ formatDateTime(e.occurred_at) }}</td>
            <td class="py-1.5 pr-3">{{ actionLabel(e.action) }}<span class="block font-mono text-xs text-ink-muted">{{ e.action }}</span></td>
            <td class="py-1.5 pr-3">{{ e.actor_roles.join(', ') || '-' }}</td>
            <td class="py-1.5 pr-3 font-mono text-xs">{{ e.entity_type }}<span v-if="e.entity_id" class="block">{{ e.entity_id.slice(0, 8) }}</span></td>
            <td class="py-1.5 pr-3">{{ e.from_status ? `${e.from_status} -> ` : '' }}{{ e.to_status }}</td>
            <td class="py-1.5 font-mono text-xs">{{ Object.keys(e.metadata).length ? JSON.stringify(e.metadata) : '' }}<span v-if="e.reason" class="block font-sans">Reason: {{ e.reason }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>
    <button v-if="hasMore" type="button" class="btn-secondary w-fit" @click="load(true)">Load older events</button>
  </div>
</template>
