/**
 * ReliefMesh design tokens.
 *
 * Designed for use under stress: high contrast (all text/background pairs below
 * meet WCAG 2.2 AA, most meet AAA), status is never conveyed by color alone
 * (components always pair color with text and an icon), large touch targets and
 * system fonts only (no external font requests).
 */

export const colors = {
  ink: { DEFAULT: '#0b1220', muted: '#374151', subtle: '#4b5563', inverse: '#ffffff' },
  surface: { DEFAULT: '#ffffff', sunken: '#f3f4f6', raised: '#ffffff', border: '#9ca3af', strong: '#1f2937' },
  brand: { DEFAULT: '#0b4f8a', hover: '#093f6e', soft: '#e0eefa', ink: '#ffffff' },
  focus: '#ffbf47',
  danger: { DEFAULT: '#b3261e', soft: '#fde8e7', ink: '#ffffff', strong: '#8c1d18' },
  warning: { DEFAULT: '#8a4b00', soft: '#fff1d6', ink: '#ffffff' },
  success: { DEFAULT: '#1b6b3a', soft: '#e3f4e8', ink: '#ffffff' },
  info: { DEFAULT: '#0b4f8a', soft: '#e0eefa', ink: '#ffffff' },
  exercise: { DEFAULT: '#5b21b6', soft: '#ede9fe', ink: '#ffffff' },
} as const

/** Urgency palette: background + foreground pairs with >= 4.5:1 contrast. */
export const urgency = {
  critical: { bg: '#b3261e', fg: '#ffffff', border: '#8c1d18' },
  high: { bg: '#a04100', fg: '#ffffff', border: '#7a3200' },
  normal: { bg: '#e0eefa', fg: '#0b3a66', border: '#0b4f8a' },
  low: { bg: '#f3f4f6', fg: '#1f2937', border: '#6b7280' },
} as const

/** Status tones used by badges. */
export const tone = {
  neutral: { bg: '#f3f4f6', fg: '#1f2937', border: '#6b7280' },
  info: { bg: '#e0eefa', fg: '#0b3a66', border: '#0b4f8a' },
  progress: { bg: '#ede9fe', fg: '#3b1a80', border: '#5b21b6' },
  success: { bg: '#e3f4e8', fg: '#14532d', border: '#1b6b3a' },
  warning: { bg: '#fff1d6', fg: '#5c3200', border: '#8a4b00' },
  danger: { bg: '#fde8e7', fg: '#7f1d1d', border: '#b3261e' },
} as const

export type Tone = keyof typeof tone

export const requestStatusTone: Record<string, Tone> = {
  draft: 'neutral',
  submitted: 'info',
  under_review: 'info',
  verified: 'info',
  assigned: 'progress',
  in_progress: 'progress',
  partially_resolved: 'warning',
  resolved: 'success',
  cancelled: 'neutral',
  expired: 'neutral',
  duplicate: 'neutral',
}

export const offerStatusTone: Record<string, Tone> = {
  draft: 'neutral',
  available: 'success',
  partially_allocated: 'info',
  fully_allocated: 'progress',
  paused: 'warning',
  expired: 'neutral',
  cancelled: 'neutral',
}

export const assignmentStatusTone: Record<string, Tone> = {
  proposed: 'info',
  accepted: 'progress',
  declined: 'neutral',
  in_progress: 'progress',
  delivered: 'success',
  partially_delivered: 'warning',
  unable_to_complete: 'danger',
  cancelled: 'neutral',
}

export const fontFamily = {
  sans: ['system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'Ubuntu', 'Cantarell', 'Noto Sans', 'Helvetica Neue', 'Arial', 'sans-serif'],
  mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'Liberation Mono', 'monospace'],
}

/** Minimum touch target in CSS pixels (WCAG 2.2 target size, enhanced). */
export const minTarget = 48

/** WCAG relative luminance contrast ratio of two hex colors. */
export function contrastRatio(a: string, b: string): number {
  const lum = (hex: string) => {
    const h = hex.replace('#', '')
    const rgb = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16) / 255)
    const [r, g, bl] = rgb.map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)) as [number, number, number]
    return 0.2126 * r + 0.7152 * g + 0.0722 * bl
  }
  const [l1, l2] = [lum(a), lum(b)].sort((x, y) => y - x) as [number, number]
  return (l1 + 0.05) / (l2 + 0.05)
}
