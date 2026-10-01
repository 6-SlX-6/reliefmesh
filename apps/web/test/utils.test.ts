import { describe, expect, it } from 'vitest'
import { precisionKm, roundCoord, validCoords } from '../utils/location'
import { looksLikeContact, looksMedical } from '../utils/privacy'
import { formatQuantity, fromLocalInput, toLocalInput } from '../utils/format'
import { uuid } from '../utils/uuid'

describe('location utils', () => {
  it('rounds like the server and caps precision', () => {
    expect(roundCoord(52.520008, 2)).toBe(52.52)
    expect(roundCoord(13.405954, 2)).toBe(13.41)
    expect(roundCoord(52.5200081234, 9)).toBe(52.52)
    expect(precisionKm(2)).toBeCloseTo(1.11, 2)
  })
  it('validates coordinates', () => {
    expect(validCoords(52, 13)).toBe(true)
    expect(validCoords(91, 0)).toBe(false)
    expect(validCoords(Number.NaN, 0)).toBe(false)
  })
})

describe('privacy hints', () => {
  it('detects medical details', () => {
    expect(looksMedical('Insulin 100 IU/ml')).toBe(true)
    expect(looksMedical('Please bring 2 x 500mg tablets')).toBe(true)
    expect(looksMedical('Pick up a parcel at the pharmacy')).toBe(false)
  })
  it('detects contact details', () => {
    expect(looksLikeContact('call me on +49 170 1234567')).toBe(true)
    expect(looksLikeContact('mail a.b@example.org')).toBe(true)
    expect(looksLikeContact('North district shelter')).toBe(false)
  })
})

describe('format utils', () => {
  it('formats quantities', () => {
    expect(formatQuantity(5, 'litres')).toBe('5 litres')
    expect(formatQuantity(null, 'x')).toBe('')
  })
  it('round-trips datetime-local values', () => {
    const iso = fromLocalInput('2026-10-01T14:30')!
    expect(toLocalInput(iso)).toBe('2026-10-01T14:30')
    expect(fromLocalInput('')).toBeNull()
  })
  it('creates v4 uuids', () => {
    expect(uuid()).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
  })
})
