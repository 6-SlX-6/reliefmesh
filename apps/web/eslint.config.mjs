import pluginVue from 'eslint-plugin-vue'
import tseslint from 'typescript-eslint'

export default [
  { ignores: ['.nuxt/**', '.output/**', 'node_modules/**', 'playwright-report/**', 'test-results/**', 'service-worker/**/*.js'] },
  ...tseslint.configs.recommended,
  ...pluginVue.configs['flat/recommended'],
  {
    files: ['**/*.vue'],
    languageOptions: { parserOptions: { parser: tseslint.parser, extraFileExtensions: ['.vue'], sourceType: 'module' } },
  },
  {
    rules: {
      // Nuxt pages/components follow Nuxt naming conventions.
      'vue/multi-word-component-names': 'off',
      // AppIcon renders a static, build-time SVG map (no user input).
      'vue/no-v-html': 'off',
      // Formatting is not enforced by the linter.
      'vue/max-attributes-per-line': 'off',
      'vue/singleline-html-element-content-newline': 'off',
      'vue/multiline-html-element-content-newline': 'off',
      'vue/html-self-closing': 'off',
      'vue/html-indent': 'off',
      'vue/html-closing-bracket-newline': 'off',
      'vue/first-attribute-linebreak': 'off',
      'vue/attributes-order': 'off',
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    },
  },
]
