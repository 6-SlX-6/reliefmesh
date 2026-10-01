<script setup lang="ts">
// Manual assignment by a coordinator. ReliefMesh never matches or assigns
// automatically; this dialog only lists options and checks quantities.
import type { AidRequest, Assignment, Offer, Volunteer } from '@reliefmesh/shared-types'

const props = defineProps<{ request: AidRequest }>()
const emit = defineEmits<{ created: [assignment: Assignment] }>()
const settings = useSettingsStore()
const toasts = useToastStore()
const errors = useFormErrors()
const open = ref(false)
const busy = ref(false)
const offers = ref<Offer[]>([])
const volunteers = ref<Volunteer[]>([])
const showAllCategories = ref(false)

const form = reactive({
  offerId: '', volunteerId: '', team: '', quantity: '', unit: '', instructions: '', eta: '',
  grantContact: false, grantPickup: false, grantDestination: false,
})

async function openDialog() {
  errors.clear()
  Object.assign(form, { offerId: '', volunteerId: '', team: '', quantity: '', unit: props.request.requested_unit, instructions: '', eta: '', grantContact: false, grantPickup: false, grantDestination: false })
  open.value = true
  try {
    const api = useApi()
    const [o, v] = await Promise.all([api.offers.list({ allocatable: true, limit: 500 }), api.volunteers.list()])
    offers.value = o.items
    volunteers.value = v.items
  } catch (e) {
    errors.fromError(e)
  }
}

const visibleOffers = computed(() => showAllCategories.value ? offers.value : offers.value.filter((o) => o.category === props.request.category))
const selectedOffer = computed(() => offers.value.find((o) => o.id === form.offerId))
const qty = computed(() => (form.quantity === '' ? 0 : Number.parseInt(form.quantity, 10)))
const overAllocation = computed(() => selectedOffer.value !== undefined && qty.value > selectedOffer.value.remaining_quantity)
const hasVolunteer = computed(() => form.volunteerId !== '')
const valid = computed(() => (form.offerId || form.volunteerId || form.team.trim()) && (!form.offerId || qty.value > 0))

watch(selectedOffer, (o) => { if (o && !form.unit) form.unit = o.unit })
watch(hasVolunteer, (v) => { if (!v) Object.assign(form, { grantContact: false, grantPickup: false, grantDestination: false }) })

