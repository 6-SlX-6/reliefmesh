<script setup lang="ts">
definePageMeta({ requires: 'export.reports' })
useHead({ title: 'Reports' })
const toasts = useToastStore()
const today = new Date()
const from = ref(new Date(today.getTime() - 30 * 86400000).toISOString().slice(0, 10))
const to = ref(new Date(today.getTime() + 86400000).toISOString().slice(0, 10))
const summary = ref<Record<string, unknown> | null>(null)
const busy = ref(false)

function download(name: string, content: string, type: string) {
  const blob = new Blob([content], { type })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}

async function csv() {
  busy.value = true
  try {
    download(`reliefmesh-requests-${from.value}-${to.value}.csv`, await useApi().exports.requestsCsv(from.value, to.value), 'text/csv')
    toasts.success('Report downloaded.')
  } catch {
    toasts.error('The report could not be generated. A connection is required.')
  } finally {
    busy.value = false
  }
}
async function loadSummary() {
  busy.value = true
  try {
    summary.value = await useApi().exports.summary(from.value, to.value)
  } catch {
    toasts.error('The summary could not be generated.')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="Reports" />
    <AlertBox tone="info" title="Non-personal reports">
      Reports contain no names, contact details, coordinates, descriptions or notes. Areas of sensitive requests are omitted. Every export is logged.
    </AlertBox>
    <form class="card grid grid-cols-1 gap-3 sm:grid-cols-4" @submit.prevent="loadSummary">
      <FormField v-slot="f" label="From"><input :id="f.id" v-model="from" class="field-input" type="date"></FormField>
      <FormField v-slot="f" label="To"><input :id="f.id" v-model="to" class="field-input" type="date"></FormField>
      <div class="flex items-end"><button type="submit" class="btn-primary w-full" :disabled="busy">Show summary</button></div>
      <div class="flex items-end"><button type="button" class="btn-secondary w-full" :disabled="busy" @click="csv"><AppIcon name="download" :size="18" /> Request list (CSV)</button></div>
    </form>
    <section v-if="summary" class="card flex flex-col gap-3">
      <h2>Summary</h2>
      <pre class="overflow-x-auto rounded-lg bg-surface-sunken p-3 text-sm">{{ JSON.stringify(summary, null, 2) }}</pre>
      <button type="button" class="btn-secondary w-fit" @click="download(`reliefmesh-summary-${from}-${to}.json`, JSON.stringify(summary, null, 2), 'application/json')">
        <AppIcon name="download" :size="18" /> Download JSON
      </button>
    </section>
  </div>
</template>
