/// <reference lib="webworker" />
// ReliefMesh service worker.
//
// It precaches the application shell so the app starts without a network
// connection. It deliberately does NOT cache API responses: offline data is
// managed by the app in IndexedDB with explicit privacy rules (see
// utils/offline-db.ts), and protected details are never cached at all.
import { cleanupOutdatedCaches, createHandlerBoundToURL, precacheAndRoute } from 'workbox-precaching'
import { NavigationRoute, registerRoute } from 'workbox-routing'

declare let self: ServiceWorkerGlobalScope

precacheAndRoute(self.__WB_MANIFEST)
cleanupOutdatedCaches()

// Client-side routes are served from the precached app shell.
registerRoute(
  new NavigationRoute(createHandlerBoundToURL('/index.html'), {
    denylist: [/^\/api\//, /^\/healthz/, /^\/readyz/],
  }),
)

self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') void self.skipWaiting()
})
