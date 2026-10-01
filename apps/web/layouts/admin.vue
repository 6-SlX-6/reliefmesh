<script setup lang="ts">
const auth = useAuthStore()
const route = useRoute()
const tabs = [
  { to: '/admin', label: 'Overview' },
  { to: '/admin/users', label: 'Users' },
  { to: '/admin/settings', label: 'Settings & notices' },
  { to: '/admin/categories', label: 'Categories' },
  { to: '/admin/audit', label: 'Audit log' },
  { to: '/admin/data', label: 'Data & retention' },
]
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <SkipLink />
    <ExerciseBanner />
    <EmergencyNotice compact />
    <AppHeader />
    <div class="border-b-4 border-ink bg-ink text-ink-inverse">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 px-4 py-3">
        <p class="flex items-center gap-2 font-bold"><AppIcon name="shield" /> Administration</p>
        <p class="text-sm">Instance administration only. Operational personal data is not shown here.</p>
      </div>
    </div>
    <div class="mx-auto w-full max-w-6xl flex-1 px-4 pb-28 pt-5 md:pb-10">
      <nav aria-label="Administration" class="no-print mb-5 overflow-x-auto">
        <ul class="flex gap-2">
          <li v-for="t in tabs" :key="t.to">
            <NuxtLink :to="t.to" class="btn-sm btn whitespace-nowrap" :class="route.path === t.to ? 'border-ink bg-ink text-ink-inverse hover:text-ink-inverse' : 'border-surface-border bg-surface text-ink'" :aria-current="route.path === t.to ? 'page' : undefined">
              {{ t.label }}
            </NuxtLink>
          </li>
          <li v-if="!auth.adminOnly">
            <NuxtLink :to="auth.isCoordinator ? '/dashboard' : '/requests'" class="btn-sm btn-ghost whitespace-nowrap">Back to operations</NuxtLink>
          </li>
        </ul>
      </nav>
      <main id="main" tabindex="-1">
        <slot />
      </main>
    </div>
    <AppNav v-if="auth.adminOnly" class="md:hidden" />
    <ToastHost />
  </div>
</template>
