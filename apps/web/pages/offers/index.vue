<script setup lang="ts">
import { OFFER_STATUSES, type Offer } from '@reliefmesh/shared-types'

definePageMeta({ requires: ['offer.create', 'offer.read_all'] })
const auth = useAuthStore()
const settings = useSettingsStore()
const coordinator = computed(() => auth.can('offer.read_all'))
useHead({ title: coordinator.value ? 'Offers' : 'My offers' })
const filters = reactive({ status: '', category: '', allocatable: false })
const matches = (o: Offer) => (!filters.status || o.status === filters.status) && (!filters.category || o.category === filters.category) &&
  (!filters.allocatable || (['available', 'partially_allocated'].includes(o.status) && o.remaining_quantity > 0))
const list = useCachedList<Offer>('offers', async () => (await useApi().offers.list({ ...filters, allocatable: filters.allocatable || undefined, limit: 300 })).items, matches)
onMounted(list.load)
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader :title="coordinator ? 'Offers' : 'My offers'">
      <template #actions>
        <NuxtLink to="/offers/new" class="btn-primary text-lg" data-testid="new-offer"><AppIcon name="plus" /> New offer</NuxtLink>
      </template>
    </PageHeader>
    <form class="card grid grid-cols-1 gap-3 sm:grid-cols-4" @submit.prevent="list.load()">
      <FormField v-slot="f" label="Status">
        <select :id="f.id" v-model="filters.status" class="field-input"><option value="">Any</option><option v-for="s in OFFER_STATUSES" :key="s" :value="s">{{ offerStatusLabel[s] }}</option></select>
      </FormField>
      <FormField v-slot="f" label="Category">
        <select :id="f.id" v-model="filters.category" class="field-input"><option value="">Any</option><option v-for="c in settings.categories" :key="c.code" :value="c.code">{{ c.label }}</option></select>
      </FormField>
      <label class="flex items-center gap-2 font-normal"><input v-model="filters.allocatable" type="checkbox" class="h-5 w-5"> Only with quantity left</label>
      <div class="flex items-end"><button type="submit" class="btn-secondary w-full">Apply filters</button></div>
    </form>
    <OfflineDataNotice :show="list.fromCache.value" :cached-at="list.cachedAt.value" />
    <AlertBox v-if="list.error.value" tone="danger">{{ list.error.value }}</AlertBox>
    <ul v-if="list.pendingItems.value.length" class="flex flex-col gap-3"><OfferCard v-for="p in list.pendingItems.value" :key="p.id" :offer="p.data" pending /></ul>
    <LoadingState v-if="list.loading.value && !list.items.value.length" />
    <EmptyState v-else-if="!list.items.value.length && !list.pendingItems.value.length" title="No offers yet" icon="package">Use "New offer" to register resources that can be allocated.</EmptyState>
    <ul v-else class="flex flex-col gap-3" data-testid="offer-list"><OfferCard v-for="o in list.items.value" :key="o.id" :offer="o" /></ul>
  </div>
</template>
