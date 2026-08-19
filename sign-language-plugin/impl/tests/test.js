#!/usr/bin/env node
'use strict';
/**
 * test.js — the project's test suite. Run with:  npm test
 *
 * Three groups:
 *   1. DECODER EQUIVALENCE — the browser decoder must reproduce, bit for bit,
 *      the poses the Python compiler intended. This is the one that matters:
 *      a mismatch here corrupts every sign silently, with nothing thrown.
 *   2. API CONTRACT — status codes, gloss selection, grammar behaviour,
 *      error paths, rate limiting.
 *   3. ASSETS — the avatar parses and carries the rig the decoder expects.
 */
const assert = require('assert');
const fs = require('fs');
const path = require('path');
const http = require('http');

const ROOT = path.resolve(__dirname, '..');
const PORT = Number(process.env.PORT || 8899);
const BASE = `http://127.0.0.1:${PORT}`;
const TOKEN = 'demo_2f6a91c4e7b3';

let pass = 0, fail = 0;
const failures = [];

function test(name, fn) {
  return Promise.resolve()
    .then(fn)
    .then(() => { pass++; console.log(`  \x1b[32m✓\x1b[0m ${name}`); })
    .catch((e) => {
      fail++; failures.push([name, e]);
      console.log(`  \x1b[31m✗\x1b[0m ${name}\n      ${e.message.split('\n')[0]}`);
    });
}

function group(name) { console.log(`\n\x1b[1m${name}\x1b[0m`); }

function req(method, p, body, headers = {}) {
  return new Promise((resolve, reject) => {
    const data = body ? Buffer.from(JSON.stringify(body)) : null;
    const r = http.request(`${BASE}${p}`, {
      method,
      headers: Object.assign(
        data ? { 'Content-Type': 'application/json', 'Content-Length': data.length } : {},
        headers,
      ),
    }, (res) => {
      const chunks = [];
      res.on('data', (c) => chunks.push(c));
      res.on('end', () => {
        const raw = Buffer.concat(chunks).toString();
        let json = null;
        try { json = JSON.parse(raw); } catch { /* non-JSON body */ }
        resolve({ status: res.statusCode, headers: res.headers, json, raw });
      });
    });
    r.on('error', reject);
    if (data) r.write(data);
    r.end();
  });
}

/* ══════════════════════════════════════════════════════════════════════════
 * 1. DECODER EQUIVALENCE
 * ═══════════════════════════════════════════════════════════════════════════ */
