<script setup lang="ts">
useHead({ title: 'Change password' })
const auth = useAuthStore()
const toasts = useToastStore()
const errors = useFormErrors()
const current = ref('')
const next = ref('')
const repeat = ref('')
const busy = ref(false)

async function submit() {
  errors.clear()
  if (next.value !== repeat.value) {
    errors.fields.value = { repeat: 'The passwords do not match.' }
    return
  }
  busy.value = true
  try {
    const res = await useApi().auth.changePassword(current.value, next.value)
    await auth.setSession(res.user, res.csrf_token)
    toasts.success('Password changed. Other sessions were signed out.')
    current.value = next.value = repeat.value = ''
    await navigateTo(auth.homePath)
  } catch (e) {
    errors.fromError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-lg">
    <PageHeader title="Change password" />
    <AlertBox v-if="auth.user?.must_change_password" tone="warning" class="mb-4">You signed in with a temporary password. Choose your own password to continue.</AlertBox>
    <form class="card flex flex-col gap-4" @submit.prevent="submit">
      <AlertBox v-if="errors.message.value" tone="danger">{{ errors.message.value }}</AlertBox>
      <FormField v-slot="f" label="Current password" required :error="errors.fields.value.current_password">
        <input :id="f.id" v-model="current" class="field-input" type="password" autocomplete="current-password" data-testid="current-password">
      </FormField>
      <FormField v-slot="f" label="New password" required :error="errors.fields.value.new_password" help="At least 12 characters. A passphrase of several words is easy to remember and strong.">
        <input :id="f.id" v-model="next" class="field-input" type="password" autocomplete="new-password" minlength="12" data-testid="new-password">
      </FormField>
      <FormField v-slot="f" label="Repeat new password" required :error="errors.fields.value.repeat">
        <input :id="f.id" v-model="repeat" class="field-input" type="password" autocomplete="new-password" data-testid="repeat-password">
      </FormField>
      <button type="submit" class="btn-primary" :disabled="busy || !current || !next" data-testid="change-password-submit">Change password</button>
    </form>
  </div>
</template>
