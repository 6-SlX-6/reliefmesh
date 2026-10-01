<script setup lang="ts">
import type { CategoryCode } from '@reliefmesh/shared-types'

const model = defineModel<CategoryCode | ''>({ required: true })
defineProps<{ error?: string }>()
const settings = useSettingsStore()
const icons: Record<string, string> = {
  drinking_water: 'cloud', food: 'package', shelter: 'home', blankets: 'package', hygiene: 'package',
  baby_supplies: 'heart', power_charging: 'activity', transport: 'truck', volunteer_support: 'users',
  translation: 'message', accessibility_support: 'user', information: 'info', pet_animal_support: 'heart',
  medicine_pickup: 'lock', other: 'circle',
}
</script>

<template>
  <fieldset>
    <legend class="mb-2 text-lg font-bold">What is needed? <span class="text-danger" aria-hidden="true">*</span></legend>
    <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3" role="radiogroup">
      <label v-for="c in settings.categories" :key="c.code" class="choice" :data-testid="`category-${c.code}`">
        <input v-model="model" type="radio" name="category" :value="c.code">
        <span class="flex flex-col">
          <span class="flex items-center gap-2 font-semibold"><AppIcon :name="icons[c.code] ?? 'circle'" :size="18" /> {{ c.label }}</span>
          <span class="text-sm font-normal text-ink-muted">{{ c.description }}</span>
        </span>
      </label>
    </div>
    <p v-if="!settings.categories.length" class="field-help">Categories are loading. If this persists, check the connection.</p>
    <p v-if="error" class="field-error" role="alert">{{ error }}</p>
  </fieldset>
</template>
