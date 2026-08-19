/**
 * sla-plugin.js — the shell. This is the only file a host page loads.
 *
 *   <script defer src="https://cdn.example.com/sla-plugin.js"></script>
 *   <script>
 *     window.onload = () => { window.sla = new SignPlugin({ token: '...' }); };
 *   </script>
 *
 * Deliberately dependency-free and framework-free, because every kilobyte here
 * is paid for by every visitor to the host site, including the ones who never
 * press the button. The expensive half (three.js + the avatar) lives in
 * sla-core.js and is injected only on first activation.
 *
 * Everything this file touches on the host page is namespaced or cleaned up:
 *   • one global, `window.SignPlugin`, plus the instance the caller assigns
 *   • all CSS scoped under .sla-root
 *   • every listener removable via destroy()
 */
(function () {
  'use strict';

  var VERSION = '1.0.0';
  var INTEGRITY_KEY = 'b7e1d4a90c2f8356ab1e7f04c95d2381e6a0b8c7d4f13592e8a76b0c5d9f2431';
  var API_KEY = 'sla_pub_9f2c41d7e8b64a05';

  var DEFAULTS = {
    token: '',
    apiEndpoint: '',
    language: 'ase',
    avatar: 'NOOR',
    side: 'right',
    width: 260,
    height: 400,
    zIndex: 999999,
    maxTextSize: 800,
    collectorMode: 'click',          // 'click' | 'selection'
    exceptions: ['.sla-exception', 'input', 'textarea', 'select'],
    clickables: ['a', 'button', '[role=button]', '[role=link]'],
    remoteConfigEnabled: true,
    doNotTrack: false,
    autoOpen: false,
    onStatus: null,
  };

  // Letters the offline dictionary must cover, so fingerspelling keeps working
  // with no network. Mirrors the reference plugin's bundled dictionary.
  var OFFLINE_IDS = (function () {
    var ids = [];
    'abcdefghijklmnopqrstuvwxyz0123456789'.split('').forEach(function (c) { ids.push('@' + c); });
    ['@space', '@period', '@question', '@exclamation', '@at', '@rest'].forEach(function (c) { ids.push(c); });
    return ids;
  }());

  /* ── tiny helpers ──────────────────────────────────────────────────────── */
  function el(tag, cls, html) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (html != null) n.innerHTML = html;
    return n;
  }

  function normaliseText(s) {
    return String(s || '')
      .replace(/&nbsp;/g, ' ')
      .replace(/[\t]+/g, ' ')
      .replace(/ +/g, ' ')
      .replace(/\n+/g, '\n')
      .trim();
  }

  function extractText(node) {
    if (!node) return '';
    var t = '';
    switch ((node.tagName || '').toLowerCase()) {
      case 'select':
        t = node.options && node.options[node.selectedIndex]
          ? node.options[node.selectedIndex].innerText : ''; break;
      case 'img':
        t = node.getAttribute('data-sla-alt') || node.alt || ''; break;
      case 'button':
        t = node.innerText || node.textContent || node.value || ''; break;
      default:
        t = node.getAttribute && node.getAttribute('data-sla-alt')
          ? node.getAttribute('data-sla-alt')
          : (node.innerText || node.textContent || '');
    }
    if (!t && node.getAttribute) {
      var attrs = ['aria-label', 'aria-description', 'title'];
      for (var i = 0; i < attrs.length; i++) {
        var v = node.getAttribute(attrs[i]);
        if (v) { t = v; break; }
      }
    }
    return normaliseText(t);
  }

  function matchesAny(node, selectors) {
    if (!node || !node.matches) return false;
    for (var i = 0; i < selectors.length; i++) {
      try { if (node.matches(selectors[i])) return true; } catch (e) { /* bad selector */ }
    }
    return false;
  }

  function closestMatch(node, selectors) {
    while (node && node !== document.body) {
      if (matchesAny(node, selectors)) return node;
      node = node.parentElement;
    }
    return null;
  }

  /* ── HMAC-SHA256 via WebCrypto ─────────────────────────────────────────
   * Same construction as the reference plugin. Worth being explicit: the key
   * is in this file, so anyone can read it. This is an anti-abuse speed bump,
   * not authentication. The control that matters is the per-token origin
   * allowlist enforced server-side.
   */
  function hmacHex(message, secret) {
    if (!(window.crypto && window.crypto.subtle)) return Promise.resolve('');
    var enc = new TextEncoder();
    return window.crypto.subtle
      .importKey('raw', enc.encode(secret), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign'])
      .then(function (key) { return window.crypto.subtle.sign('HMAC', key, enc.encode(message)); })
      .then(function (buf) {
        return Array.prototype.map
          .call(new Uint8Array(buf), function (b) { return b.toString(16).padStart(2, '0'); })
          .join('');
      })
      .catch(function () { return ''; });
  }

  /* ── styles, scoped under .sla-root ────────────────────────────────────── */
  var CSS = [
    '.sla-root{position:fixed;bottom:18px;z-index:var(--sla-z);font:14px/1.5 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;color:var(--sla-fg)}',
    '.sla-root.sla-right{right:18px}.sla-root.sla-left{left:18px}',
    '.sla-root *{box-sizing:border-box}',
    '.sla-btn{width:56px;height:56px;border-radius:50%;border:none;cursor:pointer;background:var(--sla-primary);color:var(--sla-primary-fg);box-shadow:0 6px 24px rgba(0,0,0,.32);display:grid;place-items:center;transition:transform .18s}',
    '.sla-btn:hover{transform:scale(1.07)}.sla-btn:focus-visible{outline:3px solid var(--sla-primary);outline-offset:3px}',
    '.sla-panel{position:absolute;bottom:0;width:var(--sla-w);background:var(--sla-bg);border:1px solid var(--sla-border);border-radius:var(--sla-r);box-shadow:0 18px 48px rgba(0,0,0,.45);overflow:hidden;display:none}',
    '.sla-root.sla-right .sla-panel{right:0}.sla-root.sla-left .sla-panel{left:0}',
    '.sla-root.sla-open .sla-panel{display:block}.sla-root.sla-open .sla-btn{display:none}',
    '.sla-head{display:flex;align-items:center;gap:8px;padding:8px 10px;background:var(--sla-bg2);border-bottom:1px solid var(--sla-border)}',
    '.sla-title{font-weight:600;font-size:12.5px;letter-spacing:.2px;flex:1}',
    '.sla-icon{width:28px;height:28px;border:none;border-radius:8px;background:transparent;color:var(--sla-fg);cursor:pointer;display:grid;place-items:center}',
    '.sla-icon:hover{background:var(--sla-bg3)}.sla-icon[aria-pressed=true]{background:var(--sla-primary);color:var(--sla-primary-fg)}',
    '.sla-stage{position:relative;width:100%;height:var(--sla-h);background:linear-gradient(170deg,var(--sla-bg3),var(--sla-bg))}',
    '.sla-stage canvas{display:block}',
    '.sla-cap{min-height:34px;padding:7px 10px;font-size:12.5px;background:var(--sla-bg2);border-top:1px solid var(--sla-border);color:var(--sla-fg2);word-break:break-word}',
    '.sla-cap b{color:var(--sla-fg);font-weight:600}',
    '.sla-bar{display:flex;gap:6px;align-items:center;padding:7px 10px;background:var(--sla-bg2);border-top:1px solid var(--sla-border)}',
    '.sla-bar select{flex:0 0 auto;background:var(--sla-bg3);color:var(--sla-fg);border:1px solid var(--sla-border);border-radius:7px;padding:3px 6px;font-size:12px}',
    '.sla-spacer{flex:1}',
    '.sla-load{position:absolute;inset:0;display:grid;place-items:center;background:var(--sla-bg);gap:10px;text-align:center;padding:16px}',
    '.sla-load small{display:block;color:var(--sla-fg2);font-size:12px;margin-top:8px}',
    '.sla-prog{width:120px;height:4px;border-radius:2px;background:var(--sla-bg3);overflow:hidden;margin:0 auto}',
    '.sla-prog i{display:block;height:100%;width:0;background:var(--sla-primary);transition:width .2s}',
    '.sla-hover{cursor:pointer !important;text-decoration:underline !important;text-decoration-color:var(--sla-primary) !important;text-underline-offset:3px}',
    '.sla-invalid{cursor:not-allowed !important;text-decoration:underline wavy #F2555A !important}',
    '.sla-sr{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap}',
  ].join('\n');

  var ICONS = {
    hand: '<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M11 11V4.5a1.5 1.5 0 0 1 3 0V11"/><path d="M14 10.5V3.8a1.5 1.5 0 0 1 3 0V11"/><path d="M17 11V6a1.5 1.5 0 0 1 3 0v7.5a7.5 7.5 0 0 1-7.5 7.5h-1a6.5 6.5 0 0 1-4.6-1.9L3 15.2a1.6 1.6 0 0 1 2.3-2.3L8 15.4"/><path d="M11 11V7.2a1.5 1.5 0 0 0-3 0V15"/></svg>',
    close: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>',
    replay: '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/></svg>',
    cursor: '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round"><path d="M4 4l7 16 2-7 7-2z"/></svg>',
    palette: '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><circle cx="9" cy="9.5" r="1.2" fill="currentColor"/><circle cx="15" cy="9.5" r="1.2" fill="currentColor"/><circle cx="9.5" cy="15" r="1.2" fill="currentColor"/></svg>',
  };

  var SKIN_TONES = ['#F2D3B8', '#E4B98F', '#D9A87E', '#B3805A', '#8A5A3B', '#5C3A24'];
  var CLOTH_TONES = ['#1F5AB8', '#146B54', '#8A2F4A', '#4A3B8A', '#2E3748', '#B4560F'];

  /* ══════════════════════════════════════════════════════════════════════ */
  function SignPlugin(userConfig) {
    if (window.__slaInstance) {
      throw new Error('SignPlugin: an instance is already active on this page.');
    }
    window.__slaInstance = true;

    this.version = VERSION;
    this.config = Object.assign({}, DEFAULTS, userConfig || {});
    if (!this.config.token) throw new Error('SignPlugin: token is required.');

    this.state = {
      open: false, coreReady: false, loading: false, busy: false,
      lastText: '', lastItems: null, session: null, speed: 1,
    };
    this._bound = [];
    this._hovered = null;

    var self = this;
    this._boot().catch(function (e) {
      console.error('[SLA] boot failed', e);
      self._status('error');
    });
  }

  SignPlugin.version = VERSION;

  SignPlugin.prototype._status = function (s, detail) {
    this.state.status = s;
    if (typeof this.config.onStatus === 'function') this.config.onStatus(s, detail);
    this._announce(s === 'translating' ? 'Translating' : s === 'ready' ? 'Ready' : s);
  };

  /* ── boot: remote config, then UI ──────────────────────────────────────── */
  SignPlugin.prototype._boot = function () {
    var self = this;
    var p = Promise.resolve(null);
    if (this.config.remoteConfigEnabled) {
      p = fetch(this.config.apiEndpoint + '/v1/config/' + encodeURIComponent(this.config.token) + '.json')
        .then(function (r) { return r.ok ? r.json() : null; })
        .catch(function () { return null; });
    }
    return p.then(function (remote) {
      // precedence: defaults < remote config < what the caller passed in
      if (remote && remote.config) {
        self.config = Object.assign({}, DEFAULTS, remote.config, userOverrides(self.config));
        self.remoteConfig = remote.config;
      }
      self._buildUI();
      self._attachCollector();
      self._status('idle');
      if (self.config.autoOpen) self.open();
    });

    function userOverrides(cfg) {
      var out = {};
      for (var k in userConfigKeys) if (cfg[k] !== undefined) out[k] = cfg[k];
      return out;
    }
  };

  var userConfigKeys = { token: 1, apiEndpoint: 1, language: 1, avatar: 1, autoOpen: 1, onStatus: 1 };

  /* ── UI ────────────────────────────────────────────────────────────────── */
  SignPlugin.prototype._buildUI = function () {
    var self = this;
    var cfg = this.config;
    var cs = cfg.customStyle || {};
    var dark = cs.dark || {};

    var style = el('style');
    style.textContent = CSS;
    document.head.appendChild(style);
    this._style = style;

    var root = el('div', 'sla-root sla-' + (cfg.side === 'left' ? 'left' : 'right'));
    root.setAttribute('data-sla-version', VERSION);
    root.style.setProperty('--sla-z', cfg.zIndex);
    root.style.setProperty('--sla-w', cfg.width + 'px');
    root.style.setProperty('--sla-h', cfg.height + 'px');
    root.style.setProperty('--sla-r', (cs.borderRadius || 14) + 'px');
    root.style.setProperty('--sla-primary', cs.primary || '#1F5AB8');
    root.style.setProperty('--sla-primary-fg', cs.primaryFg || '#fff');
    root.style.setProperty('--sla-bg', dark.neutral1 || '#11151F');
    root.style.setProperty('--sla-bg2', '#161B26');
    root.style.setProperty('--sla-bg3', '#1E2533');
    root.style.setProperty('--sla-border', '#2A3242');
    root.style.setProperty('--sla-fg', dark.neutralText || '#E6EAF2');
    root.style.setProperty('--sla-fg2', '#9BA6BC');

    var btn = el('button', 'sla-btn', ICONS.hand);
    btn.setAttribute('aria-label', 'Open sign language translator');
    btn.onclick = function () { self.open(); };

    var panel = el('div', 'sla-panel');
    var head = el('div', 'sla-head');
    head.appendChild(el('span', 'sla-title', 'Sign Language'));

    var modeBtn = el('button', 'sla-icon', ICONS.cursor);
    modeBtn.title = 'Toggle click / selection capture';
    modeBtn.setAttribute('aria-pressed', String(cfg.collectorMode === 'selection'));
    modeBtn.onclick = function () {
      cfg.collectorMode = cfg.collectorMode === 'click' ? 'selection' : 'click';
      modeBtn.setAttribute('aria-pressed', String(cfg.collectorMode === 'selection'));
      self._setCaption('Capture mode: <b>' + cfg.collectorMode + '</b>');
    };

    var paletteBtn = el('button', 'sla-icon', ICONS.palette);
    paletteBtn.title = 'Change appearance';
    paletteBtn.onclick = function () { self.cycleAppearance(); };

    var replayBtn = el('button', 'sla-icon', ICONS.replay);
    replayBtn.title = 'Replay';
    replayBtn.onclick = function () { self.replay(); };

    var closeBtn = el('button', 'sla-icon', ICONS.close);
    closeBtn.title = 'Close';
    closeBtn.onclick = function () { self.close(); };

    head.appendChild(modeBtn); head.appendChild(paletteBtn);
    head.appendChild(replayBtn); head.appendChild(closeBtn);

    var stage = el('div', 'sla-stage');
    var loading = el('div', 'sla-load',
      '<div><div style="font-size:13px">Loading avatar…</div>'
      + '<div class="sla-prog"><i></i></div>'
      + '<small>three.js + rig, first time only</small></div>');
    stage.appendChild(loading);

    var cap = el('div', 'sla-cap', 'Click any text on the page to translate it.');
    var bar = el('div', 'sla-bar');
    var speed = el('select');
    [['0.5', 'Slow'], ['1', 'Normal'], ['1.5', 'Fast'], ['2', 'Very fast']]
      .forEach(function (o) {
        var opt = el('option', null, o[1]); opt.value = o[0];
        if (o[0] === '1') opt.selected = true;
        speed.appendChild(opt);
      });
    speed.setAttribute('aria-label', 'Signing speed');
    speed.onchange = function () { self.setSpeed(parseFloat(speed.value)); };
    bar.appendChild(speed);
    bar.appendChild(el('span', 'sla-spacer'));
    var badge = el('span', null, '');
    badge.style.cssText = 'font-size:11px;color:var(--sla-fg2)';
    bar.appendChild(badge);

    panel.appendChild(head); panel.appendChild(stage);
    panel.appendChild(cap); panel.appendChild(bar);
    root.appendChild(panel); root.appendChild(btn);

    var live = el('div', 'sla-sr');
    live.setAttribute('aria-live', 'polite');
    root.appendChild(live);

    document.body.appendChild(root);
    this.el = { root: root, panel: panel, stage: stage, cap: cap, badge: badge, live: live,
      loading: loading, prog: loading.querySelector('i'), btn: btn };
  };

  SignPlugin.prototype._announce = function (msg) {
    if (this.el && this.el.live) this.el.live.textContent = msg;
  };

  SignPlugin.prototype._setCaption = function (html) {
    if (this.el) this.el.cap.innerHTML = html;
  };

  /* ── text capture ──────────────────────────────────────────────────────── */
  SignPlugin.prototype._attachCollector = function () {
    var self = this;
    var cfg = this.config;

    function isOurs(node) {
      return node && node.closest && node.closest('.sla-root');
    }

    function onClick(e) {
      if (!self.state.open || cfg.collectorMode !== 'click') return;
      if (isOurs(e.target)) return;
      if (closestMatch(e.target, cfg.exceptions)) return;

      var clickable = closestMatch(e.target, cfg.clickables);
      var text = extractText(e.target);
      if (!text) return;

      // Links and buttons keep working — we translate without hijacking them.
      if (!clickable) { e.preventDefault(); e.stopPropagation(); }
      self.translate(text);
    }

    var selTimer = null;
    function onSelection() {
      if (!self.state.open || cfg.collectorMode !== 'selection') return;
      clearTimeout(selTimer);
      selTimer = setTimeout(function () {
        var sel = window.getSelection();
        if (!sel || sel.isCollapsed || !sel.rangeCount) return;
        var text = normaliseText(sel.toString());
        if (text) self.translate(text);
      }, 120);
    }

    function onOver(e) {
      if (!self.state.open || cfg.collectorMode !== 'click') return;
      if (isOurs(e.target) || closestMatch(e.target, cfg.exceptions)) return;
      var text = extractText(e.target);
      if (!text) return;
      self._hovered = e.target;
      e.target.classList.add(text.length > cfg.maxTextSize ? 'sla-invalid' : 'sla-hover');
    }

    function onOut(e) {
      if (e.target && e.target.classList) {
        e.target.classList.remove('sla-hover');
        e.target.classList.remove('sla-invalid');
      }
    }

    this._listen(document, 'click', onClick, true);
    this._listen(document, 'mouseup', onSelection);
    this._listen(document, 'touchend', onSelection);
    this._listen(document, 'selectionchange', onSelection);
    this._listen(document, 'mouseover', onOver);
    this._listen(document, 'mouseout', onOut);
  };

  SignPlugin.prototype._listen = function (target, evt, fn, capture) {
    target.addEventListener(evt, fn, !!capture);
    this._bound.push([target, evt, fn, !!capture]);
  };

  /* ── core loading ──────────────────────────────────────────────────────── */
  SignPlugin.prototype._ensureCore = function () {
    var self = this;
    if (this._corePromise) return this._corePromise;

    this._corePromise = new Promise(function (resolve, reject) {
      if (window.SLACore) return resolve();
      var s = document.createElement('script');
      s.src = self.config.apiEndpoint + '/sla-core.js';
      s.id = 'sla-core-script';
      s.onload = resolve;
      s.onerror = function () { reject(new Error('failed to load sla-core.js')); };
      document.body.appendChild(s);
    }).then(function () {
      self.core = new window.SLACore({
        apiEndpoint: self.config.apiEndpoint,
        avatar: self.config.avatar,
        language: self.config.language,
        width: self.config.width,
        height: self.config.height,
      });
      self.el.stage.appendChild(self.core.element);
      self.core.resize(self.config.width, self.config.height);
      return self.core.load(function (pct) {
        self.el.prog.style.width = Math.round(pct * 100) + '%';
      });
    }).then(function () {
      return self._auth();
    }).then(function () {
      return self.core.loadOfflineDictionary(OFFLINE_IDS);
    }).then(function () {
      if (self.session && self.session.custom) self.core.setAppearance(self.session.custom);
      self.el.loading.style.display = 'none';
      self.state.coreReady = true;
      self._status('ready');
      self._setCaption('Ready. Click any text on the page.');
      self.el.badge.textContent = self.core.rig.boneNames.length + ' bones · '
        + self.core.rig.morphNames.length + ' morphs';
    });

    return this._corePromise;
  };

  /* ── API calls ─────────────────────────────────────────────────────────── */
  SignPlugin.prototype._post = function (path, payload) {
    var self = this;
    var body = Object.assign({}, payload, {
      token: this.config.token,
      doNotTrack: String(!!this.config.doNotTrack),
    });
    if (this.session && this.session.sessionId) body.sessionId = this.session.sessionId;

    var message = JSON.stringify({ origin: location.origin, payload: body });
    return hmacHex(message, INTEGRITY_KEY + this.config.token).then(function (sig) {
      return fetch(self.config.apiEndpoint + path, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json; charset=utf-8',
          'X-Api-Key': API_KEY,
          'X-Integrity': sig,
        },
        body: JSON.stringify(body),
      });
    }).then(function (r) {
      return r.json().then(function (j) {
        if (!r.ok) { var e = new Error(j.error || 'http_' + r.status); e.status = r.status; e.body = j; throw e; }
        return j;
      });
    });
  };

  SignPlugin.prototype._auth = function () {
    var self = this;
    return this._post('/v1/sign/auth', {
      avatar: this.config.avatar,
      lang: this.config.language,
    }).then(function (r) { self.session = r; return r; });
  };

  /* ── the main flow ─────────────────────────────────────────────────────── */
  SignPlugin.prototype.translate = function (text) {
    var self = this;
    text = normaliseText(text);
    if (!text) return Promise.resolve(false);

    if (text.length > this.config.maxTextSize) {
      this._setCaption('Text too long (' + text.length + ' / ' + this.config.maxTextSize + ' characters).');
      this._signOffline(['@question']);
      return Promise.resolve(false);
    }
    if (this.state.busy) return Promise.resolve(false);

    this.state.busy = true;
    this.state.lastText = text;
    this._setCaption('<b>' + escapeHtml(text.slice(0, 160)) + '</b>');
    this._status('translating');

    return this.open()
      .then(function () {
        return self._post('/v1/sign/translate', {
          q: text, lang: self.config.language,
        });
      })
      .then(function (wire) {
        var items = flatten(wire);
        if (!items.length) items = fingerspellLocal(text);
        self.state.lastItems = items;
        self._playItems(items);
        return true;
      })
      .catch(function (err) {
        // Network or server failure must not be a dead end: fall back to the
        // offline fingerspelling dictionary, which needs no network at all.
        console.warn('[SLA] translate failed, fingerspelling offline:', err.message);
        var items = fingerspellLocal(text);
        self.state.lastItems = items;
        self._setCaption('<b>' + escapeHtml(text.slice(0, 120)) + '</b><br><span style="color:#F5A524">offline — fingerspelling</span>');
        self._playItems(items);
        return false;
      })
      .then(function (v) { self.state.busy = false; return v; });
  };

  SignPlugin.prototype._playItems = function (items) {
    var self = this;
    return this._ensureCore().then(function () {
      self._status('signing');
      self.core.play(items.concat(['@rest']), {
        onDone: function () { self._status('ready'); },
      });
    });
  };

  SignPlugin.prototype._signOffline = function (ids) {
    var self = this;
    return this._ensureCore().then(function () { self.core.play(ids.concat(['@rest']), {}); });
  };

  SignPlugin.prototype.replay = function () {
    if (this.state.lastItems) this._playItems(this.state.lastItems);
    return this;
  };

  SignPlugin.prototype.setSpeed = function (v) {
    this.state.speed = v;
    if (this.core) this.core.setSpeed(v);
    return this;
  };

  SignPlugin.prototype.cycleAppearance = function () {
    this._tone = ((this._tone || 0) + 1) % SKIN_TONES.length;
    var look = { skin: SKIN_TONES[this._tone], cloth: CLOTH_TONES[this._tone] };
    if (this.core) this.core.setAppearance(look);
    this._setCaption('Appearance: skin <b>' + look.skin + '</b> · outfit <b>' + look.cloth + '</b>');
    return look;
  };

  SignPlugin.prototype.open = function () {
    var self = this;
    if (this.state.open) return this._corePromise || Promise.resolve();
    this.state.open = true;
    this.el.root.classList.add('sla-open');
    return this._ensureCore().catch(function (e) {
      self._setCaption('Could not load the 3D engine.');
      throw e;
    });
  };

  SignPlugin.prototype.close = function () {
    this.state.open = false;
    this.el.root.classList.remove('sla-open');
    if (this.core) this.core.stop();
    return this;
  };

  SignPlugin.prototype.destroy = function () {
    this._bound.forEach(function (b) { b[0].removeEventListener(b[1], b[2], b[3]); });
    this._bound.length = 0;
    if (this.core) this.core.destroy();
    if (this.el && this.el.root) this.el.root.remove();
    if (this._style) this._style.remove();
    delete window.__slaInstance;
    return this;
  };

  /* ── helpers used above ────────────────────────────────────────────────── */
  function flatten(wire) {
    var out = [];
    (function walk(x) {
      if (Array.isArray(x)) x.forEach(walk);
      else if (typeof x === 'string' && x) out.push(x);
    }(wire));
    return out;
  }

  function fingerspellLocal(text) {
    var map = { '.': '@period', '?': '@question', '!': '@exclamation', '@': '@at', ' ': '@space' };
    var out = [];
    text.toLowerCase().split('').forEach(function (ch) {
      if (/[a-z0-9]/.test(ch)) out.push('@' + ch);
      else if (map[ch]) out.push(map[ch]);
    });
    return out;
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }

  window.SignPlugin = SignPlugin;
}());
