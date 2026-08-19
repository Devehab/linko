#!/usr/bin/env node
'use strict';
/**
 * server.js — the whole backend, with zero npm dependencies.
 *
 * Endpoints (deliberately shaped like the reference API so the reverse-
 * engineering notes in ../../docs stay applicable):
 *
 *   POST /v1/sign/auth            open a session, return customisation
 *   POST /v1/sign/translate       text -> glosses -> HTA payloads
 *   GET  /v1/config/:token.json   remote config, CDN-cacheable
 *   GET  /v1/assets/:lang/:id.hta one sign, immutable, cacheable forever
 *   GET  /v1/avatar/:name.glb     the avatar
 *   GET  /v1/health               liveness + dictionary stats
 *
 * Security posture, stated honestly:
 *   The X-Integrity HMAC is computed client-side from a key that ships in the
 *   bundle, so it is an anti-abuse speed bump, NOT authentication — same as the
 *   original. The real control is the per-token origin allowlist enforced here
 *   in `checkOrigin`, plus rate limiting. See docs/01-ARCHITECTURE.md §3.b.3.
 *
 * Run:  node server/server.js [--port 8787]
 */
const http = require('http');
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const { URL } = require('url');

const lexicon = require('./lexicon');

const ROOT = path.resolve(__dirname, '..');
const DATA = path.join(ROOT, 'data');
const WEB = path.join(ROOT, 'web');
const DEMO = path.join(ROOT, 'demo');

const API_KEY = 'sla_pub_9f2c41d7e8b64a05';
const INTEGRITY_KEY = 'b7e1d4a90c2f8356ab1e7f04c95d2381e6a0b8c7d4f13592e8a76b0c5d9f2431';
const SESSION_TTL_MS = 15 * 60 * 1000;
const RATE_LIMIT = 300;
const RATE_WINDOW_MS = 60 * 1000;
const MAX_TEXT = 1000;

// ── tenants ────────────────────────────────────────────────────────────────
const TENANTS = {
  demo_2f6a91c4e7b3: {
    name: 'Demo tenant',
    // '*' is for local development only. In production this is the real control.
    allowedOrigins: ['*'],
    plan: 'free',
    rateLimit: RATE_LIMIT,
    config: {
      side: 'right',
      avatar: 'NOOR',
      opacity: 100,
      textEnabled: true,
      maxTextSize: 800,
      collectorMode: 'click',
      exceptions: ['.sla-exception', 'input', 'textarea', 'code', 'pre'],
      clickables: ['a', 'button', '[role=button]', '[role=link]'],
      appearance: {
        skin: '#D9A87E',
        cloth: '#1F5AB8',
        hair: '#2B2118',
      },
      customStyle: {
        primary: '#1F5AB8',
        primaryFg: '#FFFFFF',
        borderRadius: 14,
        dark: { neutral1: '#11151F', neutralText: '#E6EAF2', primary: '#7BB4FF' },
      },
    },
  },
};

// ── in-memory stores (Redis in production) ─────────────────────────────────
const sessions = new Map();
const rate = new Map();
const cache = new Map();
const stats = { translate: 0, cacheHits: 0, auth: 0, fingerspelled: 0, signed: 0, dropped: 0 };

// ── sign asset loading ─────────────────────────────────────────────────────
const signs = new Map();      // "lang/id" -> hta string
let manifest = {};

function loadSigns() {
  const langs = fs.existsSync(path.join(DATA, 'signs'))
    ? fs.readdirSync(path.join(DATA, 'signs')) : [];
  for (const lang of langs) {
    const dir = path.join(DATA, 'signs', lang);
    if (!fs.statSync(dir).isDirectory()) continue;
    for (const f of fs.readdirSync(dir)) {
      if (f === 'manifest.json') {
        manifest[lang] = JSON.parse(fs.readFileSync(path.join(dir, f), 'utf8'));
        continue;
      }
      if (!f.endsWith('.hta')) continue;
      signs.set(`${lang}/${f.slice(0, -4)}`, fs.readFileSync(path.join(dir, f), 'utf8'));
    }
  }
  const n = signs.size;
  const bytes = [...signs.values()].reduce((a, s) => a + s.length, 0);
  console.log(`  dictionary: ${n} signs, ${(bytes / 1024).toFixed(1)} KB`);
}

