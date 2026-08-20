#!/usr/bin/env node
// Start loopback labdns serve with testdata/web/ and production SPA assets
// overlaid into the embed so Playwright hits the same origin as REST.

import { spawn, spawnSync } from 'node:child_process'
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  statSync,
  writeFileSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const webDir = resolve(here, '..')
const repoRoot = resolve(webDir, '..')
const fixtureDir = join(repoRoot, 'testdata', 'web')
const distDir = join(webDir, 'dist')
const host = process.env.LABDNS_E2E_HOST ?? '127.0.0.1'
const mgmtPort = process.env.LABDNS_E2E_MGMT_PORT ?? '18765'
const dnsListen = process.env.LABDNS_E2E_DNS_LISTEN ?? `${host}:0`
const mgmtListen = `${host}:${mgmtPort}`

function fail(msg, extra) {
  process.stderr.write(`start-labdns: ${msg}\n`)
  if (extra) {
    process.stderr.write(String(extra).endsWith('\n') ? String(extra) : `${extra}\n`)
  }
  process.exit(1)
}

function walkFiles(dir) {
  const out = []
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) {
      out.push(...walkFiles(p))
    } else {
      out.push(p)
    }
  }
  return out
}

function ensureDist() {
  if (existsSync(join(distDir, 'index.html'))) {
    return
  }
  process.stderr.write('start-labdns: web/dist missing; running npm run build\n')
  const built = spawnSync('npm', ['run', 'build'], {
    cwd: webDir,
    stdio: 'inherit',
    env: process.env,
  })
  if (built.status !== 0) {
    fail('npm run build failed')
  }
}

function writeOverlay(work) {
  const embedDist = join(repoRoot, 'internal', 'web', 'dist')
  const replace = {}
  for (const file of walkFiles(distDir)) {
    const rel = relative(distDir, file)
    replace[join(embedDist, rel)] = file
  }
  if (!replace[join(embedDist, 'index.html')]) {
    fail('web/dist/index.html missing after build')
  }
  const overlayPath = join(work, 'overlay.json')
  writeFileSync(overlayPath, JSON.stringify({ Replace: replace }))
  return overlayPath
}

function writeConfig(work) {
  const src = join(fixtureDir, 'config.yaml')
  const tokens = join(fixtureDir, 'tokens.json')
  if (!existsSync(src) || !existsSync(tokens)) {
    fail(`fixture missing under ${fixtureDir}`)
  }
  const dest = join(work, 'config.yaml')
  const raw = readFileSync(src, 'utf8')
  if (!raw.includes('secretRef: tokens.json')) {
    fail('fixture config must use secretRef: tokens.json so e2e can absolutize it')
  }
  writeFileSync(dest, raw.replace('secretRef: tokens.json', `secretRef: ${tokens}`))
  copyFileSync(tokens, join(work, 'tokens.json'))
  return dest
}

function buildLabdns(work, overlayPath) {
  const bin = join(work, 'labdns')
  process.stderr.write('start-labdns: go build labdns with SPA overlay\n')
  const built = spawnSync(
    'go',
    ['build', '-overlay', overlayPath, '-o', bin, './cmd/labdns'],
    {
      cwd: repoRoot,
      stdio: 'inherit',
      env: { ...process.env, CGO_ENABLED: '0' },
    },
  )
  if (built.status !== 0) {
    fail('go build ./cmd/labdns failed (Go is required for operator e2e)')
  }
  return bin
}

function start() {
  if (!existsSync(join(repoRoot, 'go.mod'))) {
    fail(`repo root not found from ${here}`)
  }
  ensureDist()
  const work = mkdtempSync(join(tmpdir(), 'labdns-e2e-'))
  mkdirSync(work, { recursive: true })
  const overlayPath = writeOverlay(work)
  const configPath = writeConfig(work)
  const bin = buildLabdns(work, overlayPath)

  process.stderr.write(`start-labdns: serve --config ${configPath} management ${mgmtListen}\n`)
  const child = spawn(
    bin,
    [
      'serve',
      '--config',
      configPath,
      '--dns-listen',
      dnsListen,
      '--management-listen',
      mgmtListen,
    ],
    {
      cwd: work,
      env: process.env,
      stdio: ['ignore', 'pipe', 'pipe'],
    },
  )

  const prefix = (buf) => {
    const text = buf.toString()
    for (const line of text.split(/\n/)) {
      if (line !== '') {
        process.stderr.write(`labdns: ${line}\n`)
      }
    }
  }
  child.stdout.on('data', prefix)
  child.stderr.on('data', prefix)
  child.on('exit', (code, signal) => {
    if (signal) {
      process.exit(0)
    }
    fail(`labdns exited ${code}`)
  })

  const stop = () => {
    if (child.exitCode === null && !child.killed) {
      child.kill('SIGTERM')
    }
  }
  process.on('SIGINT', stop)
  process.on('SIGTERM', stop)
  process.on('exit', stop)

  const deadline = Date.now() + 30_000
  const live = `http://${mgmtListen}/v1/health/live`
  const root = `http://${mgmtListen}/`
  ;(async () => {
    while (Date.now() < deadline) {
      try {
        const res = await fetch(live)
        if (res.ok) {
          const spa = await fetch(root)
          const ctype = spa.headers.get('content-type') || ''
          if (!spa.ok || !ctype.includes('text/html')) {
            fail(`SPA GET / returned ${spa.status} ${ctype} (want 200 text/html; overlay may have failed)`)
          }
          process.stderr.write(`start-labdns: ready ${root}\n`)
          return
        }
      } catch {
        // bind in progress
      }
      await new Promise((r) => setTimeout(r, 100))
    }
    fail(`timeout waiting for ${live}`)
  })().catch((err) => fail(String(err)))
}

start()
