<script setup lang="ts">
// Always-visible emergency disclaimer. The wording is configured per
// deployment by administrators because emergency numbers differ by country.
withDefaults(defineProps<{ compact?: boolean }>(), { compact: false })
const settings = useSettingsStore()
</script>

<template>
  <aside
    class="flex items-start gap-3 border-l-8 border-danger bg-danger-soft text-ink"
    :class="compact ? 'px-3 py-2 text-sm' : 'rounded-lg p-4'"
    aria-label="Emergency notice"
    data-testid="emergency-notice"
  >
    <AppIcon name="alert" :size="compact ? 18 : 24" class="mt-0.5 text-danger" />
    <p class="font-semibold">
      {{ settings.emergencyNotice }}
      <NuxtLink v-if="!compact" to="/about" class="ml-1 font-normal">About ReliefMesh</NuxtLink>
    </p>
  </aside>
</template>
