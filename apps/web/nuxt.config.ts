// ReliefMesh web app: a client-rendered, offline-capable PWA.
//
// - SSR is disabled: the app is generated as static files and served by a
//   static file server; all data comes from the same-origin API.
// - No external requests: no CDNs, web fonts, analytics or telemetry.
// - The service worker precaches the app shell only. API responses are never
//   cached by the service worker; the app keeps a deliberate, privacy-aware
//   offline cache in IndexedDB instead (see utils/offline-db.ts).
// The service worker is built before `nuxt generate` writes the HTML shell,
// so the shell is added to the precache explicitly with a per-build revision.
const buildRevision = process.env.RELIEFMESH_BUILD_REVISION ?? Date.now().toString(36)

export default defineNuxtConfig({
  compatibilityDate: '2025-07-01',
  ssr: false,
  devtools: { enabled: false },
  telemetry: false,
  modules: ['@nuxtjs/tailwindcss', '@pinia/nuxt', '@vueuse/nuxt', '@vite-pwa/nuxt'],
  css: ['~/assets/css/main.css'],
  components: [{ path: '~/components', pathPrefix: false }],
  app: {
    head: {
      htmlAttrs: { lang: 'en' },
      title: 'ReliefMesh',
      meta: [
        { name: 'viewport', content: 'width=device-width, initial-scale=1, viewport-fit=cover' },
        { name: 'description', content: 'Offline-first coordination for local aid and preparedness exercises. Not an emergency service.' },
        { name: 'theme-color', content: '#0b4f8a' },
        { name: 'referrer', content: 'no-referrer' },
        { name: 'robots', content: 'noindex, nofollow' },
      ],
      link: [
        { rel: 'icon', href: '/favicon.svg', type: 'image/svg+xml' },
        { rel: 'apple-touch-icon', href: '/icons/icon-192.png' },
      ],
    },
  },
  tailwindcss: { cssPath: false, configPath: 'tailwind.config.ts', viewer: false },
  nitro: {
    // Only the app shell is generated; client-side routing handles the rest.
    prerender: { crawlLinks: false, routes: ['/', '/200.html', '/404.html'] },
    devProxy: {
      '/api': { target: process.env.RELIEFMESH_API_URL ?? 'http://localhost:8080/api', changeOrigin: false },
      '/healthz': { target: (process.env.RELIEFMESH_API_URL ?? 'http://localhost:8080/api').replace(/\/api$/, '') + '/healthz' },
    },
  },
  pwa: {
    strategies: 'injectManifest',
    srcDir: 'service-worker',
    filename: 'sw.ts',
    registerType: 'prompt',
    injectRegister: false,
    manifest: {
      name: 'ReliefMesh',
      short_name: 'ReliefMesh',
      description: 'Offline-first coordination for local aid and preparedness exercises. Not an emergency service.',
      lang: 'en',
      start_url: '/',
      scope: '/',
      display: 'standalone',
      background_color: '#ffffff',
      theme_color: '#0b4f8a',
      icons: [
        { src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png' },
        { src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png' },
        { src: '/icons/icon-512-maskable.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
      ],
    },
    injectManifest: {
      globPatterns: ['**/*.{js,css,html,svg,png,ico,webmanifest,json}'],
      globIgnores: ['**/_payload.json'],
      additionalManifestEntries: [{ url: 'index.html', revision: buildRevision }],
    },
    client: { installPrompt: false, periodicSyncForUpdates: 3600 },
    devOptions: { enabled: false, type: 'module' },
  },
  typescript: { strict: true, typeCheck: false },
  vite: {
    build: { sourcemap: false },
  },
})
