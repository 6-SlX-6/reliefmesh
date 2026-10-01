<script setup lang="ts">
import type { AidRequest, CategoryCode, ContactInput, LocationInput, RequestCreateInput, Urgency } from '@reliefmesh/shared-types'
import { MEDICINE_PICKUP } from '@reliefmesh/shared-types'
import { cacheEntities, getDB } from '~/utils/offline-db'

const emit = defineEmits<{ done: [path: string] }>()
const auth = useAuthStore()
const settings = useSettingsStore()
const sync = useSyncStore()
const toasts = useToastStore()
const errors = useFormErrors()

const category = ref<CategoryCode | ''>('')
const urgency = ref<Urgency>('normal')
const title = ref('')
const description = ref('')
const quantity = ref<string>('')
const unit = ref('')
const people = ref<string>('')
const neededBy = ref('')
const accessibility = ref('')
const sensitive = ref(false)
const requiresAuthorization = ref(false)
const location = ref<LocationInput>({ mode: 'area_only', area_label: '' })
const contact = ref<ContactInput>({ method: 'phone', visibility: 'coordinators_only', details: '' })
const busy = ref(false)
const showCritical = ref(false)
const submitAsDraft = ref(false)

const isMedicine = computed(() => category.value === MEDICINE_PICKUP)
const allowMedicineText = computed(() => settings.settings?.allow_medicine_free_text ?? false)
const medicalHint = computed(() => !isMedicine.value && (looksMedical(description.value) || looksMedical(accessibility.value)))
const contactInText = computed(() => looksLikeContact(description.value) || looksLikeContact(title.value))

function intOrNull(v: string): number | null {
  if (v === '' || v === null) return null
  const n = Number.parseInt(v, 10)
  return Number.isFinite(n) ? n : null
}

function clientValidate(): boolean {
  const f: Record<string, string> = {}
  if (!category.value) f.category = 'Choose what is needed.'
  if (!isMedicine.value && title.value.trim().length < 3) f.title = 'Write a short title (at least 3 characters).'
  if (isMedicine.value && description.value.trim() && !allowMedicineText.value) f.description = 'Leave this empty for medicine pickup requests.'
  if (location.value.mode === 'area_only' && !location.value.area_label?.trim()) f['location.area_label'] = 'Enter an area.'
  if (location.value.mode === 'approximate' && !validCoords(location.value.lat, location.value.lon)) f['location.lat'] = 'Enter coordinates or use your approximate location.'
  if (location.value.mode === 'protected_exact' && !location.value.exact?.address?.trim()) f['location.exact'] = 'Enter the address.'
  errors.fields.value = f
  errors.message.value = Object.keys(f).length ? 'Please check the highlighted fields.' : ''
  return Object.keys(f).length === 0
}

function buildInput(ack: boolean): RequestCreateInput {
  return {
    client_id: uuid(),
    category: category.value as CategoryCode,
    urgency: urgency.value,
    title: isMedicine.value ? '' : title.value.trim(),
    description: isMedicine.value && !allowMedicineText.value ? '' : description.value.trim(),
    requested_quantity: intOrNull(quantity.value),
    requested_unit: unit.value.trim(),
    estimated_people_affected: intOrNull(people.value),
    requested_by_time: fromLocalInput(neededBy.value),
    location: location.value,
    accessibility_notes: accessibility.value.trim(),
    contact: contact.value,
    sensitive_data_flag: sensitive.value,
    requires_formal_authorization: isMedicine.value ? requiresAuthorization.value : false,
    submit: !submitAsDraft.value,
    emergency_notice_acknowledged: ack,
    client_created_at: new Date().toISOString(),
  }
}

function onSubmit(draft = false) {
  submitAsDraft.value = draft
  if (!clientValidate()) return
  if (urgency.value === 'critical' && !draft) {
    showCritical.value = true
    return
  }
  void send(false)
}

async function send(ack: boolean) {
  busy.value = true
  errors.clear()
  const input = buildInput(ack || urgency.value !== 'critical' || submitAsDraft.value)
  const summary = `New request: ${input.title || 'Medicine pickup'}`
  try {
    const res = await sync.submit({ type: 'request.create', payload: input, summary }, { dropOnReject: true })
    if (res.status === 'applied') {
      const v = res.entity as AidRequest
      toasts.success(`Request ${v.reference} ${input.submit ? 'submitted' : 'saved as draft'}.`)
      emit('done', `/requests/${v.id}`)
    } else if (res.status === 'queued') {
      const placeholder = requestPlaceholder(input, input.client_id!, auth.user!, settings.approxDecimals)
      await getDB().requests.put({ id: placeholder.id, user_id: auth.user!.id, client_id: input.client_id, pending: true, data: placeholder, cached_at: new Date().toISOString() })
      toasts.info('Saved on this device. It will be sent automatically when the connection returns.')
      emit('done', `/requests/${placeholder.id}`)
    } else {
      errors.fromDetail(res.error)
      if (res.entity) await cacheEntities(getDB(), 'requests', auth.user!.id, [res.entity as AidRequest])
    }
  } catch (e) {
    errors.fromError(e)
  } finally {
    busy.value = false
    showCritical.value = false
  }
}
</script>

