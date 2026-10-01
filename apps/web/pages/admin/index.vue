<script setup lang="ts">
import type { AuditVerifyResult, Dashboard } from '@reliefmesh/shared-types'

definePageMeta({ layout: 'admin', requires: 'user.manage' })
useHead({ title: 'Administration' })
const dash = ref<Dashboard | null>(null)
const verify = ref<AuditVerifyResult | null>(null)
onMounted(async () => {
  const api = useApi()
  try {
    ;[dash.value, verify.value] = await Promise.all([api.dashboard(), api.admin.verifyAudit()])
  } catch {
    /* shown as empty */
  }
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="Administration" />
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
      <StatCard label="Open requests (aggregate)" :value="dash?.requests.open ?? '-'" />
      <StatCard label="Active assignments" :value="dash?.assignments.active ?? '-'" />
      <StatCard label="Audit log integrity" :value="verify ? (verify.valid ? 'Intact' : 'BROKEN') : '-'" :tone="verify ? (verify.valid ? 'success' : 'danger') : 'neutral'" :help="verify ? `${verify.events_checked} events verified` : undefined" to="/admin/audit" />
    </div>
    <section class="card flex flex-col gap-2">
      <h2>Responsibilities</h2>
      <ul class="list-disc pl-6">
        <li><NuxtLink to="/admin/users">Users</NuxtLink>: create accounts, assign roles, reset passwords (temporary passwords are shown once).</li>
        <li><NuxtLink to="/admin/settings">Settings & notices</NuxtLink>: the emergency notice for your country, exercise mode, location precision, retention.</li>
        <li><NuxtLink to="/admin/categories">Categories</NuxtLink>: rename, reorder or disable request categories.</li>
        <li><NuxtLink to="/admin/audit">Audit log</NuxtLink>: system events and tamper-evidence check.</li>
        <li><NuxtLink to="/admin/data">Data & retention</NuxtLink>: apply retention, delete records on request, load exercise scenarios.</li>
      </ul>
      <p class="text-sm text-ink-muted">Administrators do not see request contents or personal data. To coordinate operations, use a separate account with the coordinator role or add that role deliberately.</p>
    </section>
  </div>
</template>
