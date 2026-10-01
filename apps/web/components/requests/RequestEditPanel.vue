<script setup lang="ts">
import { URGENCIES, VERIFICATION_LEVELS, type AidRequest, type RequestUpdateInput, type ReviewStatus, type Urgency, type VerificationLevel } from '@reliefmesh/shared-types'
import { MEDICINE_PICKUP } from '@reliefmesh/shared-types'

const props = defineProps<{ request: AidRequest }>()
const emit = defineEmits<{ updated: [request: AidRequest]; queued: [] }>()
const sync = useSyncStore()
const toasts = useToastStore()
const errors = useFormErrors()
const open = ref(false)
const busy = ref(false)

const reviewStatuses: ReviewStatus[] = ['pending', 'in_review', 'reviewed', 'needs_information']
const form = reactive({
  title: '', description: '', quantity: '', unit: '', people: '', neededBy: '', accessibility: '',
  urgency: 'normal' as Urgency, review: 'pending' as ReviewStatus, verification: 'unverified' as VerificationLevel,
  team: '', expiry: '', tags: '', ack: false,
})

function reset() {
  const r = props.request
  Object.assign(form, {
    title: r.title, description: r.description, quantity: r.requested_quantity?.toString() ?? '', unit: r.requested_unit,
    people: r.estimated_people_affected?.toString() ?? '', neededBy: toLocalInput(r.requested_by_time), accessibility: r.accessibility_notes,
    urgency: r.urgency, review: r.review_status ?? 'pending', verification: r.verification_level ?? 'unverified',
    team: r.assigned_team ?? '', expiry: toLocalInput(r.expiry_at ?? null), tags: (r.tags ?? []).join(', '), ack: false,
  })
  errors.clear()
}
watch(open, (o) => { if (o) reset() })

const coordinator = computed(() => props.request.permissions.can_edit_coordinator_fields)
const isMedicine = computed(() => props.request.category === MEDICINE_PICKUP)
const intOrNull = (v: string) => (v === '' ? null : Number.parseInt(v, 10))