function getSign(lang, id) {
  return signs.get(`${lang}/${id}`) || null;
}

// ── helpers ────────────────────────────────────────────────────────────────
function hmac(message, secret) {
  return crypto.createHmac('sha256', secret).update(message).digest('hex');
}

/**
 * Recompute the client's X-Integrity signature.
 * Returns true/false; a mismatch is logged but — matching the reference
 * behaviour — is not by itself fatal, because the key is public anyway.
 */
function checkIntegrity(req, token, payload) {
  const got = req.headers['x-integrity'];
  if (!got) return false;
  const origin = req.headers.origin || '';
  const want = hmac(JSON.stringify({ origin, payload }), INTEGRITY_KEY + token);
  return crypto.timingSafeEqual(Buffer.from(got.padEnd(64, '0').slice(0, 64)),
    Buffer.from(want.slice(0, 64)));
}

/** This is the control that actually matters. */
function checkOrigin(tenant, req) {
  const list = tenant.allowedOrigins || [];
  if (list.includes('*')) return true;
  const origin = req.headers.origin;
  if (!origin) return true;                       // same-origin / curl
  return list.includes(origin);
}

function rateOk(key, limit) {
  const now = Date.now();
  let e = rate.get(key);
  if (!e || now > e.reset) { e = { count: 0, reset: now + RATE_WINDOW_MS }; rate.set(key, e); }
  e.count += 1;
  return { ok: e.count <= limit, remaining: Math.max(0, limit - e.count), reset: e.reset };
}

function send(res, status, body, headers = {}) {
  const isBuf = Buffer.isBuffer(body);
  const payload = isBuf ? body : Buffer.from(typeof body === 'string' ? body : JSON.stringify(body));
  res.writeHead(status, {
    'Content-Type': isBuf ? 'application/octet-stream' : 'application/json; charset=utf-8',
    'Content-Length': payload.length,
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'Content-Type, X-Api-Key, X-Integrity, X-Session-Id',
    'Access-Control-Expose-Headers': 'X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset',
    'X-Content-Type-Options': 'nosniff',
    ...headers,
  });
  res.end(payload);
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    let n = 0; const chunks = [];
    req.on('data', (c) => {
      n += c.length;
      if (n > 64 * 1024) { reject(new Error('payload too large')); req.destroy(); return; }
      chunks.push(c);
    });
    req.on('end', () => {
      try { resolve(chunks.length ? JSON.parse(Buffer.concat(chunks).toString()) : {}); }
      catch (e) { reject(new Error('invalid JSON')); }
    });
    req.on('error', reject);
  });
}

// ── route handlers ─────────────────────────────────────────────────────────
async function handleAuth(req, res) {
  const body = await readBody(req);
  const tenant = TENANTS[body.token];
  if (!tenant) return send(res, 401, { error: 'unknown_token' });
  if (!checkOrigin(tenant, req)) return send(res, 403, { error: 'origin_not_allowed' });

  checkIntegrity(req, body.token, body);          // observed, not enforced

  const sessionId = body.sessionId && sessions.has(body.sessionId)
    ? body.sessionId : crypto.randomUUID();
  const session = {
    token: body.token,
    lang: body.lang || 'ase',
    avatar: body.avatar || tenant.config.avatar,
    doNotTrack: String(body.doNotTrack) === 'true',
    csrfToken: crypto.randomBytes(18).toString('base64url'),
    expires: Date.now() + SESSION_TTL_MS,
  };
  sessions.set(sessionId, session);
  stats.auth += 1;

  send(res, 200, {
    sessionId,
    csrfToken: session.csrfToken,
    sessionExpiration: session.expires,
    custom: tenant.config.appearance,
    capabilities: {
      maxTextSize: tenant.config.maxTextSize,
      supportedLanguages: Object.keys(manifest),
      avatars: ['NOOR'],
      rigVersion: (manifest[session.lang] || {}).rigVersion,
      fps: (manifest[session.lang] || {}).fps,
    },
  });
}

