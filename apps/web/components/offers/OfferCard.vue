<script setup lang="ts">
import type { Offer } from '@reliefmesh/shared-types'

defineProps<{ offer: Offer; pending?: boolean }>()
const settings = useSettingsStore()
</script>

<template>
  <li class="card flex flex-col gap-2 hover:border-brand" data-testid="offer-card">
    <div class="flex flex-wrap items-center gap-2">
      <StatusBadge kind="offer" :status="offer.status" :pending="pending" />
      <span v-if="offer.verification_level" class="text-sm text-ink-muted">{{ verificationLabel[offer.verification_level] }}</span>
    </div>
    <NuxtLink :to="`/offers/${offer.id}`" class="text-lg font-bold text-ink no-underline hover:underline">{{ offer.title }}</NuxtLink>
    <p class="flex flex-wrap gap-x-4 gap-y-1 text-sm text-ink-muted">
      <span class="font-mono">{{ offer.reference }}</span>
      <span>{{ settings.categoryLabel(offer.category) }}</span>
      <span class="font-semibold text-ink">{{ offer.remaining_quantity }} of {{ offer.quantity_available }} {{ offer.unit }} available</span>
      <span>{{ deliveryModeLabel[offer.pickup_or_delivery_mode] }}</span>
      <span v-if="offer.location.area_label" class="inline-flex items-center gap-1"><AppIcon name="pin" :size="14" />{{ offer.location.area_label }}</span>
    </p>
  </li>
</template>
