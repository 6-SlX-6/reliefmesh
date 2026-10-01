<script setup lang="ts">
import { ROLES, type AdminUser, type Role } from '@reliefmesh/shared-types'

definePageMeta({ layout: 'admin', requires: 'user.manage' })
useHead({ title: 'Users' })
const auth = useAuthStore()
const toasts = useToastStore()
const errors = useFormErrors()
const users = ref<AdminUser[]>([])
const createOpen = ref(false)
const form = reactive({ username: '', display_name: '', roles: ['volunteer'] as Role[] })
const tempPassword = ref<{ user: string; password: string } | null>(null)
const editing = ref<AdminUser | null>(null)
const editRoles = ref<Role[]>([])
const resetTarget = ref<AdminUser | null>(null)
const busy = ref(false)

async function load() {
  users.value = (await useApi().admin.users()).items
}
onMounted(load)

async function create() {
  busy.value = true
  errors.clear()
  try {
    const res = await useApi().admin.createUser({ ...form, username: form.username.trim(), display_name: form.display_name.trim() })
    tempPassword.value = { user: res.user.username, password: res.temporary_password }
    createOpen.value = false
    Object.assign(form, { username: '', display_name: '', roles: ['volunteer'] })
    await load()
  } catch (e) {
    errors.fromError(e)
  } finally {
    busy.value = false
  }
}

async function update(u: AdminUser, input: { roles?: Role[]; is_active?: boolean; unlock?: boolean }) {
  try {
    await useApi().admin.updateUser(u.id, input)
    toasts.success(`${u.display_name} updated.`)
    editing.value = null
    await load()
  } catch (e) {
    errors.fromError(e)
    toasts.error(errors.message.value)
  }
}

async function reset() {
  if (!resetTarget.value) return
  busy.value = true
  try {
    const r = await useApi().admin.resetPassword(resetTarget.value.id)
    tempPassword.value = { user: resetTarget.value.username, password: r.temporary_password }
    resetTarget.value = null
    await load()
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="Users">
      <template #actions><button type="button" class="btn-primary" data-testid="create-user" @click="createOpen = true; errors.clear()"><AppIcon name="plus" :size="18" /> New user</button></template>
    </PageHeader>
    <AlertBox v-if="tempPassword" tone="warning" title="Temporary password - shown only once" data-testid="temp-password">
      <p>Give this password to <strong>{{ tempPassword.user }}</strong> in person or through a trusted channel. They must change it at first sign-in.</p>
      <p class="mt-2 select-all rounded bg-surface px-3 py-2 font-mono text-xl" data-testid="temp-password-value">{{ tempPassword.password }}</p>
      <button type="button" class="btn-secondary btn-sm mt-2" @click="tempPassword = null">I have passed it on</button>
    </AlertBox>
    <div class="card overflow-x-auto">
      <table class="w-full min-w-[40rem] text-left">
        <thead><tr class="border-b-2 border-surface-strong"><th class="py-2 pr-3">Name</th><th class="py-2 pr-3">Username</th><th class="py-2 pr-3">Roles</th><th class="py-2 pr-3">Status</th><th class="py-2">Actions</th></tr></thead>
        <tbody>
          <tr v-for="u in users" :key="u.id" class="border-b border-surface-border align-top" data-testid="user-row">
            <td class="py-2 pr-3 font-semibold">{{ u.display_name }}</td>
            <td class="py-2 pr-3 font-mono text-sm">{{ u.username }}</td>
            <td class="py-2 pr-3 text-sm">{{ u.roles.map((r) => roleLabel[r]).join(', ') }}</td>
            <td class="py-2 pr-3 text-sm">
              <span v-if="!u.is_active" class="font-semibold text-danger">Deactivated</span>
              <span v-else-if="u.locked" class="font-semibold text-warning">Locked</span>
              <span v-else>Active</span>
              <span v-if="u.must_change_password" class="block text-ink-muted">Must change password</span>
              <span class="block text-ink-muted">Last sign-in: {{ u.last_login_at ? formatRelative(u.last_login_at) : 'never' }}</span>
            </td>
            <td class="py-2">
              <div class="flex flex-wrap gap-1">
                <button type="button" class="btn-secondary btn-sm" @click="editing = u; editRoles = [...u.roles]">Roles</button>
                <button type="button" class="btn-secondary btn-sm" @click="resetTarget = u">Reset password</button>
                <button v-if="u.locked" type="button" class="btn-secondary btn-sm" @click="update(u, { unlock: true })">Unlock</button>
                <button v-if="u.id !== auth.user?.id" type="button" class="btn-secondary btn-sm" @click="update(u, { is_active: !u.is_active })">{{ u.is_active ? 'Deactivate' : 'Activate' }}</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <ConfirmDialog :open="createOpen" title="Create user" confirm-label="Create user" :busy="busy" :confirm-disabled="!form.username || !form.display_name || !form.roles.length" @confirm="create" @cancel="createOpen = false">
      <AlertBox v-if="errors.message.value" tone="danger">{{ errors.message.value }}</AlertBox>
      <FormField v-slot="f" label="Username" required :error="errors.fields.value.username" help="Lowercase letters, digits, '.', '_', '-' or '@'. No e-mail is sent.">
        <input :id="f.id" v-model="form.username" class="field-input" autocapitalize="none" data-testid="new-username">
      </FormField>
      <FormField v-slot="f" label="Display name" required :error="errors.fields.value.display_name" help="Shown to coordinators. A first name or nickname is enough.">
        <input :id="f.id" v-model="form.display_name" class="field-input" data-testid="new-display-name">
      </FormField>
      <fieldset>
        <legend class="mb-2 font-semibold">Roles</legend>
        <label v-for="r in ROLES" :key="r" class="choice mb-2"><input v-model="form.roles" type="checkbox" :value="r" :data-testid="`role-${r}`"><span><strong>{{ roleLabel[r] }}</strong><span class="block text-sm font-normal">{{ roleHelp[r] }}</span></span></label>
      </fieldset>
    </ConfirmDialog>

    <ConfirmDialog :open="editing !== null" :title="`Roles of ${editing?.display_name}`" confirm-label="Save roles" :confirm-disabled="!editRoles.length" @confirm="editing && update(editing, { roles: editRoles })" @cancel="editing = null">
      <p class="text-sm">Changing roles signs the user out on all devices.</p>
      <label v-for="r in ROLES" :key="r" class="choice"><input v-model="editRoles" type="checkbox" :value="r"><span><strong>{{ roleLabel[r] }}</strong><span class="block text-sm font-normal">{{ roleHelp[r] }}</span></span></label>
    </ConfirmDialog>

    <ConfirmDialog :open="resetTarget !== null" :title="`Reset password of ${resetTarget?.display_name}?`" confirm-label="Reset password" danger :busy="busy" @confirm="reset" @cancel="resetTarget = null">
      <p>A new temporary password is created and shown once. The user is signed out everywhere and must choose a new password.</p>
    </ConfirmDialog>
  </div>
</template>
