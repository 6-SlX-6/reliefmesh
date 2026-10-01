<script setup lang="ts">
import type { Urgency } from '@reliefmesh/shared-types'
import { urgency as palette } from '@reliefmesh/ui-tokens'

const props = defineProps<{ urgency: Urgency }>()
const c = computed(() => palette[props.urgency])
</script>

<template>
  <span class="inline-flex items-center gap-1 whitespace-nowrap rounded-md border-2 px-2 py-0.5 text-sm font-bold uppercase tracking-wide" :style="{ background: c.bg, color: c.fg, borderColor: c.border }" data-testid="urgency-badge">
    <AppIcon v-if="urgency === 'critical' || urgency === 'high'" name="alert" :size="14" />
    <span><span class="sr-only">Urgency: </span>{{ urgencyLabel[urgency] }}</span>
  </span>
</template>
