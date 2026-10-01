<script setup lang="ts">
import type { DemoInfo, RetentionPreview } from '@reliefmesh/shared-types'

definePageMeta({ layout: 'admin', requires: 'retention.manage' })
useHead({ title: 'Data and retention' })
const toasts = useToastStore()
const errors = useFormErrors()
const preview = ref<RetentionPreview | null>(null)
const demo = ref<DemoInfo | null>(null)
const retentionOpen = ref(false)
const deleteForm = reactive({ entity_type: 'request' as 'request' | 'offer', reference: '', reason: '' })
const deleteOpen = ref(false)
const busy = ref(false)

async function load() {
  const api = useApi()
  ;[preview.value, demo.value] = await Promise.all([api.admin.retentionPreview(), api.admin.demo()])
}
onMounted(load)

async function applyRetention() {
  busy.value = true
  try {
    const r = await useApi().admin.applyRetention()
    toasts.success(`Redacted ${r.requests} request(s) and ${r.offers} offer(s).`)
    retentionOpen.value = false
    await load()
  } finally {
    busy.value = false
  }
}
async function deleteRecord() {
  busy.value = true
  errors.clear()
  try {
    await useApi().admin.deleteRecord(deleteForm.entity_type, deleteForm.reference.trim(), deleteForm.reason.trim())
    toasts.success(`${deleteForm.reference} deleted.`)
    deleteOpen.value = false
    Object.assign(deleteForm, { reference: '', reason: '' })
  } catch (e) {
    errors.fromError(e)
  } finally {
    busy.value = false
  }
}
async function seed(name: string) {
  busy.value = true
  try {
    await useApi().admin.seedDemo(name)
    toasts.success('Exercise scenario loaded.')
    await load()
  } catch (e) {
    errors.fromError(e)
    toasts.error(errors.message.value)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="Data and retention" />
    <section class="card flex flex-col gap-3">
      <h2>Retention</h2>
      <p>Closed requests and cancelled or expired offers are redacted after <strong>{{ preview?.retention_days ?? '...' }}</strong> days: titles, descriptions, notes, locations and contact details are removed. Counts, categories and timestamps remain for statistics; the audit log is kept.</p>
      <p v-if="preview">Currently due: <strong>{{ preview.requests }}</strong> request(s), <strong>{{ preview.offers }}</strong> offer(s).</p>
      <button type="button" class="btn-danger w-fit" :disabled="!preview || (!preview.requests && !preview.offers)" @click="retentionOpen = true">Apply retention now...</button>
      <p class="text-sm text-ink-muted">Operators can also schedule <code>reliefmesh-api apply-retention</code>, e.g. nightly.</p>
    </section>
    <section class="card flex flex-col gap-3">
      <h2>Delete a record</h2>
      <p>For deletion requests from data subjects. Enter the reference; the content is removed and the deletion is logged with your reason.</p>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <FormField v-slot="f" label="Type"><select :id="f.id" v-model="deleteForm.entity_type" class="field-input"><option value="request">Request</option><option value="offer">Offer</option></select></FormField>
        <FormField v-slot="f" label="Reference" :error="errors.fields.value.reference"><input :id="f.id" v-model="deleteForm.reference" class="field-input font-mono" placeholder="RM-2026-0000001"></FormField>
      </div>
      <FormField v-slot="f" label="Reason" :error="errors.fields.value.reason"><input :id="f.id" v-model="deleteForm.reason" class="field-input" maxlength="1000"></FormField>
      <button type="button" class="btn-danger w-fit" :disabled="!deleteForm.reference || deleteForm.reason.length < 5" @click="deleteOpen = true">Delete record...</button>
    </section>
    <section class="card flex flex-col gap-3">
      <h2>Exercise scenarios (demo data)</h2>
      <template v-if="demo?.enabled">
        <AlertBox tone="warning">Demo accounts share the public password <code class="font-mono">{{ demo.password }}</code>. Never use demo mode with real data.</AlertBox>
        <ul class="flex flex-col gap-2">
          <li v-for="s in demo.scenarios" :key="s.name" class="flex flex-col gap-2 rounded-lg border border-surface-border p-3 sm:flex-row sm:items-center sm:justify-between">
            <div><p class="font-semibold">{{ s.title }}</p><p class="text-sm text-ink-muted">{{ s.description }}</p></div>
            <button type="button" class="btn-secondary shrink-0" :disabled="busy || demo.seeded.includes(s.name)" @click="seed(s.name)">{{ demo.seeded.includes(s.name) ? 'Loaded' : 'Load scenario' }}</button>
          </li>
        </ul>
        <p class="text-sm">Demo accounts: {{ demo.personas.map((p) => p.username).join(', ') }}</p>
      </template>
      <p v-else>Demo data is disabled on this instance (set <code>RELIEFMESH_ALLOW_DEMO_SEED=true</code> on a dedicated exercise instance to enable it).</p>
    </section>
    <ConfirmDialog :open="retentionOpen" title="Apply retention now?" confirm-label="Redact permanently" danger :busy="busy" @confirm="applyRetention" @cancel="retentionOpen = false">
      <p>Personal data and free text of {{ preview?.requests }} request(s) and {{ preview?.offers }} offer(s) will be removed permanently. This cannot be undone.</p>
    </ConfirmDialog>
    <ConfirmDialog :open="deleteOpen" :title="`Delete ${deleteForm.reference}?`" confirm-label="Delete permanently" danger :busy="busy" @confirm="deleteRecord" @cancel="deleteOpen = false">
      <AlertBox v-if="errors.message.value" tone="danger">{{ errors.message.value }}</AlertBox>
      <p>The record content is removed permanently and the record disappears for everyone. The deletion stays in the audit log.</p>
    </ConfirmDialog>
  </div>
</template>
