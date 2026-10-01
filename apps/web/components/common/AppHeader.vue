<script setup lang="ts">
const auth = useAuthStore()
const settings = useSettingsStore()
</script>

<template>
  <header class="no-print border-b-2 border-surface-strong bg-surface">
    <div class="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-4 py-3">
      <NuxtLink :to="auth.homePath" class="flex items-center gap-2 text-ink no-underline">
        <img src="/favicon.svg" alt="" width="32" height="32" class="rounded-md">
        <span class="text-lg font-bold">{{ settings.instanceName }}</span>
      </NuxtLink>
      <div class="flex flex-wrap items-center gap-3">
        <ConnectionStatus />
        <NuxtLink v-if="auth.user" to="/account" class="hidden items-center gap-1 text-sm font-semibold text-ink no-underline sm:inline-flex">
          <AppIcon name="user" :size="16" /> {{ auth.user.display_name }}
        </NuxtLink>
      </div>
    </div>
  </header>
</template>
