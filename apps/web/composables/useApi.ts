import type { ApiClient } from '@reliefmesh/api-client'

export function useApi(): ApiClient {
  return useNuxtApp().$api as ApiClient
}
