import { isApiError, isNetworkError } from '@reliefmesh/api-client'
import type { ApiErrorDetail } from '@reliefmesh/shared-types'

/** Collects field errors and a general message from API errors. */
export function useFormErrors() {
  const fields = ref<Record<string, string>>({})
  const message = ref('')

  function clear() {
    fields.value = {}
    message.value = ''
  }

  function fromDetail(d?: ApiErrorDetail) {
    fields.value = d?.fields ?? {}
    message.value = d?.message ?? 'The change was rejected.'
  }

  function fromError(e: unknown) {
    if (isApiError(e)) {
      fields.value = e.fields
      message.value = e.message
    } else if (isNetworkError(e)) {
      fields.value = {}
      message.value = 'The server cannot be reached. This action needs a connection.'
    } else {
      fields.value = {}
      message.value = 'Something went wrong.'
    }
  }

  return { fields, message, clear, fromDetail, fromError }
}
