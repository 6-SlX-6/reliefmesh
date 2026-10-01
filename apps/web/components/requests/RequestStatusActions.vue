<script setup lang="ts">
import type { AidRequest, RequestStatus, RequestStatusInput } from '@reliefmesh/shared-types'
import { isApiError } from '@reliefmesh/api-client'

const props = defineProps<{ request: AidRequest }>()
const emit = defineEmits<{ updated: [request: AidRequest]; queued: [] }>()
const sync = useSyncStore()
const toasts = useToastStore()
const errors = useFormErrors()

const target = ref<RequestStatus | 'reopen' | null>(null)
const reason = ref('')
const summary = ref('')
const duplicateRef = ref('')
const cancelActive = ref(false)
const needsCancelConfirm = ref(false)
const busy = ref(false)

const closing = computed(() => target.value && ['resolved', 'cancelled', 'expired', 'duplicate'].includes(target.value))
const destructive = computed(() => target.value === 'cancelled' || target.value === 'expired' || target.value === 'duplicate')
const actions = computed(() => props.request.permissions.allowed_statuses)

function open(s: RequestStatus | 'reopen') {
  target.value = s
  reason.value = ''
  summary.value = ''
  duplicateRef.value = ''
  cancelActive.value = false
  needsCancelConfirm.value = false
  errors.clear()
}

const confirmDisabled = computed(() => {
  if (target.value === 'cancelled' || target.value === 'reopen') return reason.value.trim().length < 3
  if (target.value === 'resolved') return summary.value.trim().length < 3
  if (target.value === 'duplicate') return !duplicateRef.value.trim()
  return false
})

async function confirm() {
  if (!target.value) return
  busy.value = true
  errors.clear()
  try {
    if (target.value === 'reopen') {
      const v = await useApi().requests.reopen(props.request.id, reason.value.trim())
      toasts.success('Request reopened.')
      emit('updated', v)
      target.value = null
      return
    }
    const payload: RequestStatusInput = {
      status: target.value, reason: reason.value.trim(), resolution_summary: summary.value.trim(),
      duplicate_of_reference: duplicateRef.value.trim(), cancel_active_assignments: cancelActive.value, version: props.request.version,
    }
    const isLocal = props.request.id.startsWith('local:')
    const res = await sync.submit({
      type: 'request.status',
      entity_id: isLocal ? undefined : props.request.id,
      entity_client_id: isLocal ? props.request.client_id : undefined,
      payload,
      summary: `${requestStatusAction[target.value] ?? target.value}: ${props.request.reference}`,
    }, { dropOnReject: true })
    if (res.status === 'applied') {
      toasts.success(`Status changed to "${requestStatusLabel[target.value]}".`)
      emit('updated', res.entity as AidRequest)
      target.value = null
    } else if (res.status === 'queued') {
      toasts.info('Status change saved on this device. It will be sent when the connection returns.')
      emit('queued')
      target.value = null
    } else if (res.error?.code === 'active_assignments') {
      needsCancelConfirm.value = true
      errors.message.value = res.error.message
      await sync.discard(res.opId)
    } else {
      errors.fromDetail(res.error)
      if (res.status === 'conflict') await sync.discard(res.opId)
      if (res.entity) emit('updated', res.entity as AidRequest)
    }
  } catch (e) {
    if (isApiError(e)) errors.fromError(e)
    else errors.fromError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section v-if="actions.length || request.permissions.can_reopen" class="card" aria-labelledby="actions-h">
    <h2 id="actions-h" class="mb-3">Next steps</h2>
    <div class="flex flex-wrap gap-2">
      <button
        v-for="s in actions"
        :key="s"
        type="button"
        :class="['cancelled', 'expired', 'duplicate'].includes(s) ? 'btn-secondary' : 'btn-primary'"
        :data-testid="`status-${s}`"
        @click="open(s)"
      >
        {{ requestStatusAction[s] ?? requestStatusLabel[s] }}
      </button>
      <button v-if="request.permissions.can_reopen" type="button" class="btn-secondary" data-testid="reopen" :disabled="!sync.online" @click="open('reopen')">
        <AppIcon name="rotate" :size="18" /> Reopen
      </button>
    </div>

    <ConfirmDialog
      :open="target !== null"
      :title="target === 'reopen' ? 'Reopen this request?' : `${requestStatusAction[target as RequestStatus] ?? ''}?`"
      :confirm-label="target === 'reopen' ? 'Reopen request' : (requestStatusAction[target as RequestStatus] ?? 'Confirm')"
      :danger="destructive"
      :busy="busy"
      :confirm-disabled="confirmDisabled"
      @confirm="confirm"
      @cancel="target = null"
    >
      <p v-if="target && target !== 'reopen'">{{ requestStatusHelp[target] }}</p>
      <AlertBox v-if="errors.message.value" tone="danger">{{ errors.message.value }}</AlertBox>
      <FormField v-if="target === 'resolved' || target === 'partially_resolved'" v-slot="f" :label="target === 'resolved' ? 'What was done?' : 'What has been done so far?'" :required="target === 'resolved'" :error="errors.fields.value.resolution_summary">
        <textarea :id="f.id" v-model="summary" class="field-input" rows="3" maxlength="2000" data-testid="resolution-summary" />
      </FormField>
      <FormField v-if="target === 'duplicate'" v-slot="f" label="Reference of the original request" required help="For example RM-2026-0000123" :error="errors.fields.value.duplicate_of_reference">
        <input :id="f.id" v-model="duplicateRef" class="field-input font-mono" data-testid="duplicate-ref">
      </FormField>
      <FormField v-if="target === 'cancelled' || target === 'expired' || target === 'reopen'" v-slot="f" label="Reason" :required="target !== 'expired'"
        help="Visible in the request history. Do not include personal details." :error="errors.fields.value.reason">
        <textarea :id="f.id" v-model="reason" class="field-input" rows="2" maxlength="1000" data-testid="status-reason" />
      </FormField>
      <label v-if="closing && needsCancelConfirm" class="choice">
        <input v-model="cancelActive" type="checkbox" data-testid="cancel-active">
        <span>Also cancel the active assignments of this request and release their allocated quantities.</span>
      </label>
    </ConfirmDialog>
  </section>
</template>
