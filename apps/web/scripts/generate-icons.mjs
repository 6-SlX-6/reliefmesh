// Generates the PWA icons (PNG) without external dependencies.
// The emblem is an abstract "mesh" of connected nodes. It deliberately avoids
// crosses, crescents or crystals, which are protected emblems under
// international humanitarian law.
import { writeFileSync } from 'node:fs'
import { deflateSync } from 'node:zlib'

const BG = [11, 79, 138]
const FG = [255, 255, 255]

// Emblem geometry in a 0..1 coordinate space.
const nodes = [
  [0.5, 0.26], [0.28, 0.66], [0.72, 0.66], [0.5, 0.52],
]
const edges = [[0, 1], [1, 2], [2, 0], [0, 3], [1, 3], [2, 3]]

function distToSegment(px, py, ax, ay, bx, by) {
  const dx = bx - ax, dy = by - ay
  const t = Math.max(0, Math.min(1, ((px - ax) * dx + (py - ay) * dy) / (dx * dx + dy * dy)))
  const cx = ax + t * dx, cy = ay + t * dy
  return Math.hypot(px - cx, py - cy)
}

function coverage(x, y, scale, offset) {
  // Map pixel to emblem space (with padding for maskable icons).
  const u = (x - offset) / scale, v = (y - offset) / scale
  for (const [nx, ny] of nodes) if (Math.hypot(u - nx, v - ny) < 0.075) return 1
  for (const [a, b] of edges) {
    const [ax, ay] = nodes[a], [bx, by] = nodes[b]
    if (distToSegment(u, v, ax, ay, bx, by) < 0.028) return 1
  }
  return 0
}

function insideRoundedRect(x, y, size, radius) {
  const r = radius
  const cx = Math.min(Math.max(x, r), size - r)
  const cy = Math.min(Math.max(y, r), size - r)
  return Math.hypot(x - cx, y - cy) <= r
}

function render(size, { maskable }) {
  const ss = 4 // supersampling
  const scale = maskable ? size * 0.7 : size
  const offset = maskable ? size * 0.15 : 0
  const radius = maskable ? 0 : size * 0.18
  const rows = []
  for (let y = 0; y < size; y++) {
    const row = Buffer.alloc(1 + size * 4)
    row[0] = 0
    for (let x = 0; x < size; x++) {
      let fg = 0, inside = 0
      for (let sy = 0; sy < ss; sy++) for (let sx = 0; sx < ss; sx++) {
        const px = x + (sx + 0.5) / ss, py = y + (sy + 0.5) / ss
        if (maskable || insideRoundedRect(px, py, size, radius)) {
          inside++
          fg += coverage(px, py, scale, offset)
        }
      }
      const n = ss * ss
      const a = inside / n, f = inside ? fg / inside : 0
      const i = 1 + x * 4
      for (let c = 0; c < 3; c++) row[i + c] = Math.round(BG[c] * (1 - f) + FG[c] * f)
      row[i + 3] = Math.round(a * 255)
    }
    rows.push(row)
  }
  return png(size, size, Buffer.concat(rows))
}

function crc32(buf) {
  let c, crc = 0xffffffff
  for (let n = 0; n < buf.length; n++) {
    c = (crc ^ buf[n]) & 0xff
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
    crc = (crc >>> 8) ^ c
  }
  return (crc ^ 0xffffffff) >>> 0
}

function chunk(type, data) {
  const len = Buffer.alloc(4)
  len.writeUInt32BE(data.length)
  const td = Buffer.concat([Buffer.from(type), data])
  const crc = Buffer.alloc(4)
  crc.writeUInt32BE(crc32(td))
  return Buffer.concat([len, td, crc])
}

function png(w, h, raw) {
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(w, 0)
  ihdr.writeUInt32BE(h, 4)
  ihdr[8] = 8; ihdr[9] = 6; ihdr[10] = 0; ihdr[11] = 0; ihdr[12] = 0
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr), chunk('IDAT', deflateSync(raw, { level: 9 })), chunk('IEND', Buffer.alloc(0)),
  ])
}

const out = new URL('../public/icons/', import.meta.url)
writeFileSync(new URL('icon-192.png', out), render(192, { maskable: false }))
writeFileSync(new URL('icon-512.png', out), render(512, { maskable: false }))
writeFileSync(new URL('icon-512-maskable.png', out), render(512, { maskable: true }))
console.log('icons written')
