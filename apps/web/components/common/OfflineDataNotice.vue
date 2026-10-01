<script setup lang="ts">
defineProps<{ show: boolean; cachedAt?: string | null }>()
</script>

<template>
  <AlertBox v-if="show" tone="warning" title="Showing saved data" data-testid="offline-data-notice">
    The server cannot be reached. This is the information saved on this device<span v-if="cachedAt"> ({{ formatRelative(cachedAt) }})</span>.
    It may be out of date. Changes you make are saved and sent automatically when the connection returns.
  </AlertBox>
</template>
