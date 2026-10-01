<script setup lang="ts">
withDefaults(defineProps<{ icon?: string; title: string }>(), { icon: 'inbox' })
</script>

<template>
  <div class="card flex flex-col items-center gap-3 py-10 text-center">
    <AppIcon :name="icon" :size="40" class="text-ink-subtle" />
    <p class="text-lg font-semibold">{{ title }}</p>
    <div class="text-ink-muted"><slot /></div>
  </div>
</template>
