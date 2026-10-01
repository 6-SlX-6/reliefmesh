// Post-build step for `nuxt generate`:
//  * computes SHA-256 hashes of inline <script> blocks in the generated HTML
//    and writes the Content-Security-Policy used by the web container
//    (.output/nginx-csp.conf) so no 'unsafe-inline' scripts are allowed;
//  * verifies that the service worker and app shell were generated.
import { createHash } from 'node:crypto'
import { existsSync, readFileSync, writeFileSync } from 'node:fs'

const root = new URL('../.output/', import.meta.url)
const pub = new URL('public/', root)

function inlineHashes(html) {
  const out = []
  const re = /<script(\s[^>]*)?>([\s\S]*?)<\/script>/gi
  let m
  while ((m = re.exec(html))) {
    const attrs = m[1] ?? ''
    const body = m[2] ?? ''
    if (/\ssrc=/i.test(attrs) || body.trim() === '') continue
    out.push(`'sha256-${createHash('sha256').update(body, 'utf8').digest('base64')}'`)
  }
  return out
}

const hashes = new Set()
for (const name of ['index.html', '200.html', '404.html']) {
  const file = new URL(name, pub)
  if (existsSync(file)) inlineHashes(readFileSync(file, 'utf8')).forEach((h) => hashes.add(h))
}

const csp = [
  "default-src 'self'",
  `script-src 'self' ${[...hashes].sort().join(' ')}`.trim(),
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self'",
  "connect-src 'self'",
  "manifest-src 'self'",
  "worker-src 'self'",
  "object-src 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  "frame-ancestors 'none'",
].join('; ')

writeFileSync(new URL('nginx-csp.conf', root), `add_header Content-Security-Policy "${csp}" always;\n`)
writeFileSync(new URL('csp.txt', root), csp + '\n')

for (const required of ['index.html', 'sw.js', 'manifest.webmanifest']) {
  if (!existsSync(new URL(required, pub))) {
    console.error(`postbuild: missing ${required} in .output/public`)
    process.exit(1)
  }
}
console.log(`postbuild: CSP written with ${hashes.size} inline script hash(es)`)
