import preset from '@reliefmesh/ui-tokens/tailwind'

export default {
  presets: [preset],
  content: [
    './components/**/*.{vue,ts}',
    './layouts/**/*.vue',
    './pages/**/*.vue',
    './composables/**/*.ts',
    './utils/**/*.ts',
    './app.vue',
    './error.vue',
  ],
}
