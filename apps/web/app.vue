<script setup lang="ts">
const auth = useAuthStore()
const settings = useSettingsStore()
const sync = useSyncStore()
const route = useRoute()

useHead({
  titleTemplate: (title) => (title ? `${title} - ${settings.instanceName}` : settings.instanceName),
})

onMounted(async () => {
  await settings.loadPublic()
})

watch(
  () => auth.status,
  async (status) => {
    if (status === 'authenticated') {
      await settings.load()
      sync.start()
    }
  },
  { immediate: true },
)

// Move focus to the main heading after navigation so screen reader and
// keyboard users know the page changed.
watch(
  () => route.fullPath,
  async () => {
    await nextTick()
    const h1 = document.querySelector<HTMLElement>('main h1')
    if (h1) {
      h1.setAttribute('tabindex', '-1')
      h1.focus({ preventScroll: false })
    }
  },
)
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
