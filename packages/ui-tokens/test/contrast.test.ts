import { describe, expect, it } from 'vitest'
import { colors, contrastRatio, tone, urgency } from '../src/index'

describe('design tokens', () => {
  it('urgency badges meet WCAG AA contrast', () => {
    for (const [name, c] of Object.entries(urgency)) {
      expect(contrastRatio(c.bg, c.fg), name).toBeGreaterThanOrEqual(4.5)
    }
  })
  it('status tones meet WCAG AA contrast', () => {
    for (const [name, c] of Object.entries(tone)) {
      expect(contrastRatio(c.bg, c.fg), name).toBeGreaterThanOrEqual(4.5)
    }
  })
  it('primary text and buttons meet WCAG AA contrast', () => {
    expect(contrastRatio(colors.ink.DEFAULT, colors.surface.DEFAULT)).toBeGreaterThanOrEqual(7)
    expect(contrastRatio(colors.ink.muted, colors.surface.DEFAULT)).toBeGreaterThanOrEqual(4.5)
    expect(contrastRatio(colors.brand.DEFAULT, colors.brand.ink)).toBeGreaterThanOrEqual(4.5)
    expect(contrastRatio(colors.danger.DEFAULT, colors.danger.ink)).toBeGreaterThanOrEqual(4.5)
    expect(contrastRatio(colors.exercise.DEFAULT, colors.exercise.ink)).toBeGreaterThanOrEqual(4.5)
    expect(contrastRatio(colors.ink.muted, colors.surface.sunken)).toBeGreaterThanOrEqual(4.5)
  })
})