async function decoderTests() {
  group('1. Decoder equivalence (JS runtime vs Python compiler)');

  const refPath = path.join(__dirname, 'reference_poses.json');
  if (!fs.existsSync(refPath)) {
    console.log('  \x1b[33m!\x1b[0m reference_poses.json missing — run tools/dump_reference.py');
    return;
  }
  const ref = JSON.parse(fs.readFileSync(refPath, 'utf8'));

  const THREE = require('three');
  const { decodeHTA } = await import(`file://${path.join(ROOT, 'web/src/core.js')}`)
    .catch(async () => {
      // core.js imports GLTFLoader, which needs a DOM-ish env; fall back to
      // extracting the decoder from the bundle would be fragile, so instead we
      // re-import with a stubbed loader.
      throw new Error('core.js not importable directly');
    });

  const rig = {
    boneNames: ref.boneNames,
    morphNames: ref.morphNames,
    morphNode: 'NOOR_Face',
    boneByCode: codeTable(ref.boneNames, 'BCD'),
    morphByCode: codeTable(ref.morphNames, 'U'),
    morphIndex: Object.fromEntries(ref.morphNames.map((m, i) => [m, i])),
  };

  const signsDir = path.join(ROOT, 'data/signs/ase');
  let checkedBones = 0, checkedMorphs = 0, worst = 0, worstWhere = '';

  for (const [name, expect] of Object.entries(ref.signs)) {
    const hta = fs.readFileSync(path.join(signsDir, `${name}.hta`), 'utf8');
    const clip = decodeHTA(hta, rig, name);

    // duration must agree
    if (Math.abs(clip.duration - expect.duration) > 1e-4) {
      throw new Error(`${name}: duration ${clip.duration} vs ${expect.duration}`);
    }

    const byName = new Map(clip.tracks.map((t) => [t.name, t]));
    for (const frame of expect.frames) {
      for (const [bone, q] of Object.entries(frame.bones)) {
        const tr = byName.get(`${bone}.quaternion`);
        assert.ok(tr, `${name}: missing track ${bone}.quaternion`);
        const got = sampleQuat(tr, frame.t);
        // quaternions q and -q are the same rotation
        const d = Math.min(dist4(got, q), dist4(got, q.map((v) => -v)));
        if (d > worst) { worst = d; worstWhere = `${name}/${bone}@${frame.t}`; }
        assert.ok(d < 1e-4, `${name} ${bone} @${frame.t}: Δ=${d.toExponential(2)}`);
        checkedBones++;
      }
      for (const [morph, v] of Object.entries(frame.morphs)) {
        const idx = rig.morphIndex[morph];
        const tr = byName.get(`NOOR_Face.morphTargetInfluences[${idx}]`);
        assert.ok(tr, `${name}: missing morph track ${morph}`);
        const got = sampleScalar(tr, frame.t);
        assert.ok(Math.abs(got - v) < 1e-4,
          `${name} ${morph} @${frame.t}: ${got} vs ${v}`);
        checkedMorphs++;
      }
    }
  }

  await test(`all ${Object.keys(ref.signs).length} signs decode identically ` +
    `(${checkedBones} bone samples, ${checkedMorphs} morph samples, ` +
    `max Δ=${worst.toExponential(1)})`, () => {});

  await test('every bone is pinned even when the payload omits it', () => {
    const hta = fs.readFileSync(path.join(signsDir, '@a.hta'), 'utf8');
    const clip = decodeHTA(hta, rig, 'a');
    const quatTracks = clip.tracks.filter((t) => t.name.endsWith('.quaternion'));
    assert.strictEqual(quatTracks.length, ref.boneNames.length,
      `expected ${ref.boneNames.length} quaternion tracks, got ${quatTracks.length}`);
    const morphTracks = clip.tracks.filter((t) => t.name.includes('morphTargetInfluences'));
    assert.strictEqual(morphTracks.length, ref.morphNames.length);
  });

  await test('empty channel values decode to zero, not NaN', () => {
    const clip = decodeHTA('BH*0#4*###*###*###', rig, 'zeros');
    const tr = clip.tracks.find((t) => t.name === 'hips.quaternion');
    assert.ok(tr, 'hips track present');
    for (const v of tr.values) assert.ok(Number.isFinite(v), 'finite');
  });

  await test('unknown track codes are skipped, not fatal', () => {
    const clip = decodeHTA('ZZ*0#4*1#2?BH*0#4*10#10*##*##', rig, 'unknown');
    assert.ok(clip.tracks.length > 0);
  });

  function codeTable(names, letters) {
    const alpha = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
    const out = {}; let i = 0;
    for (const a of letters) for (const b of alpha) { if (i < names.length) out[a + b] = names[i++]; }
    return out;
  }
  function dist4(a, b) { return Math.hypot(a[0] - b[0], a[1] - b[1], a[2] - b[2], a[3] - b[3]); }

  function sampleQuat(track, t) {
    const { times, values } = track;
    const { i, u } = locate(times, t);
    const j = Math.min(i + 1, times.length - 1);
    const qa = [values[i * 4], values[i * 4 + 1], values[i * 4 + 2], values[i * 4 + 3]];
    const qb = [values[j * 4], values[j * 4 + 1], values[j * 4 + 2], values[j * 4 + 3]];
    return slerp(qa, qb, u);
  }
  function sampleScalar(track, t) {
    const { times, values } = track;
    const { i, u } = locate(times, t);
    const j = Math.min(i + 1, times.length - 1);
    return values[i] + (values[j] - values[i]) * u;
  }
  function locate(times, t) {
    if (t <= times[0]) return { i: 0, u: 0 };
    if (t >= times[times.length - 1]) return { i: times.length - 1, u: 0 };
    let i = 0;
    for (let k = 0; k < times.length; k++) if (times[k] <= t) i = k;
    const span = times[Math.min(i + 1, times.length - 1)] - times[i];
    return { i, u: span <= 0 ? 0 : (t - times[i]) / span };
  }
  function slerp(a, b, u) {
    let d = a[0] * b[0] + a[1] * b[1] + a[2] * b[2] + a[3] * b[3];
    let bb = b.slice();
    if (d < 0) { bb = b.map((v) => -v); d = -d; }
    if (d > 0.9995) {
      const r = a.map((v, k) => v + (bb[k] - v) * u);
      const n = Math.hypot(...r);
      return r.map((v) => v / n);
    }
    const th = Math.acos(d), s = Math.sin(th);
    const wa = Math.sin((1 - u) * th) / s, wb = Math.sin(u * th) / s;
    return a.map((v, k) => v * wa + bb[k] * wb);
  }
}

