import { defineStore } from 'pinia'

export interface Toast {
  id: number
  kind: 'success' | 'info' | 'warning' | 'error'
  message: string
}

let next = 1

export const useToastStore = defineStore('toasts', {
  state: () => ({ items: [] as Toast[] }),
  actions: {
    push(kind: Toast['kind'], message: string, timeoutMs = 6000) {
      const id = next++
      this.items.push({ id, kind, message })
      if (timeoutMs > 0) setTimeout(() => this.dismiss(id), timeoutMs)
    },
    success(message: string) {
      this.push('success', message)
    },
    info(message: string) {
      this.push('info', message)
    },
    warning(message: string) {
      this.push('warning', message, 9000)
    },
    error(message: string) {
      this.push('error', message, 0)
    },
    dismiss(id: number) {
      this.items = this.items.filter((t) => t.id !== id)
    },
  },
})
