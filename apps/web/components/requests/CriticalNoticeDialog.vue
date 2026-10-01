<script setup lang="ts">
// Shown before a critical request is submitted. It must never suggest
// delaying a call to emergency services.
defineProps<{ open: boolean; busy?: boolean }>()
const emit = defineEmits<{ confirm: []; cancel: [] }>()
const settings = useSettingsStore()
const notice = computed(() => settings.settings?.critical_urgency_notice ?? settings.emergencyNotice)
</script>

<template>
  <ConfirmDialog
    :open="open"
    :busy="busy"
    danger
    title="Is anyone in immediate danger?"
    confirm-label="No immediate danger - submit request"
    cancel-label="Go back"
    @confirm="emit('confirm')"
    @cancel="emit('cancel')"
  >
    <p class="text-lg font-semibold" data-testid="critical-notice">{{ notice }}</p>
    <AlertBox tone="danger">
      Submitting this request does <strong>not</strong> notify emergency services, the police, the fire brigade or any authority.
    </AlertBox>
  </ConfirmDialog>
</template>
