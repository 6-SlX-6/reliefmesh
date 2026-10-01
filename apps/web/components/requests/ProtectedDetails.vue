<script setup lang="ts">
// Reveals protected values through the audited API endpoint. The values are
// kept in component memory only - never in IndexedDB, local storage or the
// service worker cache - and are cleared when the component unmounts.
import type { ExactLocation, ProtectedContact } from '@reliefmesh/shared-types'

const props = defineProps<{ kind: 'request' | 'offer' | 'assignment'; id: string; canReveal: boolean }>()
const sync = useSyncStore()
const errors = useFormErrors()
const confirmOpen = ref(false)
const busy = ref(false)
const contact = ref<ProtectedContact | null>(null)
const exact = ref<ExactLocation | null>(null)
const pickup = ref<ExactLocation | null>(null)
const revealed = ref(false)

async function reveal() {
  busy.value = true
  errors.clear()
  try {
    const api = useApi()
    if (props.kind === 'assignment') {
      const r = await api.assignments.revealProtected(props.id)
      contact.value = r.destination_contact ?? null
      exact.value = r.destination_location ?? null
      pickup.value = r.pickup_location ?? null
    } else {
      const r = props.kind === 'request' ? await api.requests.revealProtected(props.id) : await api.offers.revealProtected(props.id)
      contact.value = r.contact ?? null
      exact.value = r.exact_location ?? null
    }
    revealed.value = true
    confirmOpen.value = false
  } catch (e) {
    errors.fromError(e)
  } finally {
    busy.value = false
  }
}

function hide() {
  contact.value = exact.value = pickup.value = null
  revealed.value = false
}
onBeforeUnmount(hide)
</script>

<template>
  <section v-if="canReveal" class="card border-2 border-brand" aria-labelledby="prot-h" data-testid="protected-details">
    <h2 id="prot-h" class="mb-2 flex items-center gap-2"><AppIcon name="lock" /> Protected details</h2>
    <template v-if="!revealed">
      <p class="mb-3">Contact details and exact locations are encrypted. Opening them is recorded in the history{{ kind === 'request' ? ' and visible to the requester' : '' }}.</p>
      <button type="button" class="btn-primary" :disabled="!sync.online" data-testid="reveal-protected" @click="confirmOpen = true">
        <AppIcon name="eye" :size="18" /> Show protected details
      </button>
      <p v-if="!sync.online" class="field-help">Needs a connection to the server.</p>
    </template>
    <template v-else>
      <dl class="kv">
        <div v-if="contact"><dt>Contact ({{ contactMethodLabel[contact.method] }})</dt><dd class="font-semibold" data-testid="revealed-contact">{{ contact.details }}</dd></div>
        <div v-if="exact"><dt>{{ kind === 'assignment' ? 'Destination' : 'Exact location' }}</dt><dd>
          <span v-if="exact.address" class="block whitespace-pre-line font-semibold" data-testid="revealed-address">{{ exact.address }}</span>
          <span v-if="exact.directions" class="block whitespace-pre-line">{{ exact.directions }}</span>
          <span v-if="exact.lat != null" class="block font-mono text-sm">{{ exact.lat }}, {{ exact.lon }}</span>
        </dd></div>
        <div v-if="pickup"><dt>Pickup location</dt><dd>
          <span v-if="pickup.address" class="block whitespace-pre-line font-semibold">{{ pickup.address }}</span>
          <span v-if="pickup.directions" class="block whitespace-pre-line">{{ pickup.directions }}</span>
        </dd></div>
      </dl>
      <p class="mt-3 text-sm text-ink-muted">ReliefMesh does not provide routes and cannot tell whether a place is safe to reach. Use your own judgement and local guidance.</p>
      <button type="button" class="btn-secondary mt-3" @click="hide">Hide again</button>
    </template>
    <ConfirmDialog :open="confirmOpen" title="Open protected details?" confirm-label="Show details" :busy="busy" @confirm="reveal" @cancel="confirmOpen = false">
      <p>This access will be logged with your name and time. Only open protected details when you need them for your task.</p>
      <AlertBox v-if="errors.message.value" tone="danger">{{ errors.message.value }}</AlertBox>
    </ConfirmDialog>
  </section>
</template>
