<script setup lang="ts">
withDefaults(defineProps<{ label?: string }>(), { label: 'Loading...' })
</script>

<template>
  <div class="flex items-center gap-3 p-6 text-ink-muted" role="status" aria-live="polite">
    <AppIcon name="refresh" :size="22" class="animate-spin" />
    <span>{{ label }}</span>
  </div>
</template>
