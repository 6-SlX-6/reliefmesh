<script setup lang="ts">
const settings = useSettingsStore()
</script>

<template>
  <div
    v-if="settings.exerciseMode"
    class="bg-exercise px-4 py-1.5 text-center text-sm font-bold uppercase tracking-wide text-exercise-ink"
    role="note"
    data-testid="exercise-banner"
  >
    {{ settings.exerciseLabel }}
  </div>
</template>
