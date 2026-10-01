<script setup lang="ts">
import type { Note, NoteVisibility } from '@reliefmesh/shared-types'

const props = defineProps<{ kind: 'request' | 'offer'; id: string; clientId?: string; visibilities: NoteVisibility[]; sensitiveContext?: boolean }>()
const sync = useSyncStore()
const toasts = useToastStore()
const errors = useFormErrors()
const notes = ref<Note[]>([])
const loadError = ref('')
const body = ref('')
const visibility = ref<NoteVisibility>(props.visibilities.includes('shared') && props.visibilities.length === 1 ? 'shared' : (props.visibilities[0] ?? 'shared'))
const sensitive = ref(false)
const busy = ref(false)
const isLocal = computed(() => props.id.startsWith('local:'))
const medicalHint = computed(() => looksMedical(body.value))

async function load() {
  loadError.value = ''
  if (isLocal.value) return
  if (!sync.online) {
    loadError.value = 'Notes are only available while connected.'
    return
  }
  try {
    const api = useApi()
    notes.value = (props.kind === 'request' ? await api.requests.notes(props.id) : await api.offers.notes(props.id)).items
  } catch (e) {
    errors.fromError(e)
    loadError.value = errors.message.value
  }
}

async function add() {
  if (!body.value.trim()) return
  busy.value = true
  errors.clear()
  const payload = { client_id: uuid(), body: body.value.trim(), visibility: visibility.value, is_sensitive: sensitive.value }
  try {
    const res = await sync.submit({
      type: props.kind === 'request' ? 'request.note' : 'offer.note',
      entity_id: isLocal.value ? undefined : props.id,
      entity_client_id: isLocal.value ? props.clientId : undefined,
      payload,
      summary: `Note: ${payload.body.slice(0, 40)}`,
    }, { dropOnReject: true })
    if (res.status === 'applied') {
      body.value = ''
      sensitive.value = false
      await load()
      toasts.success('Note added.')
    } else if (res.status === 'queued') {
      body.value = ''
      toasts.info('Note saved on this device. It will be sent when the connection returns.')
    } else {
      errors.fromDetail(res.error)
    }
  } catch (e) {
    errors.fromError(e)
  } finally {
    busy.value = false
  }
}

onMounted(load)
watch(() => sync.online, (o) => { if (o) void load() })
</script>

<template>
  <section class="card" aria-labelledby="notes-h">
    <h2 id="notes-h" class="mb-3 flex items-center gap-2"><AppIcon name="message" /> Notes and updates</h2>
    <p v-if="loadError" class="mb-3 text-ink-muted">{{ loadError }}</p>
    <ol v-if="notes.length" class="mb-4 flex flex-col gap-3">
      <li v-for="n in notes" :key="n.id" class="rounded-lg border border-surface-border bg-surface-sunken p-3">
        <p class="mb-1 flex flex-wrap items-center gap-2 text-sm text-ink-muted">
          <strong class="text-ink">{{ n.author.display_name || n.author.label }}</strong>
          <span>{{ formatDateTime(n.created_at) }}</span>
          <span class="rounded border border-surface-border px-1.5 text-xs font-semibold">{{ noteVisibilityLabel[n.visibility].split(' (')[0] }}</span>
          <span v-if="n.is_sensitive" class="inline-flex items-center gap-1 text-xs font-semibold"><AppIcon name="lock" :size="12" />Sensitive</span>
        </p>
        <p v-if="n.redacted" class="italic text-ink-muted">This note was redacted.</p>
        <p v-else class="whitespace-pre-line">{{ n.body }}</p>
      </li>
    </ol>
    <p v-else-if="!loadError && !isLocal" class="mb-4 text-ink-muted">No notes yet.</p>

    <form v-if="visibilities.length" class="flex flex-col gap-3 border-t border-surface-border pt-4" @submit.prevent="add">
      <AlertBox v-if="errors.message.value && !loadError" tone="danger">{{ errors.message.value }}</AlertBox>
      <FormField v-slot="f" label="Add a note" :error="errors.fields.value.body" help="Notes cannot be edited later. Do not write health details or contact data here.">
        <textarea :id="f.id" v-model="body" class="field-input" rows="3" maxlength="4000" data-testid="note-body" />
      </FormField>
      <AlertBox v-if="medicalHint || sensitiveContext" tone="warning">Avoid medical details. ReliefMesh handles logistics only.</AlertBox>
      <FormField v-if="visibilities.length > 1" v-slot="f" label="Who can read this note?">
        <select :id="f.id" v-model="visibility" class="field-input">
          <option v-for="v in visibilities" :key="v" :value="v">{{ noteVisibilityLabel[v] }}</option>
        </select>
      </FormField>
      <label class="flex items-center gap-2 font-normal"><input v-model="sensitive" type="checkbox" class="h-5 w-5"> Mark as sensitive</label>
      <button type="submit" class="btn-primary w-fit" :disabled="busy || !body.trim()" data-testid="add-note">Add note</button>
    </form>
  </section>
</template>
