<script setup lang="ts">
// Accessible confirmation dialog based on the native <dialog> element, which
// provides focus trapping, Escape handling and inertness of the page.
const props = withDefaults(defineProps<{
  open: boolean
  title: string
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
  busy?: boolean
  confirmDisabled?: boolean
}>(), { confirmLabel: 'Confirm', cancelLabel: 'Cancel', danger: false, busy: false, confirmDisabled: false })

const emit = defineEmits<{ confirm: []; cancel: [] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const titleId = `dlg-${Math.random().toString(36).slice(2)}`

watch(() => props.open, (open) => {
  const d = dialog.value
  if (!d) return
  if (open && !d.open) d.showModal()
  if (!open && d.open) d.close()
}, { flush: 'post' })

onMounted(() => {
  if (props.open) dialog.value?.showModal()
})

function onCancel(e: Event) {
  e.preventDefault()
  if (!props.busy) emit('cancel')
}
</script>

<template>
  <dialog
    ref="dialog"
    class="w-[min(40rem,calc(100vw-2rem))] rounded-xl border-2 border-surface-strong bg-surface p-0 text-ink shadow-2xl backdrop:bg-black/60"
    :aria-labelledby="titleId"
    @cancel="onCancel"
  >
    <form method="dialog" class="flex flex-col gap-4 p-5" @submit.prevent="emit('confirm')">
      <h2 :id="titleId" class="flex items-center gap-2">
        <AppIcon v-if="danger" name="alert" :size="24" class="text-danger" />
        {{ title }}
      </h2>
      <div class="flex flex-col gap-4">
        <slot />
      </div>
      <div class="flex flex-col-reverse gap-3 border-t border-surface-border pt-4 sm:flex-row sm:justify-end">
        <button type="button" class="btn-secondary" :disabled="busy" @click="emit('cancel')">{{ cancelLabel }}</button>
        <button type="submit" :class="danger ? 'btn-danger' : 'btn-primary'" :disabled="busy || confirmDisabled" data-testid="confirm-button">
          <AppIcon v-if="busy" name="refresh" :size="18" class="animate-spin" />
          {{ confirmLabel }}
        </button>
      </div>
    </form>
  </dialog>
</template>
