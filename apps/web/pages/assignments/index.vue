<script setup lang="ts">
import type { Assignment } from '@reliefmesh/shared-types'

definePageMeta({ requires: ['assignment.work_own', 'assignment.manage'] })
const auth = useAuthStore()
const coordinator = computed(() => auth.can('assignment.manage'))
useHead({ title: coordinator.value ? 'Assignments' : 'My tasks' })
const activeOnly = ref(true)
const mine = ref(!coordinator.value)
const isActive = (a: Assignment) => ['proposed', 'accepted', 'in_progress', 'partially_delivered'].includes(a.status)
const list = useCachedList<Assignment>('assignments',
  async () => (await useApi().assignments.list({ active: activeOnly.value || undefined, mine: mine.value || undefined, limit: 300 })).items,
  (a) => (!activeOnly.value || isActive(a)) && (!mine.value || a.volunteer?.id === auth.user?.id))
onMounted(list.load)
watch([activeOnly, mine], () => void list.load())

async function onUpdated(a: Assignment) {
  const i = list.items.value.findIndex((x) => x.id === a.id)
  if (i >= 0) list.items.value[i] = a
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader :title="coordinator ? 'Assignments' : 'My tasks'" />
    <AlertBox v-if="!coordinator" tone="info">
      These are the tasks a coordinator assigned to you. Accept or decline each one, then mark it as started and delivered.
      ReliefMesh does not give directions: use your own judgement and follow local guidance on safety.
    </AlertBox>
    <div class="flex flex-wrap gap-4">
      <label class="flex items-center gap-2 font-normal"><input v-model="activeOnly" type="checkbox" class="h-5 w-5"> Active only</label>
      <label v-if="coordinator && auth.isVolunteer" class="flex items-center gap-2 font-normal"><input v-model="mine" type="checkbox" class="h-5 w-5"> Only mine</label>
    </div>
    <OfflineDataNotice :show="list.fromCache.value" :cached-at="list.cachedAt.value" />
    <AlertBox v-if="list.error.value" tone="danger">{{ list.error.value }}</AlertBox>
    <LoadingState v-if="list.loading.value && !list.items.value.length" />
    <EmptyState v-else-if="!list.items.value.length" title="No assignments" icon="truck">
      <span v-if="!coordinator">You have no {{ activeOnly ? 'active ' : '' }}tasks right now.</span>
    </EmptyState>
    <ul v-else class="flex flex-col gap-3" data-testid="assignment-list">
      <AssignmentCard v-for="a in list.items.value" :key="a.id" :assignment="a" show-request @updated="onUpdated" @queued="list.load()" />
    </ul>
  </div>
</template>