async function handleTranslate(req, res) {
  const body = await readBody(req);
  const tenant = TENANTS[body.token];
  if (!tenant) return send(res, 401, { error: 'unknown_token' });
  if (!checkOrigin(tenant, req)) return send(res, 403, { error: 'origin_not_allowed' });

  const q = typeof body.q === 'string' ? body.q : '';
  if (!q.trim()) return send(res, 400, { error: 'empty_text' });
  if (q.length > MAX_TEXT) {
    return send(res, 400, { error: 'text_too_long', limit: MAX_TEXT, got: q.length });
  }

  const lang = body.lang || 'ase';
  if (!manifest[lang]) return send(res, 422, { error: 'unsupported_language', lang });

  const rl = rateOk(`${body.token}:${req.socket.remoteAddress}`, tenant.rateLimit);
  const rlHeaders = {
    'X-RateLimit-Limit': String(tenant.rateLimit),
    'X-RateLimit-Remaining': String(rl.remaining),
    'X-RateLimit-Reset': String(Math.floor(rl.reset / 1000)),
  };
  if (!rl.ok) return send(res, 429, { error: 'rate_limited' }, rlHeaders);

  checkIntegrity(req, body.token, body);

  const t0 = process.hrtime.bigint();
  const key = crypto.createHash('sha256')
    .update(lang + ' ' + lexicon.normalise(q)).digest('hex');
  let result = cache.get(key);
  const hit = Boolean(result);
  if (hit) stats.cacheHits += 1;
  if (!result) {
    result = lexicon.translate(q, { maxTextSize: tenant.config.maxTextSize });
    if (cache.size > 5000) cache.clear();
    cache.set(key, result);
  }

  for (const s of result.sentences) {
    for (const g of s.glosses) {
      if (g.type === 'sign') stats.signed += 1;
      else if (g.type === 'fingerspell') stats.fingerspelled += 1;
      else stats.dropped += 1;
    }
  }
  stats.translate += 1;

  // ── expand gloss ids into HTA payloads ────────────────────────────────
  const missing = new Set();
  const wire = result.sentences.map((s) => s.glosses.map((g) => {
    if (!g.ids.length) return [];                          // dropped word
    const out = [];
    for (const id of g.ids) {
      const hta = getSign(lang, id);
      if (hta) {
        // A leading '@' means "the client already has this in its offline
        // dictionary" — send the id, not the payload. Same trick the original
        // uses, and it is why a 16-letter word still costs almost nothing.
        out.push(id.startsWith('@') ? id : hta);
      } else {
        missing.add(id);
        for (const ch of lexicon.fingerspell(id)) out.push(ch);
      }
    }
    return out;
  }));

  const ms = Number(process.hrtime.bigint() - t0) / 1e6;

  if (body.format === 'json') {
    return send(res, 200, {
      sentences: result.sentences.map((s) => ({
        source: s.source,
        nmm: s.nmm,
        glosses: s.glosses.map((g) => ({
          type: g.type, gloss: g.gloss || null, source: g.source, ids: g.ids,
        })),
      })),
      meta: { engine: 'rule-v1', latencyMs: +ms.toFixed(2), cacheHit: hit,
        truncated: result.truncated, missing: [...missing] },
    }, rlHeaders);
  }

  send(res, 200, wire, rlHeaders);
}

function handleConfig(res, token) {
  const tenant = TENANTS[token];
  if (!tenant) return send(res, 404, { error: 'unknown_token' });
  send(res, 200, { config: tenant.config, version: 1, updatedAt: new Date().toISOString() },
    { 'Cache-Control': 'public, max-age=300, stale-while-revalidate=86400' });
}

