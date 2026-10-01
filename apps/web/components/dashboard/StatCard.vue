<script setup lang="ts">
withDefaults(defineProps<{ label: string; value: number | string; tone?: 'neutral' | 'danger' | 'warning' | 'success' | 'info'; to?: string; help?: string }>(), {
  tone: 'neutral', to: undefined, help: undefined,
})
const tones = {
  neutral: 'border-surface-border',
  danger: 'border-danger',
  warning: 'border-warning',
  success: 'border-success',
  info: 'border-info',
}
</script>

<template>
  <component :is="to ? 'NuxtLink' : 'div'" :to="to" class="card flex flex-col gap-1 border-l-8 text-ink no-underline" :class="tones[tone]" data-testid="stat-card">
    <span class="text-3xl font-bold tabular-nums">{{ value }}</span>
    <span class="font-semibold">{{ label }}</span>
    <span v-if="help" class="text-sm text-ink-muted">{{ help }}</span>
  </component>
</template>
