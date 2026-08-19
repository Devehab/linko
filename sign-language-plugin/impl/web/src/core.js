/**
 * core.js — the 3D engine. Bundled to web/sla-core.js and injected lazily by
 * sla-plugin.js the first time the user actually turns the translator on.
 *
 * That split is the single most important performance decision in the whole
 * design: visitors who never use the translator never download three.js or the
 * avatar. The shell stays tiny; this file is the expensive half.
 *
 * Responsibilities
 *   • decode HTA into THREE.AnimationClip
 *   • load and display the avatar
 *   • run a sign queue with cross-faded transitions
 *   • expose appearance controls (skin tone, clothing)
 */
import * as THREE from 'three';
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js';

const FPS = 24;
const ANGLE_DIVISOR = 2.7;
const TRANSITION = 0.18;
const MORPH_PREFIX = 'UVWXYZ';

/* ══════════════════════════════════════════════════════════════════════════
 * HTA decoder
 *
 * This is a line-for-line counterpart of sample_hta() in tools/preview.py.
 * The two are kept equivalent on purpose and tests/test_decoder.js asserts it —
 * if the browser and the compiler ever disagree about what a payload means,
 * every sign in the dictionary is silently wrong.
 * ═══════════════════════════════════════════════════════════════════════════ */

const _euler = new THREE.Euler();
const _quat = new THREE.Quaternion();

function parseChannel(str) {
  // "" -> 0 ; "12#-4#" -> [12, -4, 0]
  return str.split('#').map((v) => (v === '' ? 0 : parseInt(v, 10) || 0));
}

function toQuaternions(xs, ys, zs, count) {
  const X = parseChannel(xs);
  const Y = parseChannel(ys);
  const Z = parseChannel(zs);
  const out = new Float32Array(count * 4);
  let lx = 0, ly = 0, lz = 0;
  for (let i = 0; i < count; i++) {
    if (i < X.length) lx = X[i];
    if (i < Y.length) ly = Y[i];
    if (i < Z.length) lz = Z[i];
    _euler.set(
      THREE.MathUtils.degToRad(lx * ANGLE_DIVISOR),
      THREE.MathUtils.degToRad(ly * ANGLE_DIVISOR),
      THREE.MathUtils.degToRad(lz * ANGLE_DIVISOR),
      'XYZ',
    );
    _quat.setFromEuler(_euler);
    out[i * 4] = _quat.x; out[i * 4 + 1] = _quat.y;
    out[i * 4 + 2] = _quat.z; out[i * 4 + 3] = _quat.w;
  }
  return out;
}

function toInfluences(str, count) {
  const src = parseChannel(str);
  const out = new Float32Array(count);
  let last = 0;
  for (let i = 0; i < count; i++) {
    if (i < src.length) last = src[i] / 100;
    out[i] = last;
  }
  return out;
}

/**
 * decodeHTA(hta, rig) -> THREE.AnimationClip
 *
 * `rig` supplies the code tables and the target node names. Every bone and
 * morph the payload does NOT mention is explicitly pinned back to its rest
 * value — skip that and poses accumulate across signs until the avatar folds
 * in on itself after a handful of glosses.
 */
export function decodeHTA(hta, rig, name = 'sign', offset = 0) {
  const tracks = [];
  const seen = new Set();

  for (const raw of hta.split('?')) {
    if (!raw) continue;
    const parts = raw.split('*');
    const code = parts[0];
    const isMorph = MORPH_PREFIX.includes(code[0]);
    const longName = isMorph ? rig.morphByCode[code] : rig.boneByCode[code];
    if (longName === undefined) {
      console.warn(`[SLA] unknown track code ${code}`);
      continue;
    }

    const times = parts[1].split('#')
      .map((v) => (v === '' ? 0 : parseInt(v, 10) || 0) / FPS + offset);
    const t = new Float32Array(times);

    if (isMorph) {
      const idx = rig.morphIndex[longName];
      const trackName = `${rig.morphNode}.morphTargetInfluences[${idx}]`;
      tracks.push(new THREE.NumberKeyframeTrack(trackName, t, toInfluences(parts[2], t.length)));
      seen.add(trackName);
    } else {
      const trackName = `${longName}.quaternion`;
      tracks.push(new THREE.QuaternionKeyframeTrack(
        trackName, t, toQuaternions(parts[2] || '', parts[3] || '', parts[4] || '', t.length),
      ));
      seen.add(trackName);
    }
  }

  // pin everything not mentioned back to rest
  for (const bone of rig.boneNames) {
    const n = `${bone}.quaternion`;
    if (!seen.has(n)) {
      tracks.push(new THREE.QuaternionKeyframeTrack(n, [0], [0, 0, 0, 1]));
    }
  }
  for (let i = 0; i < rig.morphNames.length; i++) {
    const n = `${rig.morphNode}.morphTargetInfluences[${i}]`;
    if (!seen.has(n)) tracks.push(new THREE.NumberKeyframeTrack(n, [0], [0]));
  }

  const clip = new THREE.AnimationClip(name, -1, tracks);
  clip.resetDuration();
  return clip;
}