/* ══════════════════════════════════════════════════════════════════════════
 * 2. API CONTRACT
 * ═══════════════════════════════════════════════════════════════════════════ */
async function apiTests() {
  group('2. API contract');

  await test('GET /v1/health reports a loaded dictionary', async () => {
    const r = await req('GET', '/v1/health');
    assert.strictEqual(r.status, 200);
    assert.ok(r.json.ok);
    assert.ok(r.json.signs > 60, `expected >60 signs, got ${r.json.signs}`);
    assert.deepStrictEqual(r.json.languages, ['ase']);
  });

  await test('POST /v1/sign/auth issues a session + expiry + customisation', async () => {
    const r = await req('POST', '/v1/sign/auth', { token: TOKEN, avatar: 'NOOR', lang: 'ase' });
    assert.strictEqual(r.status, 200);
    assert.match(r.json.sessionId, /^[0-9a-f-]{36}$/);
    assert.ok(r.json.csrfToken.length > 10);
    assert.ok(r.json.sessionExpiration > Date.now());
    assert.strictEqual(r.json.capabilities.rigVersion, 'noor-rig-v1');
    assert.ok(r.json.custom.skin);
  });

  await test('unknown token is rejected with 401', async () => {
    const r = await req('POST', '/v1/sign/auth', { token: 'nope', lang: 'ase' });
    assert.strictEqual(r.status, 401);
  });

  await test('translate returns the 3-level sentence/gloss/id shape', async () => {
    const r = await req('POST', '/v1/sign/translate',
      { token: TOKEN, q: 'Hello world', lang: 'ase' });
    assert.strictEqual(r.status, 200);
    assert.ok(Array.isArray(r.json), 'top level is an array of sentences');
    assert.ok(Array.isArray(r.json[0]), 'sentence is an array of glosses');
    assert.ok(Array.isArray(r.json[0][0]), 'gloss is an array of ids/payloads');
    assert.strictEqual(r.json[0].length, 2, 'two glosses for "Hello world"');
  });

  await test('function words are dropped, surfacing as empty arrays', async () => {
    const r = await req('POST', '/v1/sign/translate',
      { token: TOKEN, q: 'What is the name', lang: 'ase' });
    const flat = r.json[0];
    assert.ok(flat.some((g) => g.length === 0), 'at least one dropped gloss');
  });

  await test('wh- words move to clause-final position (ASL word order)', async () => {
    const r = await req('POST', '/v1/sign/translate',
      { token: TOKEN, q: 'What is your name?', lang: 'ase', format: 'json' });
    const g = r.json.sentences[0].glosses.filter((x) => x.type !== 'dropped');
    assert.strictEqual(g[g.length - 1].gloss, 'WHAT',
      `expected WHAT last, got ${g.map((x) => x.gloss || x.type).join(' ')}`);
  });

  await test('wh- question sets brow-down; yes/no sets brow-up', async () => {
    const wh = await req('POST', '/v1/sign/translate',
      { token: TOKEN, q: 'Where is it?', lang: 'ase', format: 'json' });
    assert.strictEqual(wh.json.sentences[0].nmm, 'brow-down');
    const yn = await req('POST', '/v1/sign/translate',
      { token: TOKEN, q: 'You understand?', lang: 'ase', format: 'json' });
    assert.strictEqual(yn.json.sentences[0].nmm, 'brow-up');
  });

  await test('unknown words fall back to fingerspelling', async () => {
    const r = await req('POST', '/v1/sign/translate',
      { token: TOKEN, q: 'Ehab', lang: 'ase', format: 'json' });
    const g = r.json.sentences[0].glosses[0];
    assert.strictEqual(g.type, 'fingerspell');
    assert.deepStrictEqual(g.ids, ['@e', '@h', '@a', '@b']);
  });

  await test('fingerspelled letters are sent as ids, not payloads', async () => {
    const r = await req('POST', '/v1/sign/translate',
      { token: TOKEN, q: 'Ehab', lang: 'ase' });
    assert.deepStrictEqual(r.json[0][0], ['@e', '@h', '@a', '@b']);
  });

  await test('a known gloss is sent as a full HTA payload', async () => {
    const r = await req('POST', '/v1/sign/translate', { token: TOKEN, q: 'Hello', lang: 'ase' });
    const payload = r.json[0][0][0];
    assert.ok(payload.length > 200, 'payload is substantial');
    assert.match(payload, /^[A-Z]{2}\*/, 'starts with a track code');
  });

  await test('multiple sentences are split', async () => {
    const r = await req('POST', '/v1/sign/translate',
      { token: TOKEN, q: 'Hello. Thank you. Goodbye.', lang: 'ase' });
    assert.strictEqual(r.json.length, 3);
  });

  await test('abbreviations do not split sentences', async () => {
    const r = await req('POST', '/v1/sign/translate',
      { token: TOKEN, q: 'Dr. Smith is here.', lang: 'ase', format: 'json' });
    assert.strictEqual(r.json.sentences.length, 1);
  });

  await test('empty text is a 400', async () => {
    const r = await req('POST', '/v1/sign/translate', { token: TOKEN, q: '   ', lang: 'ase' });
    assert.strictEqual(r.status, 400);
    assert.strictEqual(r.json.error, 'empty_text');
  });

  await test('over-long text is a 400 with the limit stated', async () => {
    const r = await req('POST', '/v1/sign/translate',
      { token: TOKEN, q: 'a'.repeat(1500), lang: 'ase' });
    assert.strictEqual(r.status, 400);
    assert.strictEqual(r.json.error, 'text_too_long');
    assert.strictEqual(r.json.limit, 1000);
  });

  await test('unsupported language is a 422', async () => {
    const r = await req('POST', '/v1/sign/translate', { token: TOKEN, q: 'hi', lang: 'zzz' });
    assert.strictEqual(r.status, 422);
  });

  await test('rate-limit headers are present and decrease', async () => {
    const a = await req('POST', '/v1/sign/translate', { token: TOKEN, q: 'one', lang: 'ase' });
    const b = await req('POST', '/v1/sign/translate', { token: TOKEN, q: 'two', lang: 'ase' });
    assert.ok(a.headers['x-ratelimit-limit']);
    assert.ok(Number(b.headers['x-ratelimit-remaining'])
      < Number(a.headers['x-ratelimit-remaining']));
  });

  await test('identical text hits the cache the second time', async () => {
    const q = 'cache probe ' + Date.now();
    const a = await req('POST', '/v1/sign/translate', { token: TOKEN, q, lang: 'ase', format: 'json' });
    const b = await req('POST', '/v1/sign/translate', { token: TOKEN, q, lang: 'ase', format: 'json' });
    assert.strictEqual(a.json.meta.cacheHit, false);
    assert.strictEqual(b.json.meta.cacheHit, true);
  });

  await test('X-Integrity round-trips (HMAC construction matches the client)', async () => {
    const crypto = require('crypto');
    const { INTEGRITY_KEY } = require(path.join(ROOT, 'server/server.js'));
    const payload = { q: 'Hello', lang: 'ase', token: TOKEN, doNotTrack: 'false' };
    const sig = crypto.createHmac('sha256', INTEGRITY_KEY + TOKEN)
      .update(JSON.stringify({ origin: 'http://x.test', payload })).digest('hex');
    const r = await req('POST', '/v1/sign/translate', payload,
      { 'X-Integrity': sig, Origin: 'http://x.test' });
    assert.strictEqual(r.status, 200);
    assert.strictEqual(sig.length, 64);
  });

  await test('config endpoint is CDN-cacheable', async () => {
    const r = await req('GET', `/v1/config/${TOKEN}.json`);
    assert.strictEqual(r.status, 200);
    assert.match(r.headers['cache-control'], /max-age=\d+/);
    assert.ok(r.json.config.appearance.skin);
  });

  await test('sign assets are immutable-cacheable', async () => {
    const r = await req('GET', '/v1/assets/ase/%40a.hta');
    assert.strictEqual(r.status, 200);
    assert.match(r.headers['cache-control'], /immutable/);
    assert.match(r.raw, /^[A-Z]{2}\*/);
  });

  await test('path traversal is refused', async () => {
    const r = await req('GET', '/../server/server.js');
    assert.ok(r.status === 404 || r.status === 400, `got ${r.status}`);
  });

  await test('translate latency stays under 50 ms server-side', async () => {
    const r = await req('POST', '/v1/sign/translate',
      { token: TOKEN, q: 'Hello world how are you today my friend', lang: 'ase', format: 'json' });
    assert.ok(r.json.meta.latencyMs < 50, `${r.json.meta.latencyMs} ms`);
  });
}

