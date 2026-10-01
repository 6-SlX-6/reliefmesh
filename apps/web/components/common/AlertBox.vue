<script setup lang="ts">
const props = withDefaults(defineProps<{ tone?: 'info' | 'warning' | 'danger' | 'success'; title?: string }>(), { tone: 'info', title: undefined })
const styles = {
  info: { cls: 'border-info bg-info-soft', icon: 'info' },
  warning: { cls: 'border-warning bg-warning-soft', icon: 'alert' },
  danger: { cls: 'border-danger bg-danger-soft', icon: 'alert' },
  success: { cls: 'border-success bg-success-soft', icon: 'check' },
}
const s = computed(() => styles[props.tone])
</script>

<template>
  <div class="flex items-start gap-3 rounded-lg border-l-8 p-4 text-ink" :class="s.cls" :role="tone === 'danger' ? 'alert' : 'status'">
    <AppIcon :name="s.icon" :size="22" class="mt-0.5" />
    <div class="min-w-0 flex-1">
      <p v-if="title" class="font-bold">{{ title }}</p>
      <div class="text-base"><slot /></div>
    </div>
  </div>
</template>
