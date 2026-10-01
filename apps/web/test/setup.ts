import 'fake-indexeddb/auto'
import { config } from '@vue/test-utils'

// Nuxt components and helpers that are not available outside the Nuxt runtime.
config.global.stubs = {
  NuxtLink: { props: ['to'], template: '<a :href="String(to)"><slot /></a>' },
}

const g = globalThis as Record<string, unknown>
g.useApi = () => ({})
g.navigateTo = () => undefined
g.useRoute = () => ({ path: '/', fullPath: '/', query: {} })
g.useNuxtApp = () => ({ $pwa: undefined })

// happy-dom lacks showModal on <dialog>.
if (typeof HTMLDialogElement !== 'undefined' && !HTMLDialogElement.prototype.showModal) {
  HTMLDialogElement.prototype.showModal = function showModal(this: HTMLDialogElement) { this.setAttribute('open', '') }
  HTMLDialogElement.prototype.close = function close(this: HTMLDialogElement) { this.removeAttribute('open') }
}
