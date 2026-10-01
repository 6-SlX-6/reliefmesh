<script setup lang="ts">
import { isApiError, isNetworkError } from '@reliefmesh/api-client'

definePageMeta({ layout: 'auth' })
useHead({ title: 'Sign in' })

const auth = useAuthStore()
const settings = useSettingsStore()
const route = useRoute()
const username = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await auth.login(username.value.trim(), password.value)
    password.value = ''
    const redirect = typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/') && !route.query.redirect.startsWith('//')
      ? route.query.redirect : auth.homePath
    await navigateTo(auth.user?.must_change_password ? '/change-password' : redirect)
  } catch (e) {
    if (isNetworkError(e)) error.value = 'The ReliefMesh server cannot be reached. Signing in needs a connection to the server.'
    else if (isApiError(e)) error.value = e.message
    else error.value = 'Sign-in failed.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="card flex flex-col gap-5">
    <div>
      <h1>Sign in to {{ settings.instanceName }}</h1>
      <p class="mt-1 text-ink-muted">Accounts are created by your organization's administrator.</p>
    </div>
    <AlertBox v-if="auth.sessionExpired || route.query.expired" tone="warning">
      Your session has ended. Please sign in again. Changes saved on this device are kept and will be sent after you sign in.
    </AlertBox>
    <AlertBox v-if="error" tone="danger" data-testid="login-error">{{ error }}</AlertBox>
    <form class="flex flex-col gap-4" @submit.prevent="submit">
      <FormField v-slot="f" label="Username" required>
        <input :id="f.id" v-model="username" class="field-input" autocomplete="username" autocapitalize="none" spellcheck="false" required data-testid="login-username">
      </FormField>
      <FormField v-slot="f" label="Password" required>
        <input :id="f.id" v-model="password" class="field-input" type="password" autocomplete="current-password" required data-testid="login-password">
      </FormField>
      <button type="submit" class="btn-primary text-lg" :disabled="busy || !username || !password" data-testid="login-submit">
        {{ busy ? 'Signing in...' : 'Sign in' }}
      </button>
    </form>
    <p class="text-sm text-ink-muted">Forgot your password? Ask an administrator of your organization to reset it. ReliefMesh does not send e-mails.</p>
  </div>
</template>
