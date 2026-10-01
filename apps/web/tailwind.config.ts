import type { Config } from 'tailwindcss'
import preset from '@reliefmesh/ui-tokens/tailwind'

export default {
  presets: [preset as Partial<Config>],
  content: [
    './components/**/*.{vue,ts}',
    './layouts/**/*.vue',
    './pages/**/*.vue',
    './composables/**/*.ts',
    './utils/**/*.ts',
    './app.vue',
    './error.vue',
  ],
} satisfies Config
