<script setup lang="ts">
import type { Settings } from '@reliefmesh/shared-types'

definePageMeta({ layout: 'admin', requires: 'settings.manage' })
useHead({ title: 'Settings' })
const settingsStore = useSettingsStore()
const toasts = useToastStore()
const errors = useFormErrors()
const form = ref<Partial<Settings>>({})
const busy = ref(false)

onMounted(async () => {
  await settingsStore.load()
  form.value = { ...settingsStore.settings }
})

async function save() {
  busy.value = true
  errors.clear()
  try {
    const s = await useApi().admin.updateSettings({
      instance_name: form.value.instance_name, emergency_notice: form.value.emergency_notice,
      critical_urgency_notice: form.value.critical_urgency_notice, medicine_privacy_notice: form.value.medicine_privacy_notice,
      exercise_mode: form.value.exercise_mode, exercise_label: form.value.exercise_label,
      approx_location_decimals: Number(form.value.approx_location_decimals), retention_days_closed: Number(form.value.retention_days_closed),
      default_request_expiry_hours: Number(form.value.default_request_expiry_hours), allow_medicine_free_text: form.value.allow_medicine_free_text,
      volunteer_access_requires_grant: form.value.volunteer_access_requires_grant,
    })
    form.value = { ...s }
    await settingsStore.load()
    toasts.success('Settings saved.')
  } catch (e) {
    errors.fromError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="flex flex-col gap-4" @submit.prevent="save">
    <PageHeader title="Settings and notices" />
    <AlertBox v-if="errors.message.value" tone="danger">{{ errors.message.value }}</AlertBox>
    <section class="card flex flex-col gap-4">
      <h2>Emergency notice</h2>
      <p class="text-sm">Shown on every screen and before sign-in. Use the correct emergency number for your country. It must clearly state that ReliefMesh is not an emergency service.</p>
      <FormField v-slot="f" label="Emergency notice" required :error="errors.fields.value.emergency_notice">
        <textarea :id="f.id" v-model="form.emergency_notice" class="field-input" rows="3" maxlength="1000" data-testid="setting-emergency-notice" />
      </FormField>
      <FormField v-slot="f" label="Notice before submitting a critical request" required :error="errors.fields.value.critical_urgency_notice">
        <textarea :id="f.id" v-model="form.critical_urgency_notice" class="field-input" rows="3" maxlength="1500" />
      </FormField>
      <FormField v-slot="f" label="Medicine pickup privacy notice" required :error="errors.fields.value.medicine_privacy_notice">
        <textarea :id="f.id" v-model="form.medicine_privacy_notice" class="field-input" rows="3" maxlength="1500" />
      </FormField>
    </section>
    <section class="card flex flex-col gap-4">
      <h2>Instance</h2>
      <FormField v-slot="f" label="Instance name" :error="errors.fields.value.instance_name"><input :id="f.id" v-model="form.instance_name" class="field-input" maxlength="80"></FormField>
      <label class="choice"><input v-model="form.exercise_mode" type="checkbox"><span><strong>Exercise mode</strong><span class="block text-sm font-normal">Shows a banner on every screen so nobody mistakes exercise data for a real situation.</span></span></label>
      <FormField v-slot="f" label="Exercise banner text" :error="errors.fields.value.exercise_label"><input :id="f.id" v-model="form.exercise_label" class="field-input" maxlength="120"></FormField>
    </section>
    <section class="card flex flex-col gap-4">
      <h2>Privacy</h2>
      <FormField v-slot="f" label="Approximate location precision" help="Decimal places for rounded coordinates. 2 = about 1.1 km (default), 1 = about 11 km, 3 = about 110 m." :error="errors.fields.value.approx_location_decimals">
        <select :id="f.id" v-model.number="form.approx_location_decimals" class="field-input sm:max-w-xs">
          <option :value="0">0 (about 111 km)</option><option :value="1">1 (about 11 km)</option><option :value="2">2 (about 1.1 km)</option><option :value="3">3 (about 110 m)</option>
        </select>
      </FormField>
      <FormField v-slot="f" label="Redact closed records after (days)" :error="errors.fields.value.retention_days_closed">
        <input :id="f.id" v-model.number="form.retention_days_closed" class="field-input sm:max-w-xs" type="number" min="1" max="3650">
      </FormField>
      <FormField v-slot="f" label="Default review deadline for new requests (hours)" help="Requests past this time are highlighted for review. They are never closed automatically." :error="errors.fields.value.default_request_expiry_hours">
        <input :id="f.id" v-model.number="form.default_request_expiry_hours" class="field-input sm:max-w-xs" type="number" min="1" max="8760">
      </FormField>
      <label class="choice"><input v-model="form.volunteer_access_requires_grant" type="checkbox"><span><strong>Volunteers need an explicit grant for protected details</strong><span class="block text-sm font-normal">Recommended. If off, an active assignment alone gives access.</span></span></label>
      <label class="choice"><input v-model="form.allow_medicine_free_text" type="checkbox"><span><strong>Allow free text for medicine pickup requests</strong><span class="block text-sm font-normal">Not recommended. Free text invites health data.</span></span></label>
    </section>
    <button type="submit" class="btn-primary w-fit" :disabled="busy" data-testid="save-settings">Save settings</button>
  </form>
</template>