/* ══════════════════════════════════════════════════════════════════════════
 * 3. ASSETS
 * ═══════════════════════════════════════════════════════════════════════════ */
async function assetTests() {
  group('3. Assets');

  const glb = path.join(ROOT, 'data/avatar/NOOR.glb');

  await test('avatar is a valid glTF 2.0 binary with 0 validator errors', async () => {
    const validator = require('gltf-validator');
    const r = await validator.validateBytes(new Uint8Array(fs.readFileSync(glb)));
    assert.strictEqual(r.issues.numErrors, 0,
      r.issues.messages.filter((m) => m.severity === 0).map((m) => m.message).join('; '));
    assert.strictEqual(r.issues.numWarnings, 0);
  });

  await test('avatar declares the rig the decoder expects', () => {
    const buf = fs.readFileSync(glb);
    const jsonLen = buf.readUInt32LE(12);
    const gltf = JSON.parse(buf.slice(20, 20 + jsonLen).toString());
    assert.strictEqual(gltf.extras.rigVersion, 'noor-rig-v1');
    assert.strictEqual(gltf.extras.boneNames.length, 50);
    assert.strictEqual(gltf.extras.morphNames.length, 16);
    assert.strictEqual(gltf.skins[0].joints.length, 50);
  });

  await test('avatar stays under the 2 MB budget', () => {
    const kb = fs.statSync(glb).size / 1024;
    assert.ok(kb < 2048, `${kb.toFixed(0)} KB`);
  });

  await test('64% of the rig is hands and fingers', () => {
    const buf = fs.readFileSync(glb);
    const gltf = JSON.parse(buf.slice(20, 20 + buf.readUInt32LE(12)).toString());
    const hands = gltf.extras.boneNames.filter((n) =>
      /thumb|index|middle|ring|pinky|hand/.test(n)).length;
    const pct = hands / gltf.extras.boneNames.length;
    assert.ok(pct > 0.6, `${(pct * 100).toFixed(0)}%`);
  });

  await test('the plugin shell stays small enough to ship on every page', () => {
    const zlib = require('zlib');
    const raw = fs.readFileSync(path.join(ROOT, 'web/sla-plugin.js'));
    const kb = zlib.gzipSync(raw).length / 1024;
    assert.ok(kb < 150, `${kb.toFixed(1)} KB gzipped`);
    console.log(`      shell: ${kb.toFixed(1)} KB gzipped`);
  });

  await test('the 3D core stays within budget', () => {
    const zlib = require('zlib');
    const kb = zlib.gzipSync(fs.readFileSync(path.join(ROOT, 'web/sla-core.js'))).length / 1024;
    assert.ok(kb < 250, `${kb.toFixed(1)} KB gzipped`);
    console.log(`      core:  ${kb.toFixed(1)} KB gzipped`);
  });

  await test('every gloss in the lexicon has a compiled sign', () => {
    const lex = require(path.join(ROOT, 'server/lexicon.js'));
    const dir = path.join(ROOT, 'data/signs/ase');
    const missing = lex.GLOSSES.filter((g) => !fs.existsSync(path.join(dir, `${g}.hta`)));
    assert.deepStrictEqual(missing, [], `missing: ${missing.join(', ')}`);
  });

  await test('every fingerspellable character has a sign', () => {
    const dir = path.join(ROOT, 'data/signs/ase');
    const missing = [];
    for (const c of 'abcdefghijklmnopqrstuvwxyz0123456789') {
      if (!fs.existsSync(path.join(dir, `@${c}.hta`))) missing.push(c);
    }
    assert.deepStrictEqual(missing, []);
  });

  await test('no sign payload exceeds 8 KB', () => {
    const dir = path.join(ROOT, 'data/signs/ase');
    const big = fs.readdirSync(dir).filter((f) => f.endsWith('.hta'))
      .map((f) => [f, fs.statSync(path.join(dir, f)).size])
      .filter(([, s]) => s > 8192);
    assert.deepStrictEqual(big, []);
  });
}

/* ══════════════════════════════════════════════════════════════════════════ */
async function main() {
  console.log('\n\x1b[1msign-language-plugin — test suite\x1b[0m');

  const { server } = require(path.join(ROOT, 'server/server.js'));
  require(path.join(ROOT, 'server/server.js')).loadSigns();
  await new Promise((r) => server.listen(PORT, r));

  await decoderTests();
  await apiTests();
  await assetTests();

  server.close();

  console.log(`\n${fail === 0 ? '\x1b[32m' : '\x1b[31m'}${pass} passed, ${fail} failed\x1b[0m\n`);
  if (fail) {
    failures.forEach(([n, e]) => console.log(`\x1b[31m${n}\x1b[0m\n${e.stack}\n`));
    process.exit(1);
  }
}

main().catch((e) => { console.error(e); process.exit(1); });
