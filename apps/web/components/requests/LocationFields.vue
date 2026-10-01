<script setup lang="ts">
import { LOCATION_MODES, type LocationInput } from '@reliefmesh/shared-types'

const model = defineModel<LocationInput>({ required: true })
const props = withDefaults(defineProps<{ errors?: Record<string, string>; hasStoredExact?: boolean; legend?: string }>(), {
  errors: () => ({}), hasStoredExact: false, legend: 'Where is it needed?',
})
const settings = useSettingsStore()
const geoBusy = ref(false)
const geoError = ref('')
const precisionText = computed(() => `about ${precisionKm(settings.approxDecimals).toLocaleString(undefined, { maximumFractionDigits: 1 })} km`)

function setMode(mode: LocationInput['mode']) {
  model.value = { ...model.value, mode }
}

async function useMyLocation() {
  geoBusy.value = true
  geoError.value = ''
  try {
    const p = await currentApproximatePosition(settings.approxDecimals)
    model.value = { ...model.value, lat: p.lat, lon: p.lon }
  } catch (e) {
    geoError.value = (e as Error).message
  } finally {
    geoBusy.value = false
  }
}

const exact = computed({
  get: () => model.value.exact ?? {},
  set: (v) => { model.value = { ...model.value, exact: v, keep_exact: false } },
})

function numberOrNull(v: string): number | null {
  const n = parseFloat(v)
  return Number.isFinite(n) ? n : null
}
const err = (k: string) => props.errors[`location.${k}`]
</script>

<template>
  <fieldset class="flex flex-col gap-3">
    <legend class="mb-1 text-lg font-bold">{{ legend }}</legend>
    <p class="field-help">Share only as much location detail as needed. Exact addresses are encrypted and every access is logged.</p>
    <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
      <label v-for="m in LOCATION_MODES" :key="m" class="choice" :data-testid="`location-${m}`">
        <input :checked="model.mode === m" type="radio" name="location-mode" :value="m" @change="setMode(m)">
        <span class="flex flex-col">
          <span class="font-semibold">{{ locationModeLabel[m] }}</span>
          <span class="text-sm font-normal text-ink-muted">{{ locationModeHelp[m] }}</span>
        </span>
      </label>
    </div>
    <p v-if="err('mode')" class="field-error">{{ err('mode') }}</p>

    <FormField v-if="model.mode !== 'none'" v-slot="f" label="Area" :required="model.mode === 'area_only'" :error="err('area_label')"
      help="District, village, street area or shelter name. Avoid house numbers here.">
      <input :id="f.id" v-model="model.area_label" class="field-input" maxlength="160" :aria-describedby="f.describedBy" :aria-invalid="f.invalid" data-testid="area-label">
    </FormField>

    <div v-if="model.mode === 'approximate'" class="flex flex-col gap-3 rounded-lg border-2 border-dashed border-surface-border p-3">
      <p class="text-sm">Coordinates are rounded to {{ precisionText }} before they are saved.</p>
      <div class="grid grid-cols-2 gap-3">
        <FormField v-slot="f" label="Latitude" :error="err('lat')">
          <input :id="f.id" :value="model.lat ?? ''" class="field-input" inputmode="decimal" :aria-invalid="f.invalid" @input="model = { ...model, lat: numberOrNull(($event.target as HTMLInputElement).value) }">
        </FormField>
        <FormField v-slot="f" label="Longitude">
          <input :id="f.id" :value="model.lon ?? ''" class="field-input" inputmode="decimal" @input="model = { ...model, lon: numberOrNull(($event.target as HTMLInputElement).value) }">
        </FormField>
      </div>
      <button type="button" class="btn-secondary w-fit" :disabled="geoBusy" @click="useMyLocation">
        <AppIcon name="pin" :size="18" /> {{ geoBusy ? 'Locating...' : 'Use my approximate location' }}
      </button>
      <p v-if="geoError" class="field-error">{{ geoError }}</p>
    </div>

    <div v-if="model.mode === 'protected_exact'" class="flex flex-col gap-3 rounded-lg border-2 border-brand bg-brand-soft p-3">
      <p class="flex items-start gap-2 text-sm font-semibold"><AppIcon name="lock" :size="18" /> Encrypted. Only coordinators and responders explicitly granted access can open it. You will see in the history when it was opened.</p>
      <label v-if="hasStoredExact" class="choice">
        <input :checked="model.keep_exact" type="checkbox" @change="model = { ...model, keep_exact: ($event.target as HTMLInputElement).checked, exact: undefined }">
        <span>Keep the exact location that is already stored</span>
      </label>
      <template v-if="!model.keep_exact">
        <FormField v-slot="f" label="Address" :error="err('exact') || err('exact.address')" help="Street, house number, entrance or room.">
          <textarea :id="f.id" :value="exact.address ?? ''" class="field-input" rows="2" maxlength="300" :aria-invalid="f.invalid" data-testid="exact-address" @input="exact = { ...exact, address: ($event.target as HTMLTextAreaElement).value }" />
        </FormField>
        <FormField v-slot="f" label="Directions (optional)" help="E.g. 'blue door at the back'. No health information.">
          <textarea :id="f.id" :value="exact.directions ?? ''" class="field-input" rows="2" maxlength="500" @input="exact = { ...exact, directions: ($event.target as HTMLTextAreaElement).value }" />
        </FormField>
      </template>
    </div>
  </fieldset>
</template>