function handleAsset(res, lang, id) {
  const hta = getSign(lang, decodeURIComponent(id));
  if (!hta) return send(res, 404, { error: 'unknown_sign' });
  send(res, 200, hta, {
    'Content-Type': 'text/plain; charset=utf-8',
    'Cache-Control': 'public, max-age=31536000, immutable',
  });
}

const MIME = {
  '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8', '.json': 'application/json; charset=utf-8',
  '.glb': 'model/gltf-binary', '.png': 'image/png', '.svg': 'image/svg+xml',
  '.hta': 'text/plain; charset=utf-8', '.map': 'application/json',
};

function serveFile(res, file, extraHeaders = {}) {
  fs.readFile(file, (err, buf) => {
    if (err) return send(res, 404, { error: 'not_found' });
    send(res, 200, buf, {
      'Content-Type': MIME[path.extname(file)] || 'application/octet-stream',
      ...extraHeaders,
    });
  });
}

/** Reject anything that escapes the served root. */
function safeJoin(root, rel) {
  const p = path.resolve(root, '.' + path.posix.normalize('/' + rel));
  return p.startsWith(path.resolve(root)) ? p : null;
}

// ── router ─────────────────────────────────────────────────────────────────
const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, `http://${req.headers.host || 'localhost'}`);
  const p = url.pathname;

  if (req.method === 'OPTIONS') return send(res, 204, '');

  try {
    if (req.method === 'POST' && p === '/v1/sign/auth') return await handleAuth(req, res);
    if (req.method === 'POST' && p === '/v1/sign/translate') return await handleTranslate(req, res);

    if (req.method === 'GET') {
      let m;
      if ((m = p.match(/^\/v1\/config\/([\w-]+)\.json$/))) return handleConfig(res, m[1]);
      if ((m = p.match(/^\/v1\/assets\/([\w-]+)\/(.+)\.hta$/))) return handleAsset(res, m[1], m[2]);
      if ((m = p.match(/^\/v1\/avatar\/([\w-]+)\.glb$/))) {
        return serveFile(res, path.join(DATA, 'avatar', `${m[1]}.glb`),
          { 'Cache-Control': 'public, max-age=31536000, immutable' });
      }
      if (p === '/v1/health') {
        return send(res, 200, {
          ok: true, uptimeSec: Math.floor(process.uptime()),
          signs: signs.size, languages: Object.keys(manifest),
          glosses: lexicon.GLOSSES.length, sessions: sessions.size, stats,
        });
      }
      if (p === '/v1/dictionary') {
        return send(res, 200, { manifest, glosses: lexicon.LEMMAS });
      }

      // static: /  -> demo, /sla-*.js -> web
      if (p === '/' || p === '/index.html') return serveFile(res, path.join(DEMO, 'index.html'));
      for (const root of [WEB, DEMO]) {
        const f = safeJoin(root, p);
        if (f && fs.existsSync(f) && fs.statSync(f).isFile()) return serveFile(res, f);
      }
    }
    send(res, 404, { error: 'not_found', path: p });
  } catch (e) {
    send(res, 400, { error: 'bad_request', detail: e.message });
  }
});

// ── boot ───────────────────────────────────────────────────────────────────
function main() {
  const argPort = process.argv.indexOf('--port');
  const port = argPort > -1 ? Number(process.argv[argPort + 1]) : Number(process.env.PORT) || 8787;
  loadSigns();
  server.listen(port, () => {
    console.log(`\n  sign-language-plugin backend`);
    console.log(`  http://localhost:${port}/            demo page`);
    console.log(`  http://localhost:${port}/v1/health   status`);
    console.log(`  token: demo_2f6a91c4e7b3\n`);
  });
}

if (require.main === module) main();
module.exports = { server, TENANTS, INTEGRITY_KEY, API_KEY, hmac, loadSigns, getSign };
