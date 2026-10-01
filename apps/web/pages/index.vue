<script setup lang="ts">
const auth = useAuthStore()
await navigateTo(auth.homePath, { replace: true })
</script>

<template>
  <LoadingState />
</template>