<template>
  <form class="flex flex-col gap-6" novalidate data-testid="request-form" @submit.prevent="onSubmit(false)">
    <AlertBox v-if="errors.message.value" tone="danger" title="The request was not sent">{{ errors.message.value }}</AlertBox>
    <AlertBox v-if="!sync.online" tone="warning" title="You are offline">
      You can still fill in and submit this request. It is saved on this device and sent automatically when the connection returns.
    </AlertBox>

    <section class="card"><CategoryPicker v-model="category" :error="errors.fields.value.category" /></section>

    <section v-if="isMedicine" class="flex flex-col gap-3">
      <MedicinePrivacyNotice />
    </section>

    <section class="card"><UrgencyPicker v-model="urgency" :error="errors.fields.value.urgency || errors.fields.value.emergency_notice_acknowledged" /></section>

    <section class="card flex flex-col gap-4">
      <h2>Details</h2>
      <FormField v-if="!isMedicine" v-slot="f" label="Short title" required :error="errors.fields.value.title" help="For example: 'Drinking water for 20 people at the gym'.">
        <input :id="f.id" v-model="title" class="field-input" maxlength="120" :aria-describedby="f.describedBy" :aria-invalid="f.invalid" data-testid="request-title">
      </FormField>
      <FormField v-if="!isMedicine || allowMedicineText" v-slot="f" label="Description" :error="errors.fields.value.description"
        :help="isMedicine ? 'Logistics only. No medical details.' : 'What exactly is needed? Do not include health details or names.'">
        <textarea :id="f.id" v-model="description" class="field-input" rows="4" maxlength="4000" :aria-describedby="f.describedBy" :aria-invalid="f.invalid" data-testid="request-description" />
      </FormField>
      <AlertBox v-if="medicalHint" tone="warning">This text may contain health information. Please remove it - it is not needed to organize help.</AlertBox>
      <AlertBox v-if="contactInText" tone="warning">It looks like contact details are in the title or description. Please use the protected contact field below instead.</AlertBox>
      <label v-if="isMedicine" class="choice">
        <input v-model="requiresAuthorization" type="checkbox" data-testid="requires-authorization">
        <span>Formal authorization is needed for the pickup (for example a signed authorization). Do not upload or describe it here.</span>
      </label>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <FormField v-slot="f" label="Quantity" :error="errors.fields.value.requested_quantity">
          <input :id="f.id" v-model="quantity" class="field-input" type="number" min="0" inputmode="numeric" data-testid="request-quantity">
        </FormField>
        <FormField v-slot="f" label="Unit" help="e.g. litres, meals, places" :error="errors.fields.value.requested_unit">
          <input :id="f.id" v-model="unit" class="field-input" maxlength="40" data-testid="request-unit">
        </FormField>
        <FormField v-slot="f" label="People affected" :error="errors.fields.value.estimated_people_affected">
          <input :id="f.id" v-model="people" class="field-input" type="number" min="0" inputmode="numeric">
        </FormField>
      </div>
      <FormField v-slot="f" label="Needed by (optional)" :error="errors.fields.value.requested_by_time">
        <input :id="f.id" v-model="neededBy" class="field-input sm:max-w-xs" type="datetime-local">
      </FormField>
      <FormField v-slot="f" label="Accessibility needs (optional)" help="E.g. step-free access, wheelchair-accessible vehicle. No diagnoses." :error="errors.fields.value.accessibility_notes">
        <textarea :id="f.id" v-model="accessibility" class="field-input" rows="2" maxlength="1000" />
      </FormField>
      <label v-if="!isMedicine" class="choice">
        <input v-model="sensitive" type="checkbox">
        <span>This request contains sensitive information. Details are hidden from lists and shown only to the people handling it.</span>
      </label>
    </section>

    <section class="card"><LocationFields v-model="location" :errors="errors.fields.value" /></section>
    <section class="card"><ContactFields v-model="contact" :errors="errors.fields.value" /></section>

    <AlertBox tone="info" title="Who can see this request?">
      Coordinators of {{ auth.user?.organization.name }} see the request. Volunteers only see it once it is assigned to them,
      and only the details needed for their task. Your contact details and exact address stay encrypted and every access is logged.
    </AlertBox>

    <div class="flex flex-col gap-3 sm:flex-row">
      <button type="submit" class="btn-primary text-lg" :disabled="busy" data-testid="submit-request">
        <AppIcon name="check" /> {{ busy ? 'Sending...' : 'Submit request' }}
      </button>
      <button type="button" class="btn-secondary" :disabled="busy" @click="onSubmit(true)">Save as draft</button>
    </div>

    <CriticalNoticeDialog :open="showCritical" :busy="busy" @confirm="send(true)" @cancel="showCritical = false" />
  </form>
</template>