/* ══════════════════════════════════════════════════════════════════════════
 * SLACore
 * ═══════════════════════════════════════════════════════════════════════════ */
export class SLACore {
  constructor(opts = {}) {
    this.opts = Object.assign({
      apiEndpoint: '', width: 260, height: 400, alpha: true, antialias: true,
      fpsLimit: 0, avatar: 'NOOR', language: 'ase', pixelRatioCap: 1.75,
    }, opts);

    this.loadStatus = 'UNLOADED';
    this.queue = [];
    this.playing = false;
    this.speed = 1;
    this.clipCache = new Map();
    this.offline = new Map();          // fingerspelling dictionary, cached
    this._listeners = {};
    this._raf = null;
    this._lastFrame = 0;

    this.element = document.createElement('div');
    this.element.style.cssText = 'width:100%;height:100%;position:relative;overflow:hidden';

    this.renderer = new THREE.WebGLRenderer({
      alpha: this.opts.alpha, antialias: this.opts.antialias, powerPreference: 'low-power',
    });
    this.renderer.outputColorSpace = THREE.SRGBColorSpace;
    this.renderer.setClearColor(0x000000, 0);
    this.element.appendChild(this.renderer.domElement);
    this.renderer.domElement.style.cssText = 'display:block;width:100%;height:100%';

    this.scene = new THREE.Scene();
    this.camera = new THREE.PerspectiveCamera(22, 0.65, 0.05, 20);
    this.camera.position.set(0, 1.30, 1.72);
    this.camera.lookAt(0, 1.26, 0);

    const key = new THREE.DirectionalLight(0xffffff, 2.4);
    key.position.set(0.7, 1.6, 1.5);
    const fill = new THREE.DirectionalLight(0xbfd4ff, 0.9);
    fill.position.set(-1.2, 0.6, 0.8);
    const rim = new THREE.DirectionalLight(0xffe8c8, 1.1);
    rim.position.set(0, 1.2, -1.6);
    this.scene.add(key, fill, rim, new THREE.HemisphereLight(0xdfe8ff, 0x2a2f3a, 1.1));

    this.resize(this.opts.width, this.opts.height);
  }

  on(evt, fn) { (this._listeners[evt] ||= []).push(fn); return this; }
  emit(evt, ...a) { (this._listeners[evt] || []).forEach((f) => { try { f(...a); } catch (e) { console.error(e); } }); }

  /* ── loading ─────────────────────────────────────────────────────────── */
  async load(onProgress) {
    this.loadStatus = 'LOADING';
    const url = `${this.opts.apiEndpoint}/v1/avatar/${this.opts.avatar}.glb`;
    const gltf = await new Promise((resolve, reject) => {
      new GLTFLoader().load(url, resolve,
        (e) => { if (e.total) onProgress?.(e.loaded / e.total); }, reject);
    });

    this.root = gltf.scene;
    this.scene.add(this.root);

    const extras = gltf.parser.json.extras || {};
    this.boneNames = extras.boneNames || [];
    this.morphNames = extras.morphNames || [];

    this.materials = {};
    this.faceMesh = null;
    this.root.traverse((o) => {
      if (o.isMesh) {
        o.frustumCulled = false;
        const mats = Array.isArray(o.material) ? o.material : [o.material];
        for (const m of mats) this.materials[m.name] = m;
        if (o.morphTargetInfluences?.length) this.faceMesh = o;
      }
    });
    // the mesh node carrying the morph targets is the animation target
    this.morphNode = this.faceMesh ? (this.faceMesh.parent?.name || this.faceMesh.name) : 'NOOR_Face';
    if (this.faceMesh && !this.faceMesh.name) this.faceMesh.name = this.morphNode;

    this.rig = {
      boneNames: this.boneNames,
      morphNames: this.morphNames,
      morphNode: this.faceMesh ? this.faceMesh.name : this.morphNode,
      boneByCode: buildCodeTable(this.boneNames, 'BCD'),
      morphByCode: buildCodeTable(this.morphNames, 'U'),
      morphIndex: Object.fromEntries(this.morphNames.map((m, i) => [m, i])),
    };

    this.mixer = new THREE.AnimationMixer(this.root);
    this.mixer.addEventListener('finished', () => this._next());

    this.loadStatus = 'LOADED';
    this._startLoop();
    this.emit('loaded');
    return this;
  }