async function save() {
  busy.value = true
  errors.clear()
  const r = props.request
  const payload: RequestUpdateInput = { version: r.version }
  if (!isMedicine.value && form.title !== r.title) payload.title = form.title
  if (form.description !== r.description && !r.details_redacted) payload.description = form.description
  if (intOrNull(form.quantity) !== r.requested_quantity) payload.requested_quantity = intOrNull(form.quantity)
  if (form.unit !== r.requested_unit) payload.requested_unit = form.unit
  if (intOrNull(form.people) !== r.estimated_people_affected) payload.estimated_people_affected = intOrNull(form.people)
  if (fromLocalInput(form.neededBy) !== (r.requested_by_time ? new Date(r.requested_by_time).toISOString() : null)) payload.requested_by_time = fromLocalInput(form.neededBy)
  if (form.accessibility !== r.accessibility_notes && !r.details_redacted) payload.accessibility_notes = form.accessibility
  if (form.urgency !== r.urgency) {
    payload.urgency = form.urgency
    if (form.urgency === 'critical') payload.emergency_notice_acknowledged = form.ack
  }
  if (coordinator.value) {
    if (form.review !== r.review_status) payload.review_status = form.review
    if (form.verification !== r.verification_level) payload.verification_level = form.verification
    if (form.team !== (r.assigned_team ?? '')) payload.assigned_team = form.team
    if (fromLocalInput(form.expiry) !== (r.expiry_at ? new Date(r.expiry_at).toISOString() : null)) payload.expiry_at = fromLocalInput(form.expiry)
    const tags = form.tags.split(',').map((t) => t.trim()).filter(Boolean)
    if (tags.join(',') !== (r.tags ?? []).join(',')) payload.tags = tags
  }
  try {
    const res = await sync.submit({ type: 'request.update', entity_id: r.id, payload, summary: `Edit ${r.reference}` }, { dropOnReject: true })
    if (res.status === 'applied') {
      toasts.success('Changes saved.')
      emit('updated', res.entity as AidRequest)
      open.value = false
    } else if (res.status === 'queued') {
      toasts.info('Changes saved on this device and will be sent when the connection returns.')
      emit('queued')
      open.value = false
    } else {
      errors.fromDetail(res.error)
      if (res.status === 'conflict') {
        await sync.discard(res.opId)
        if (res.entity) emit('updated', res.entity as AidRequest)
        errors.message.value = 'Someone else changed this request in the meantime. The latest version is shown; please apply your changes again.'
      }
    }
  } catch (e) {
    errors.fromError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section v-if="request.permissions.can_edit" class="card">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h2>{{ coordinator ? 'Review and edit' : 'Edit request' }}</h2>
      <button type="button" class="btn-secondary" :aria-expanded="open" data-testid="edit-request" @click="open = !open">
        <AppIcon name="edit" :size="18" /> {{ open ? 'Close' : 'Edit' }}
      </button>
    </div>
    <p v-if="!open && !coordinator" class="mt-2 text-ink-muted">You can edit your request until a coordinator starts reviewing it.</p>
    <form v-if="open" class="mt-4 flex flex-col gap-4" @submit.prevent="save">
      <AlertBox v-if="errors.message.value" tone="danger">{{ errors.message.value }}</AlertBox>
      <fieldset v-if="coordinator" class="flex flex-col gap-4 rounded-lg border-2 border-brand p-3">
        <legend class="px-1 font-bold">Coordinator review</legend>
        <p class="text-sm">Urgency is assessed manually. Do not prioritize on the basis of personal characteristics or health data.</p>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <FormField v-slot="f" label="Urgency" :error="errors.fields.value.urgency">
            <select :id="f.id" v-model="form.urgency" class="field-input" data-testid="edit-urgency"><option v-for="u in URGENCIES" :key="u" :value="u">{{ urgencyLabel[u] }}</option></select>
          </FormField>
          <FormField v-slot="f" label="Review status">
            <select :id="f.id" v-model="form.review" class="field-input"><option v-for="s in reviewStatuses" :key="s" :value="s">{{ reviewStatusLabel[s] }}</option></select>
          </FormField>
          <FormField v-slot="f" label="Verification">
            <select :id="f.id" v-model="form.verification" class="field-input"><option v-for="v in VERIFICATION_LEVELS" :key="v" :value="v">{{ verificationLabel[v] }}</option></select>
          </FormField>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <FormField v-slot="f" label="Responsible team" :error="errors.fields.value.assigned_team">
            <input :id="f.id" v-model="form.team" class="field-input" maxlength="120">
          </FormField>
          <FormField v-slot="f" label="Review by (expiry)" help="Shown as 'past expiry' on the dashboard. Never closes automatically." :error="errors.fields.value.expiry_at">
            <input :id="f.id" v-model="form.expiry" class="field-input" type="datetime-local">
          </FormField>
          <FormField v-slot="f" label="Tags" help="Comma separated" :error="errors.fields.value.tags">
            <input :id="f.id" v-model="form.tags" class="field-input">
          </FormField>
        </div>
      </fieldset>
      <FormField v-if="!coordinator" v-slot="f" label="Urgency" :error="errors.fields.value.urgency || errors.fields.value.emergency_notice_acknowledged">
        <select :id="f.id" v-model="form.urgency" class="field-input sm:max-w-xs"><option v-for="u in URGENCIES" :key="u" :value="u">{{ urgencyLabel[u] }}</option></select>
      </FormField>
      <label v-if="form.urgency === 'critical' && request.urgency !== 'critical' && !coordinator" class="choice border-danger">
        <input v-model="form.ack" type="checkbox">
        <span class="font-semibold">{{ useSettingsStore().settings?.critical_urgency_notice }}</span>
      </label>
      <FormField v-if="!isMedicine" v-slot="f" label="Title" :error="errors.fields.value.title">
        <input :id="f.id" v-model="form.title" class="field-input" maxlength="120">
      </FormField>
      <FormField v-if="!request.details_redacted && (!isMedicine || useSettingsStore().settings?.allow_medicine_free_text)" v-slot="f" label="Description" :error="errors.fields.value.description">
        <textarea :id="f.id" v-model="form.description" class="field-input" rows="3" maxlength="4000" />
      </FormField>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <FormField v-slot="f" label="Quantity"><input :id="f.id" v-model="form.quantity" class="field-input" type="number" min="0"></FormField>
        <FormField v-slot="f" label="Unit"><input :id="f.id" v-model="form.unit" class="field-input" maxlength="40"></FormField>
        <FormField v-slot="f" label="People affected"><input :id="f.id" v-model="form.people" class="field-input" type="number" min="0"></FormField>
      </div>
      <FormField v-slot="f" label="Needed by"><input :id="f.id" v-model="form.neededBy" class="field-input sm:max-w-xs" type="datetime-local"></FormField>
      <FormField v-if="!request.details_redacted" v-slot="f" label="Accessibility needs"><textarea :id="f.id" v-model="form.accessibility" class="field-input" rows="2" maxlength="1000" /></FormField>
      <div class="flex gap-2">
        <button type="submit" class="btn-primary" :disabled="busy" data-testid="save-request">Save changes</button>
        <button type="button" class="btn-secondary" @click="open = false">Cancel</button>
      </div>
    </form>
  </section>
</template>