async function create() {
  busy.value = true
  errors.clear()
  try {
    const a = await useApi().assignments.create({
      client_id: uuid(),
      request_id: props.request.id,
      offer_id: form.offerId || null,
      volunteer_user_id: form.volunteerId || null,
      team_label: form.team.trim(),
      quantity_assigned: form.offerId ? qty.value : (form.quantity ? qty.value : 0),
      unit: form.unit.trim(),
      instructions: form.instructions.trim(),
      eta_text: form.eta.trim(),
      protected_contact_access_granted: form.grantContact,
      pickup_location_access_granted: form.grantPickup,
      destination_location_access_granted: form.grantDestination,
    })
    toasts.success('Assignment created.')
    emit('created', a)
    open.value = false
  } catch (e) {
    errors.fromError(e)
    const remaining = (e as { details?: Record<string, unknown> }).details?.remaining_quantity
    if (remaining !== undefined) errors.message.value += ` Remaining: ${remaining}.`
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div>
    <button type="button" class="btn-primary" data-testid="open-assign" :disabled="!useSyncStore().online" @click="openDialog">
      <AppIcon name="plus" :size="18" /> Assign help or resources
    </button>
    <p v-if="!useSyncStore().online" class="field-help">Assigning needs a connection so quantities can be checked.</p>
    <ConfirmDialog :open="open" title="Create assignment" confirm-label="Create assignment" :busy="busy" :confirm-disabled="!valid" @confirm="create" @cancel="open = false">
      <AlertBox v-if="errors.message.value" tone="danger" data-testid="assign-error">{{ errors.message.value }}</AlertBox>
      <p class="text-sm text-ink-muted">You decide who helps. ReliefMesh does not suggest or dispatch responders and does not provide routes.</p>

      <FormField v-slot="f" label="Offer to allocate from (optional)" :error="errors.fields.value.offer_id">
        <select :id="f.id" v-model="form.offerId" class="field-input" data-testid="assign-offer">
          <option value="">No offer</option>
          <option v-for="o in visibleOffers" :key="o.id" :value="o.id">
            {{ o.reference }} - {{ o.title }} ({{ o.remaining_quantity }} {{ o.unit }} left{{ showAllCategories ? `, ${settings.categoryLabel(o.category)}` : '' }})
          </option>
        </select>
      </FormField>
      <label class="flex items-center gap-2 font-normal"><input v-model="showAllCategories" type="checkbox" class="h-5 w-5"> Show offers from all categories</label>

      <div class="grid grid-cols-2 gap-3">
        <FormField v-slot="f" :label="form.offerId ? 'Quantity to allocate' : 'Quantity (optional)'" :required="!!form.offerId" :error="errors.fields.value.quantity_assigned">
          <input :id="f.id" v-model="form.quantity" class="field-input" type="number" min="0" data-testid="assign-quantity" :aria-invalid="overAllocation ? 'true' : undefined">
        </FormField>
        <FormField v-slot="f" label="Unit"><input :id="f.id" v-model="form.unit" class="field-input" maxlength="40"></FormField>
      </div>
      <AlertBox v-if="overAllocation" tone="warning" data-testid="over-allocation-warning">
        This is more than the {{ selectedOffer?.remaining_quantity }} {{ selectedOffer?.unit }} still available on this offer. The server will reject over-allocation.
      </AlertBox>

      <FormField v-slot="f" label="Volunteer (optional)" :error="errors.fields.value.volunteer_user_id">
        <select :id="f.id" v-model="form.volunteerId" class="field-input" data-testid="assign-volunteer">
          <option value="">No individual volunteer</option>
          <option v-for="v in volunteers" :key="v.id" :value="v.id">
            {{ v.display_name }} - {{ availabilityLabel[v.availability] }}{{ v.active_assignments ? `, ${v.active_assignments} active` : '' }}
          </option>
        </select>
      </FormField>
      <FormField v-slot="f" label="Team (optional)" help="E.g. 'Shelter kitchen team'.">
        <input :id="f.id" v-model="form.team" class="field-input" maxlength="120">
      </FormField>

      <fieldset v-if="hasVolunteer" class="flex flex-col gap-2 rounded-lg border-2 border-brand p-3">
        <legend class="px-1 font-bold">Share protected details with this volunteer?</legend>
        <p class="text-sm">Share only what is needed. Access is possible while the assignment is active and every opening is logged.</p>
        <label class="choice"><input v-model="form.grantDestination" type="checkbox" :disabled="!request.location.has_exact" data-testid="grant-destination"><span>Destination (exact location of the request){{ request.location.has_exact ? '' : ' - none stored' }}</span></label>
        <label class="choice"><input v-model="form.grantContact" type="checkbox" :disabled="request.contact_visibility !== 'assigned_responders' || !request.has_contact_details"><span>Requester contact details{{ request.contact_visibility !== 'assigned_responders' ? ' - requester allowed coordinators only' : '' }}</span></label>
        <label class="choice"><input v-model="form.grantPickup" type="checkbox" :disabled="!selectedOffer?.location.has_exact"><span>Pickup location of the offer</span></label>
      </fieldset>

      <FormField v-slot="f" label="Instructions (optional)" help="Operational notes only, e.g. 'Pick up at gate B, hand over at the shelter desk'. Do not describe routes or claim that a place is safe.">
        <textarea :id="f.id" v-model="form.instructions" class="field-input" rows="3" maxlength="2000" />
      </FormField>
      <FormField v-slot="f" label="Expected time (optional)" help="Free text, e.g. 'this afternoon'.">
        <input :id="f.id" v-model="form.eta" class="field-input" maxlength="120">
      </FormField>
    </ConfirmDialog>
  </div>
</template>
