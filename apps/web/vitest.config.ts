import { fileURLToPath } from 'node:url'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { defineConfig } from 'vitest/config'

// Unit tests run without the Nuxt runtime. Nuxt's auto-imports (Vue APIs,
// utils, stores, components) are reproduced with unplugin so components can
// be mounted in isolation; Nuxt-only helpers are stubbed in test/setup.ts.
export default defineConfig({
  plugins: [
    vue(),
    AutoImport({
      imports: ['vue', 'pinia'],
      dirs: ['utils', 'stores', 'composables'],
      vueTemplate: true,
      dts: false,
    }),
    Components({ dirs: ['components'], directoryAsNamespace: false, dts: false }),
  ],
  resolve: {
    alias: {
      '~': fileURLToPath(new URL('./', import.meta.url)),
    },
  },
  test: {
    environment: 'happy-dom',
    include: ['test/**/*.test.ts'],
    setupFiles: ['test/setup.ts'],
  },
})
