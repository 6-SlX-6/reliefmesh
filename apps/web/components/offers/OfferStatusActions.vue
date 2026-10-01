<script setup lang="ts">
import type { Offer, OfferStatus } from '@reliefmesh/shared-types'

const props = defineProps<{ offer: Offer }>()
const emit = defineEmits<{ updated: [offer: Offer]; queued: [] }>()
const sync = useSyncStore()
const toasts = useToastStore()
const errors = useFormErrors()
const target = ref<OfferStatus | null>(null)
const reason = ref('')
const busy = ref(false)

async function confirm() {
  if (!target.value) return
  busy.value = true
  errors.clear()
  const isLocal = props.offer.id.startsWith('local:')
  try {
    const res = await sync.submit({
      type: 'offer.status', entity_id: isLocal ? undefined : props.offer.id, entity_client_id: isLocal ? props.offer.client_id : undefined,
      payload: { status: target.value, reason: reason.value.trim(), version: props.offer.version },
      summary: `${offerStatusAction[target.value] ?? target.value}: ${props.offer.reference}`,
    }, { dropOnReject: true })
    if (res.status === 'applied') {
      toasts.success('Offer updated.')
      emit('updated', res.entity as Offer)
      target.value = null
    } else if (res.status === 'queued') {
      toasts.info('Saved on this device. It will be sent when the connection returns.')
      emit('queued')
      target.value = null
    } else {
      errors.fromDetail(res.error)
      if (res.status === 'conflict') await sync.discard(res.opId)
    }
  } catch (e) {
    errors.fromError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="offer.permissions.allowed_statuses.length" class="flex flex-wrap gap-2">
    <button v-for="s in offer.permissions.allowed_statuses" :key="s" type="button" :class="s === 'available' ? 'btn-primary' : 'btn-secondary'" :data-testid="`offer-status-${s}`" @click="target = s; reason = ''; errors.clear()">
      {{ offerStatusAction[s] ?? offerStatusLabel[s] }}
    </button>
    <ConfirmDialog :open="target !== null" :title="`${offerStatusAction[target as OfferStatus] ?? ''}?`" :danger="target === 'cancelled' || target === 'expired'"
      :busy="busy" :confirm-disabled="target === 'cancelled' && reason.trim().length < 3" @confirm="confirm" @cancel="target = null">
      <AlertBox v-if="errors.message.value" tone="danger">{{ errors.message.value }}</AlertBox>
      <FormField v-if="target === 'cancelled' || target === 'expired' || target === 'paused'" v-slot="f" label="Reason" :required="target === 'cancelled'">
        <textarea :id="f.id" v-model="reason" class="field-input" rows="2" maxlength="1000" />
      </FormField>
    </ConfirmDialog>
  </div>
</template>
