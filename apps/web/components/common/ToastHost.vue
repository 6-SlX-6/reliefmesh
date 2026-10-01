<script setup lang="ts">
const toasts = useToastStore()
const styles = {
  success: 'border-success bg-success-soft',
  info: 'border-info bg-info-soft',
  warning: 'border-warning bg-warning-soft',
  error: 'border-danger bg-danger-soft',
}
const icons = { success: 'check', info: 'info', warning: 'alert', error: 'alert' }
</script>

<template>
  <div class="pointer-events-none fixed inset-x-0 bottom-20 z-50 flex flex-col items-center gap-2 px-4 md:bottom-6" aria-live="polite" aria-atomic="false">
    <div
      v-for="t in toasts.items"
      :key="t.id"
      class="pointer-events-auto flex w-full max-w-lg items-start gap-3 rounded-lg border-l-8 p-4 text-ink shadow-lg"
      :class="styles[t.kind]"
      :role="t.kind === 'error' ? 'alert' : 'status'"
      data-testid="toast"
    >
      <AppIcon :name="icons[t.kind]" :size="20" class="mt-0.5" />
      <p class="flex-1 font-semibold">{{ t.message }}</p>
      <button type="button" class="btn-ghost btn-sm -my-1" @click="toasts.dismiss(t.id)">
        <AppIcon name="x" :size="16" /><span class="sr-only">Dismiss message</span>
      </button>
    </div>
  </div>
</template>
