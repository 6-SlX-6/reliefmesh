<script setup lang="ts">
import type { Availability, Volunteer } from '@reliefmesh/shared-types'

definePageMeta({ requires: 'volunteer.list' })
useHead({ title: 'Volunteers' })
const auth = useAuthStore()
const toasts = useToastStore()
const items = ref<Volunteer[]>([])
const error = ref('')
const loading = ref(true)
const canManage = computed(() => auth.can('volunteer.availability.manage'))

async function load() {
  loading.value = true
  try {
    items.value = (await useApi().volunteers.list()).items
    error.value = ''
  } catch {
    error.value = 'The volunteer list needs a connection to the server.'
  } finally {
    loading.value = false
  }
}
async function setAvailability(v: Volunteer, availability: Availability) {
  try {
    await useApi().volunteers.setAvailability(v.id, availability, v.availability_note)
    v.availability = availability
    toasts.success(`${v.display_name}: ${availabilityLabel[availability]}.`)
  } catch {
    toasts.error('Could not change availability.')
  }
}
onMounted(load)
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="Volunteers" />
    <p class="text-ink-muted">Display names and availability only. Volunteers set their own availability; organization managers can adjust it.</p>
    <AlertBox v-if="error" tone="warning">{{ error }}</AlertBox>
    <LoadingState v-if="loading && !items.length" />
    <EmptyState v-else-if="!items.length && !error" title="No volunteers yet" icon="users">An administrator can create volunteer accounts.</EmptyState>
    <div v-else-if="items.length" class="card overflow-x-auto">
      <table class="w-full text-left">
        <thead><tr class="border-b-2 border-surface-strong"><th class="py-2 pr-4">Name</th><th class="py-2 pr-4">Availability</th><th class="py-2 pr-4">Note</th><th class="py-2 text-right">Active tasks</th></tr></thead>
        <tbody>
          <tr v-for="v in items" :key="v.id" class="border-b border-surface-border">
            <td class="py-2 pr-4 font-semibold">{{ v.display_name }}</td>
            <td class="py-2 pr-4">
              <select v-if="canManage" :value="v.availability" class="field-input min-h-[40px] py-1" :aria-label="`Availability of ${v.display_name}`" @change="setAvailability(v, ($event.target as HTMLSelectElement).value as Availability)">
                <option v-for="(label, key) in availabilityLabel" :key="key" :value="key">{{ label }}</option>
              </select>
              <span v-else>{{ availabilityLabel[v.availability] }}</span>
            </td>
            <td class="py-2 pr-4 text-sm">{{ v.availability_note }}</td>
            <td class="py-2 text-right tabular-nums">{{ v.active_assignments }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
