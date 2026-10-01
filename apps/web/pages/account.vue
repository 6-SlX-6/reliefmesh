<script setup lang="ts">
import type { Availability } from '@reliefmesh/shared-types'

useHead({ title: 'My account' })
const auth = useAuthStore()
const sync = useSyncStore()
const toasts = useToastStore()
const availability = ref<Availability>(auth.user?.availability ?? 'unavailable')
const note = ref(auth.user?.availability_note ?? '')
const logoutOpen = ref(false)

async function saveAvailability() {
  try {
    await useApi().me.setAvailability(availability.value, note.value)
    if (auth.user) Object.assign(auth.user, { availability: availability.value, availability_note: note.value })
    toasts.success('Availability updated.')
  } catch {
    toasts.error('Could not update availability. A connection is required.')
  }
}

function requestLogout() {
  if (sync.pending || sync.problems) logoutOpen.value = true
  else void doLogout()
}
async function doLogout() {
  logoutOpen.value = false
  await auth.logout()
  await navigateTo('/login')
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="My account" />
    <section class="card">
      <dl class="kv">
        <div><dt>Name</dt><dd>{{ auth.user?.display_name }}</dd></div>
        <div><dt>Username</dt><dd class="font-mono">{{ auth.user?.username }}</dd></div>
        <div><dt>Organization</dt><dd>{{ auth.user?.organization.name }}</dd></div>
        <div><dt>Roles</dt><dd>{{ auth.user?.roles.map((r) => roleLabel[r]).join(', ') }}</dd></div>
      </dl>
    </section>
    <section v-if="auth.isVolunteer" class="card flex flex-col gap-3">
      <h2>My availability</h2>
      <p class="text-ink-muted">Coordinators see this when they plan assignments.</p>
      <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
        <label v-for="(label, key) in availabilityLabel" :key="key" class="choice"><input v-model="availability" type="radio" :value="key"><span class="font-semibold">{{ label }}</span></label>
      </div>
      <FormField v-slot="f" label="Note (optional)" help="E.g. 'until 18:00', 'only with car'."><input :id="f.id" v-model="note" class="field-input" maxlength="200"></FormField>
      <button type="button" class="btn-primary w-fit" :disabled="!sync.online" @click="saveAvailability">Save availability</button>
    </section>
    <section class="card flex flex-col gap-3">
      <h2>Security</h2>
      <NuxtLink to="/change-password" class="btn-secondary w-fit">Change password</NuxtLink>
      <button type="button" class="btn-danger w-fit" data-testid="logout" @click="requestLogout"><AppIcon name="logout" :size="18" /> Sign out</button>
      <p class="text-sm text-ink-muted">Signing out removes the data saved on this device for offline use.</p>
    </section>
    <ConfirmDialog :open="logoutOpen" title="Unsynchronized changes" confirm-label="Sign out anyway" danger @confirm="doLogout" @cancel="logoutOpen = false">
      <p>{{ sync.pending + sync.problems }} change(s) have not reached the server yet. They stay on this device and are sent the next time you sign in here - but nobody else can see them until then.</p>
      <NuxtLink to="/sync" @click="logoutOpen = false">Review pending changes</NuxtLink>
    </ConfirmDialog>
  </div>
</template>