  /* ── appearance ──────────────────────────────────────────────────────── */
  setAppearance({ skin, cloth, face } = {}) {
    const set = (name, hex) => {
      const m = this.materials[name];
      if (m && hex) { m.color.set(hex); m.needsUpdate = true; }
    };
    set('skin', skin); set('cloth', cloth); set('face', face);
    return this;
  }

  setSpeed(v) {
    this.speed = v;
    if (this.mixer) this.mixer.timeScale = v;
    return this;
  }

  resize(w, h) {
    this.opts.width = w; this.opts.height = h;
    const dpr = Math.min(Math.max(window.devicePixelRatio || 1, 1), this.opts.pixelRatioCap);
    this.renderer.setPixelRatio(dpr);
    this.renderer.setSize(w, h, false);
    this.camera.aspect = w / h;
    this.camera.updateProjectionMatrix();
    return this;
  }

  /* ── playback ────────────────────────────────────────────────────────── */
  _clipFor(item) {
    if (this.clipCache.has(item)) return this.clipCache.get(item);
    const hta = item.startsWith('@') ? this.offline.get(item) : item;
    if (!hta) return null;
    const clip = decodeHTA(hta, this.rig, item.startsWith('@') ? item : 'sign');
    this.clipCache.set(item, clip);
    return clip;
  }

  /** Preload the offline fingerspelling dictionary. */
  async loadOfflineDictionary(ids) {
    const base = `${this.opts.apiEndpoint}/v1/assets/${this.opts.language}`;
    await Promise.all(ids.map(async (id) => {
      if (this.offline.has(id)) return;
      try {
        const r = await fetch(`${base}/${encodeURIComponent(id)}.hta`);
        if (r.ok) this.offline.set(id, await r.text());
      } catch { /* offline dictionary is best-effort */ }
    }));
    return this;
  }

  /** Queue a flat list of items (HTA strings or '@id' references) and play. */
  play(items, { onSign, onDone } = {}) {
    if (this.loadStatus !== 'LOADED') return false;
    this.stop();
    this.queue = items.filter(Boolean).map((it) => ({ item: it }));
    this._onSign = onSign; this._onDone = onDone;
    this.playing = true;
    this._prev = null;
    this._next(true);
    return true;
  }

  _next(first = false) {
    const entry = this.queue.shift();
    if (!entry) {
      this.playing = false;
      this._onDone?.();
      this.emit('idle');
      return;
    }
    const clip = this._clipFor(entry.item);
    if (!clip) return this._next(first);

    const action = this.mixer.clipAction(clip);
    action.reset();
    action.setLoop(THREE.LoopOnce, 1);
    action.clampWhenFinished = true;
    action.timeScale = this.speed;

    if (this._prev && !first) action.crossFadeFrom(this._prev, TRANSITION, true);
    action.play();
    this._prev = action;
    this._onSign?.(entry.item);
    this.emit('sign', entry.item);
  }

  stop() {
    this.queue.length = 0;
    this.playing = false;
    if (this.mixer) this.mixer.stopAllAction();
    this._prev = null;
    return this;
  }

  /* ── render loop ─────────────────────────────────────────────────────── */
  _startLoop() {
    const clock = new THREE.Clock();
    const minDelta = this.opts.fpsLimit ? 1 / this.opts.fpsLimit : 0;
    let acc = 0;
    const tick = () => {
      this._raf = requestAnimationFrame(tick);
      const dt = clock.getDelta();
      acc += dt;
      if (minDelta && acc < minDelta) return;
      if (this.mixer) this.mixer.update(acc);
      acc = 0;
      this.renderer.render(this.scene, this.camera);
    };
    tick();
  }

  destroy() {
    if (this._raf) cancelAnimationFrame(this._raf);
    this.stop();
    this.renderer.dispose();
    this.scene.traverse((o) => {
      if (o.geometry) o.geometry.dispose();
      if (o.material) (Array.isArray(o.material) ? o.material : [o.material]).forEach((m) => m.dispose());
    });
    this.element.remove();
  }
}

/** Rebuild the two-letter code table the compiler used. Must match rig.py. */
function buildCodeTable(names, letters) {
  const alpha = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
  const out = {};
  let i = 0;
  for (const a of letters) {
    for (const b of alpha) {
      if (i >= names.length) return out;
      out[a + b] = names[i++];
    }
  }
  return out;
}

if (typeof window !== 'undefined') {
  window.SLACore = SLACore;
  window.SLACore.decodeHTA = decodeHTA;
  window.SLACore.THREE = THREE;
}
