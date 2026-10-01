<script setup lang="ts">
import { URGENCIES, type Urgency } from '@reliefmesh/shared-types'

const model = defineModel<Urgency>({ required: true })
defineProps<{ error?: string }>()
</script>

<template>
  <fieldset>
    <legend class="mb-1 text-lg font-bold">How urgent is it? <span class="text-danger" aria-hidden="true">*</span></legend>
    <p class="field-help mb-2">Choose honestly. A coordinator reviews every request. ReliefMesh never sets urgency automatically.</p>
    <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
      <label v-for="u in URGENCIES" :key="u" class="choice" :data-testid="`urgency-${u}`">
        <input v-model="model" type="radio" name="urgency" :value="u">
        <span class="flex flex-col gap-1">
          <UrgencyBadge :urgency="u" />
          <span class="text-sm font-normal">{{ urgencyHelp[u] }}</span>
        </span>
      </label>
    </div>
    <p v-if="error" class="field-error" role="alert">{{ error }}</p>
  </fieldset>
</template>
