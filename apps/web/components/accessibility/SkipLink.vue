<script setup lang="ts">
// Moves keyboard focus to the main region without changing the URL (a hash
// change would otherwise be treated as a navigation by the router).
function skip(e: MouseEvent) {
  const main = document.getElementById('main')
  if (!main) return
  e.preventDefault()
  main.focus()
  main.scrollIntoView()
}
</script>

<template>
  <a href="#main" class="sr-only-focusable fixed left-2 top-2 z-[100] rounded-lg bg-surface px-4 py-3 font-bold text-brand shadow-lg" @click="skip">
    Skip to main content
  </a>
</template>
