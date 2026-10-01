<script setup lang="ts">
const items = useNavigation()
const route = useRoute()
const menuOpen = ref(false)
const primary = computed(() => items.value.filter((i) => i.primary).slice(0, 3))
const isActive = (to: string) => route.path === to || route.path.startsWith(to + '/')
watch(() => route.fullPath, () => { menuOpen.value = false })
</script>

<template>
  <!-- Desktop sidebar -->
  <nav class="no-print hidden w-60 shrink-0 md:block" aria-label="Main">
    <ul class="sticky top-4 flex flex-col gap-1">
      <li v-for="item in items" :key="item.to">
        <NuxtLink
          :to="item.to"
          class="flex min-h-target items-center gap-3 rounded-lg px-3 font-semibold no-underline"
          :class="isActive(item.to) ? 'bg-brand text-brand-ink hover:text-brand-ink' : 'text-ink hover:bg-surface'"
          :aria-current="isActive(item.to) ? 'page' : undefined"
        >
          <AppIcon :name="item.icon" :size="20" /> {{ item.label }}
        </NuxtLink>
      </li>
    </ul>
  </nav>

  <!-- Mobile bottom bar -->
  <nav class="no-print fixed inset-x-0 bottom-0 z-40 border-t-2 border-surface-strong bg-surface md:hidden" aria-label="Main">
    <ul class="grid grid-cols-4">
      <li v-for="item in primary" :key="item.to">
        <NuxtLink
          :to="item.to"
          class="flex min-h-[60px] flex-col items-center justify-center gap-0.5 text-xs font-semibold no-underline"
          :class="isActive(item.to) ? 'bg-brand-soft text-brand' : 'text-ink'"
          :aria-current="isActive(item.to) ? 'page' : undefined"
        >
          <AppIcon :name="item.icon" :size="22" /> {{ item.label }}
        </NuxtLink>
      </li>
      <li>
        <button type="button" class="flex min-h-[60px] w-full flex-col items-center justify-center gap-0.5 text-xs font-semibold text-ink" :aria-expanded="menuOpen" aria-controls="mobile-menu" @click="menuOpen = !menuOpen">
          <AppIcon name="list" :size="22" /> Menu
        </button>
      </li>
    </ul>
    <ul v-if="menuOpen" id="mobile-menu" class="max-h-[60vh] overflow-y-auto border-t border-surface-border p-2">
      <li v-for="item in items" :key="item.to">
        <NuxtLink :to="item.to" class="flex min-h-target items-center gap-3 rounded-lg px-3 font-semibold text-ink no-underline hover:bg-surface-sunken">
          <AppIcon :name="item.icon" :size="20" /> {{ item.label }}
        </NuxtLink>
      </li>
    </ul>
  </nav>
</template>
