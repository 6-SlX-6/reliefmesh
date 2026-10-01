<script setup lang="ts">
const settings = useSettingsStore()
</script>

<template>
  <AlertBox tone="warning" title="Medicine pickup: logistics only" data-testid="medicine-notice">
    {{ settings.settings?.medicine_privacy_notice ?? 'Do not enter diagnoses, prescription details, medication names or medical records.' }}
  </AlertBox>
</template>
