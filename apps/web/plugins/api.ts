import { createApiClient, isNetworkError } from '@reliefmesh/api-client'

/**
 * Provides the typed API client as `$api`. The client reads the CSRF token
 * from the auth store and reports network failures to the sync store so the
 * connection indicator reacts immediately.
 */
export default defineNuxtPlugin((nuxtApp) => {
  const api = createApiClient({
    baseUrl: '',
    getCsrfToken: () => useAuthStore().csrf,
    onUnauthorized: () => useAuthStore().handleUnauthorized(),
    fetch: async (input, init) => {
      try {
        const res = await fetch(input, init)
        if (res.status < 500) useSyncStore().markReachable(true)
        return res
      } catch (e) {
        useSyncStore().markReachable(false)
        throw e
      }
    },
  })
  nuxtApp.provide('api', api)
  return { provide: { isNetworkError } }
})
