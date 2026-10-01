<script setup lang="ts">
withDefaults(defineProps<{ title: string; back?: string; backLabel?: string }>(), { back: undefined, backLabel: 'Back' })
</script>

<template>
  <header class="mb-5 flex flex-col gap-3">
    <NuxtLink v-if="back" :to="back" class="inline-flex w-fit items-center gap-1 font-semibold no-underline">
      <AppIcon name="chevronLeft" :size="18" /> {{ backLabel }}
    </NuxtLink>
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h1 class="break-words">{{ title }}</h1>
        <div class="mt-2 flex flex-wrap items-center gap-2"><slot name="meta" /></div>
      </div>
      <div class="flex flex-wrap gap-2"><slot name="actions" /></div>
    </div>
  </header>
</template>
