<script setup lang="ts">
// Offers a reload when a new app version has been downloaded. The update is
// never applied automatically so nobody loses a half-filled form.
const { $pwa } = useNuxtApp()
const needRefresh = computed(() => Boolean(($pwa as { needRefresh?: boolean } | undefined)?.needRefresh))
const offlineReady = computed(() => Boolean(($pwa as { offlineReady?: boolean } | undefined)?.offlineReady))
const dismissedReady = ref(false)
function update() {
  void ($pwa as { updateServiceWorker?: (reload?: boolean) => Promise<void> } | undefined)?.updateServiceWorker?.(true)
}
</script>

<template>
  <div v-if="needRefresh" class="no-print flex flex-wrap items-center justify-center gap-3 bg-info-soft px-4 py-2 text-sm font-semibold" role="status">
    A new version of ReliefMesh is available.
    <button type="button" class="btn-primary btn-sm" @click="update">Reload now</button>
  </div>
  <div v-else-if="offlineReady && !dismissedReady" class="no-print flex flex-wrap items-center justify-center gap-3 bg-success-soft px-4 py-2 text-sm font-semibold" role="status">
    ReliefMesh is ready to work offline on this device.
    <button type="button" class="btn-ghost btn-sm" @click="dismissedReady = true">OK</button>
  </div>
</template>
