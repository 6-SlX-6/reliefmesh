<script setup lang="ts">
import type { NuxtError } from '#app'

const props = defineProps<{ error: NuxtError }>()
const handle = () => clearError({ redirect: '/' })
</script>

<template>
  <div class="mx-auto max-w-xl p-6">
    <EmergencyNotice />
    <main class="card mt-6">
      <h1>Something went wrong</h1>
      <p class="mt-2">{{ props.error.statusCode === 404 ? 'This page does not exist.' : 'An unexpected error occurred.' }}</p>
      <button class="btn-primary mt-4" type="button" @click="handle">Go to start page</button>
    </main>
  </div>
</template>
