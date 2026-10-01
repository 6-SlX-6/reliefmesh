import { colors, fontFamily, minTarget } from './index'

/** Tailwind preset consumed by apps/web/tailwind.config.ts. */
export default {
  theme: {
    extend: {
      colors: {
        ink: colors.ink,
        surface: colors.surface,
        brand: colors.brand,
        focus: colors.focus,
        danger: colors.danger,
        warning: colors.warning,
        success: colors.success,
        info: colors.info,
        exercise: colors.exercise,
      },
      fontFamily,
      minHeight: { target: `${minTarget}px` },
      minWidth: { target: `${minTarget}px` },
      fontSize: { base: ['1.0625rem', { lineHeight: '1.6' }] },
    },
  },
}
