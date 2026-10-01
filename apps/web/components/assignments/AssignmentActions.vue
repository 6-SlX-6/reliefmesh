<script setup lang="ts">
import { EVIDENCE_TYPES, type Assignment, type AssignmentStatus, type AssignmentStatusInput, type EvidenceType, type HandoverStatus } from '@reliefmesh/shared-types'

const props = defineProps<{ assignment: Assignment }>()
const emit = defineEmits<{ updated: [assignment: Assignment]; queued: [] }>()
const sync = useSyncStore()
const toasts = useToastStore()
const errors = useFormErrors()
const target = ref<AssignmentStatus | null>(null)
const reason = ref('')
const evidence = ref<EvidenceType>('volunteer_confirmation')
const evidenceRef = ref('')
const handover = ref<HandoverStatus>('handed_over')
const handoverNotes = ref('')
const busy = ref(false)

const needsReason = computed(() => target.value === 'declined' || target.value === 'unable_to_complete' || target.value === 'cancelled')
const isDelivery = computed(() => target.value === 'delivered' || target.value === 'partially_delivered')
const handoverOptions: HandoverStatus[] = ['handed_over', 'received', 'not_applicable', 'not_started']

function open(s: AssignmentStatus) {
  target.value = s
  reason.value = ''
  evidence.value = props.assignment.viewer_relation === 'coordinator' ? 'coordinator_confirmation' : 'volunteer_confirmation'
  evidenceRef.value = ''
  handover.value = 'handed_over'
  handoverNotes.value = props.assignment.handover_notes
  errors.clear()
}

async function confirm() {
  if (!target.value) return
  busy.value = true
  errors.clear()
  const payload: AssignmentStatusInput = { status: target.value, reason: reason.value.trim(), version: props.assignment.version }
  if (isDelivery.value) {
    payload.completion_evidence_type = evidence.value
    payload.completion_evidence_reference = evidenceRef.value.trim()
    payload.handover_status = handover.value
    payload.handover_notes = handoverNotes.value.trim()
  }
  try {
    const res = await sync.submit({
      type: 'assignment.status', entity_id: props.assignment.id, payload,
      summary: `${assignmentStatusAction[target.value] ?? target.value}: ${props.assignment.request?.reference ?? 'assignment'}`,
    }, { dropOnReject: true })
    if (res.status === 'applied') {
      toasts.success(`Assignment: ${assignmentStatusLabel[target.value]}.`)
      emit('updated', res.entity as Assignment)
      target.value = null
    } else if (res.status === 'queued') {
      toasts.info('Saved on this device. It will be sent when the connection returns.')
      emit('queued')
      target.value = null
    } else {
      errors.fromDetail(res.error)
      if (res.status === 'conflict') {
        await sync.discard(res.opId)
        if (res.entity) emit('updated', res.entity as Assignment)
      }
    }
  } catch (e) {
    errors.fromError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="assignment.permissions.allowed_statuses.length" class="flex flex-wrap gap-2">
    <button
      v-for="s in assignment.permissions.allowed_statuses"
      :key="s"
      type="button"
      :class="['declined', 'unable_to_complete', 'cancelled'].includes(s) ? 'btn-secondary' : 'btn-primary'"
      :data-testid="`assignment-${s}`"
      @click="open(s)"
    >
      {{ assignmentStatusAction[s] }}
    </button>
    <ConfirmDialog
      :open="target !== null"
      :title="`${assignmentStatusAction[target as AssignmentStatus] ?? ''}?`"
      :confirm-label="assignmentStatusAction[target as AssignmentStatus] ?? 'Confirm'"
      :danger="needsReason"
      :busy="busy"
      :confirm-disabled="needsReason && reason.trim().length < 3"
      @confirm="confirm"
      @cancel="target = null"
    >
      <AlertBox v-if="errors.message.value" tone="danger">{{ errors.message.value }}</AlertBox>
      <FormField v-if="needsReason" v-slot="f" label="Reason" required help="Visible to coordinators. No personal or health details." :error="errors.fields.value.reason">
        <textarea :id="f.id" v-model="reason" class="field-input" rows="2" maxlength="1000" data-testid="assignment-reason" />
      </FormField>
      <template v-if="isDelivery">
        <p class="text-sm">Delivery does not close the request. A coordinator confirms when the need is actually met.</p>
        <FormField v-slot="f" label="How was the delivery confirmed?" help="Photos or ID documents are never required.">
          <select :id="f.id" v-model="evidence" class="field-input" data-testid="evidence-type">
            <option v-for="e in EVIDENCE_TYPES" :key="e" :value="e">{{ evidenceLabel[e] }}</option>
          </select>
        </FormField>
        <FormField v-slot="f" label="Reference (optional)" help="E.g. an inventory sheet number.">
          <input :id="f.id" v-model="evidenceRef" class="field-input" maxlength="200">
        </FormField>
        <FormField v-slot="f" label="Handover">
          <select :id="f.id" v-model="handover" class="field-input"><option v-for="h in handoverOptions" :key="h" :value="h">{{ handoverLabel[h] }}</option></select>
        </FormField>
        <FormField v-slot="f" label="Handover notes (optional)" help="E.g. 'handed to shelter desk'. No personal details.">
          <textarea :id="f.id" v-model="handoverNotes" class="field-input" rows="2" maxlength="2000" />
        </FormField>
      </template>
    </ConfirmDialog>
  </div>
</template>
