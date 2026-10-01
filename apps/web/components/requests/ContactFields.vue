<script setup lang="ts">
import { CONTACT_METHODS, type ContactInput, type ContactVisibility } from '@reliefmesh/shared-types'

const model = defineModel<ContactInput>({ required: true })
const props = withDefaults(defineProps<{ errors?: Record<string, string>; hasStoredDetails?: boolean }>(), { errors: () => ({}), hasStoredDetails: false })
const visibilities: ContactVisibility[] = ['coordinators_only', 'assigned_responders', 'none']
const visHelp: Record<ContactVisibility, string> = {
  coordinators_only: 'Only coordinators can open your contact details (logged).',
  assigned_responders: 'Coordinators and the responder assigned to help you can open them (logged).',
  none: 'No contact details are stored. You can still follow the status here.',
}
const showDetails = computed(() => model.value.visibility !== 'none' && model.value.method !== 'none')
const err = (k: string) => props.errors[`contact.${k}`]
</script>

<template>
  <fieldset class="flex flex-col gap-3">
    <legend class="mb-1 text-lg font-bold">How can we reach you?</legend>
    <div class="grid grid-cols-1 gap-2">
      <label v-for="v in visibilities" :key="v" class="choice" :data-testid="`contact-${v}`">
        <input v-model="model.visibility" type="radio" name="contact-visibility" :value="v">
        <span class="flex flex-col">
          <span class="font-semibold">{{ contactVisibilityLabel[v] }}</span>
          <span class="text-sm font-normal text-ink-muted">{{ visHelp[v] }}</span>
        </span>
      </label>
    </div>
    <template v-if="model.visibility !== 'none'">
      <FormField v-slot="f" label="Contact method" :error="err('method')">
        <select :id="f.id" v-model="model.method" class="field-input" data-testid="contact-method">
          <option v-for="m in CONTACT_METHODS" :key="m" :value="m">{{ contactMethodLabel[m] }}</option>
        </select>
      </FormField>
      <label v-if="hasStoredDetails && showDetails" class="choice">
        <input v-model="model.keep_details" type="checkbox">
        <span>Keep the contact details that are already stored</span>
      </label>
      <FormField v-if="showDetails && !model.keep_details" v-slot="f" label="Contact details" :error="err('details')"
        help="Only what is needed to reach you, e.g. a phone number or 'ask at the shelter desk'. Encrypted.">
        <input :id="f.id" v-model="model.details" class="field-input" maxlength="300" autocomplete="off" :aria-invalid="f.invalid" data-testid="contact-details">
      </FormField>
    </template>
  </fieldset>
</template>
