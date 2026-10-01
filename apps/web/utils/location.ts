import { MAX_APPROX_DECIMALS } from '@reliefmesh/shared-types'

/** Rounds a coordinate exactly like the server does. */
export function roundCoord(v: number, decimals: number): number {
  const d = Math.min(Math.max(decimals, 0), MAX_APPROX_DECIMALS)
  const p = 10 ** d
  return Math.round(v * p) / p
}

/** Approximate size of a rounding cell in kilometres (latitude direction). */
export function precisionKm(decimals: number): number {
  return 111 / 10 ** Math.min(Math.max(decimals, 0), MAX_APPROX_DECIMALS)
}

export function validCoords(lat: unknown, lon: unknown): boolean {
  return typeof lat === 'number' && typeof lon === 'number' && Number.isFinite(lat) && Number.isFinite(lon) &&
    lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

/**
 * Reads the device position and immediately rounds it, so the precise
 * position never leaves this function (it is not stored or sent).
 */
export function currentApproximatePosition(decimals: number): Promise<{ lat: number; lon: number }> {
  return new Promise((resolve, reject) => {
    if (typeof navigator === 'undefined' || !navigator.geolocation) {
      reject(new Error('Location is not available on this device.'))
      return
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => resolve({ lat: roundCoord(pos.coords.latitude, decimals), lon: roundCoord(pos.coords.longitude, decimals) }),
      () => reject(new Error('Location permission was denied or the position is unavailable.')),
      { enableHighAccuracy: false, maximumAge: 300_000, timeout: 15_000 },
    )
  })
}
