<script setup lang="ts">
import { DELIVERY_MODES, type CategoryCode, type ContactInput, type DeliveryMode, type LocationInput, type Offer, type OfferCreateInput } from '@reliefmesh/shared-types'
import { getDB, storePlaceholder } from '~/utils/offline-db'

const emit = defineEmits<{ done: [path: string] }>()
const auth = useAuthStore()
const sync = useSyncStore()
const toasts = useToastStore()
const errors = useFormErrors()

const category = ref<CategoryCode | ''>('')
const title = ref('')
const description = ref('')
const quantity = ref('')
const unit = ref('')
const mode = ref<DeliveryMode>('pickup')
const start = ref('')
const end = ref('')
const restrictions = ref('')
const accessibility = ref('')
const location = ref<LocationInput>({ mode: 'area_only', area_label: '' })
const contact = ref<ContactInput>({ method: 'phone', visibility: 'coordinators_only', details: '' })
const busy = ref(false)

function validate(): boolean {
  const f: Record<string, string> = {}
  if (!category.value) f.category = 'Choose a category.'
  if (title.value.trim().length < 3) f.title = 'Write a short title (at least 3 characters).'
  if (quantity.value === '' || Number.parseInt(quantity.value, 10) < 0) f.quantity_available = 'Enter the available quantity.'
  if (location.value.mode === 'area_only' && !location.value.area_label?.trim()) f['location.area_label'] = 'Enter an area.'
  errors.fields.value = f
  errors.message.value = Object.keys(f).length ? 'Please check the highlighted fields.' : ''
  return !Object.keys(f).length
}

async function submit(publish = true) {
  if (!validate()) return
  busy.value = true
  const input: OfferCreateInput = {
    client_id: uuid(), category: category.value as CategoryCode, title: title.value.trim(), description: description.value.trim(),
    quantity_available: Number.parseInt(quantity.value, 10), unit: unit.value.trim(), pickup_or_delivery_mode: mode.value,
    availability_start: fromLocalInput(start.value), availability_end: fromLocalInput(end.value),
    restrictions: restrictions.value.trim(), accessibility_notes: accessibility.value.trim(),
    location: location.value, contact: contact.value, publish, client_created_at: new Date().toISOString(),
  }
  try {
    const res = await sync.submit({ type: 'offer.create', payload: input, summary: `New offer: ${input.title}` }, { dropOnReject: true })
    if (res.status === 'applied') {
      const o = res.entity as Offer
      toasts.success(`Offer ${o.reference} created.`)
      emit('done', `/offers/${o.id}`)
    } else if (res.status === 'queued') {
      const p = offerPlaceholder(input, input.client_id!, auth.user!)
      await storePlaceholder(getDB(), 'offers', auth.user!.id, input.client_id!, p)
      toasts.info('Offer saved on this device. It will be sent when the connection returns.')
      emit('done', `/offers/${p.id}`)
    } else {
      errors.fromDetail(res.error)
    }
  } catch (e) {
    errors.fromError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="flex flex-col gap-6" novalidate data-testid="offer-form" @submit.prevent="submit(true)">
    <AlertBox v-if="errors.message.value" tone="danger" title="The offer was not saved">{{ errors.message.value }}</AlertBox>
    <section class="card"><CategoryPicker v-model="category" :error="errors.fields.value.category" /></section>
    <section class="card flex flex-col gap-4">
      <h2>What can you offer?</h2>
      <FormField v-slot="f" label="Title" required :error="errors.fields.value.title" help="E.g. '40 blankets from the sports club'.">
        <input :id="f.id" v-model="title" class="field-input" maxlength="120" data-testid="offer-title">
      </FormField>
      <FormField v-slot="f" label="Description" :error="errors.fields.value.description">
        <textarea :id="f.id" v-model="description" class="field-input" rows="3" maxlength="4000" />
      </FormField>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <FormField v-slot="f" label="Quantity available" required :error="errors.fields.value.quantity_available">
          <input :id="f.id" v-model="quantity" class="field-input" type="number" min="0" data-testid="offer-quantity">
        </FormField>
        <FormField v-slot="f" label="Unit" :error="errors.fields.value.unit"><input :id="f.id" v-model="unit" class="field-input" maxlength="40" data-testid="offer-unit"></FormField>
        <FormField v-slot="f" label="Pickup or delivery">
          <select :id="f.id" v-model="mode" class="field-input"><option v-for="m in DELIVERY_MODES" :key="m" :value="m">{{ deliveryModeLabel[m] }}</option></select>
        </FormField>
      </div>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField v-slot="f" label="Available from (optional)" :error="errors.fields.value.availability_start"><input :id="f.id" v-model="start" class="field-input" type="datetime-local"></FormField>
        <FormField v-slot="f" label="Available until (optional)" :error="errors.fields.value.availability_end"><input :id="f.id" v-model="end" class="field-input" type="datetime-local"></FormField>
      </div>
      <FormField v-slot="f" label="Restrictions (optional)" help="E.g. 'only weekdays', 'max. 2 wheelchairs per trip'."><textarea :id="f.id" v-model="restrictions" class="field-input" rows="2" maxlength="1000" /></FormField>
      <FormField v-slot="f" label="Accessibility (optional)"><textarea :id="f.id" v-model="accessibility" class="field-input" rows="2" maxlength="1000" /></FormField>
    </section>
    <section class="card"><LocationFields v-model="location" legend="Where is it located?" :errors="errors.fields.value" /></section>
    <section class="card"><ContactFields v-model="contact" :errors="errors.fields.value" /></section>
    <div class="flex flex-col gap-3 sm:flex-row">
      <button type="submit" class="btn-primary text-lg" :disabled="busy" data-testid="submit-offer"><AppIcon name="check" /> {{ busy ? 'Saving...' : 'Publish offer' }}</button>
      <button type="button" class="btn-secondary" :disabled="busy" @click="submit(false)">Save as draft</button>
    </div>
  </form>
</template>
