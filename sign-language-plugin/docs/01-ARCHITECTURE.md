# 01 — المعمارية التفصيلية

كل ما في هذا المستند مستخرج من ملفات HAR ومن فكّ حزم JavaScript المحمّلة فعليًا.

---

## 1. طريقة التضمين في الموقع المضيف

هذا هو الكود الحرفي الموجود في `<body>` صفحة `handtalk.me/en/about/`:

```html
<!-- Hand Talk -->
<script defer src="https://plugin.handtalk.me/web/latest/handtalk.min.js"></script>
<script>
  window.onload = function() {
    window.ht = new HT({
      token: "894ad581fb6306c7363cb9c32a5dd9dd",
      avatar: "HUGO",
      language: "en-ase",
      maxTextSize: 800,
      exceptions: ["#rd-close_button-m7cc0x80"]
    });
  }
</script>
<!-- End Hand Talk -->
```

### الملاحظات المعمارية

| الملاحظة | الأثر |
|---|---|
| `defer` وليس `async` | لا يعيق تحليل الـ HTML، ويُنفَّذ بترتيب مضمون |
| `new HT({...})` على `window` | نقطة دخول واحدة، تُسجّل نفسها كـ `window.HT` |
| `token` هو معرّف العميل | يُستخدم للإعداد عن بُعد + التتبّع + توقيع الطلبات |
| `exceptions` | محدّدات CSS تُستثنى من التقاط النص |
| حارس تعدد النسخ | الكود يرمي خطأً إذا وُجدت نسخة نشطة: `window.isHTClassInstanceInWindow` |

---

## 2. طبقات النظام

```
┌─────────────────────────────────────────────────────────────────────┐
│  الطبقة 0 — الموقع المضيف (أي موقع)                                 │
│  سطر <script> واحد                                                  │
└─────────────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────────────┐
│  الطبقة 1 — Plugin Shell   (handtalk.min.js — 546 KB)               │
│  ┌───────────────┐ ┌──────────────┐ ┌────────────────────────────┐  │
│  │ Preact UI     │ │ Text         │ │ Remote Config Provider     │  │
│  │ (زر + قوائم)  │ │ Collector    │ │ يجلب الإعداد من CDN        │  │
│  └───────────────┘ └──────────────┘ └────────────────────────────┘  │
│  ┌───────────────┐ ┌──────────────┐ ┌────────────────────────────┐  │
│  │ zustand store │ │ Translation  │ │ Analytics (داخل iframe)    │  │
│  │ (حالة عامة)   │ │ Client + HMAC│ │ عزل GA عن الموقع المضيف    │  │
│  └───────────────┘ └──────────────┘ └────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │ Addons App (تُحمّل عند الطلب — 177 KB)                          │ │
│  │ تكبير خط · قارئ شاشة · تباين · قناع قراءة · مكبّر · ...          │ │
│  └────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
                              │ يحقن <script> عند التفعيل
┌─────────────────────────────▼───────────────────────────────────────┐
│  الطبقة 2 — 3D Core   (core.min.js — 809 KB)                        │
│  ┌──────────────────┐ ┌──────────────────┐ ┌─────────────────────┐  │
│  │ three.js r117    │ │ Animation Decoder│ │ Sign Emitter        │  │
│  │ WebGLRenderer    │ │ HTA → Clip       │ │ طابور الإشارات      │  │
│  │ GLTFLoader       │ │                  │ │                     │  │
│  └──────────────────┘ └──────────────────┘ └─────────────────────┘  │
│  ┌──────────────────┐ ┌──────────────────┐ ┌─────────────────────┐  │
│  │ AnimationMixer   │ │ Avatar Loader    │ │ Offline Dictionary  │  │
│  │ crossFade 0.2s   │ │ HUGO / MAYA      │ │ 43 مدخلًا (حروف+أرقام)│ │
│  └──────────────────┘ └──────────────────┘ └─────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                              │ HTTPS
┌─────────────────────────────▼───────────────────────────────────────┐
│  الطبقة 3 — Backend  (translation-v3.handtalk.me)                   │
│  POST /api/v2/sign/auth       → جلسة + csrfToken + تخصيص المظهر     │
│  POST /api/v2/sign/translate  → نص → مصفوفة glosses + HTA           │
│  POST /api/v2/addons/ai/synonyms-meanings → تبسيط لغوي (AI)         │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 3. تسلسل التشغيل الكامل (من HAR)

### المرحلة أ — تحميل الصفحة (`Before starting.har`)

يُحمَّل ما مجموعه **8 طلبات** إلى `plugin.handtalk.me`:

| # | المورد | الحجم | الغرض |
|---|---|---|---|
| 1 | `web/latest/handtalk.min.js` | 546 KB | القشرة الأساسية |
| 2 | `remote-config/894ad581...json` | 1.1 KB | **إعداد عن بُعد مفتاحه هو الـ token** |
| 3 | `web/latest/en.d06df75b.js` | 11.8 KB | حزمة الترجمة (i18n) للإنجليزية |
| 4 | `web/latest/sign.d34b1089.js` | 50.3 KB | جزء من واجهة المترجم |
| 5 | `web/latest/sign.53bddf57.js` | 268.7 KB | جزء من واجهة المترجم |
| 6 | `web/latest/sign.c5a1917e.js` | 157.2 KB | جزء من واجهة المترجم |
| 7 | `web/latest/AddonsApp.7d4db4ca.js` | 177.2 KB | تطبيق أدوات إمكانية الوصول |
| 8 | `web/latest/SignTranslationRequestModal...js` | 12.4 KB | نافذة طلب ترجمة |

**درس معماري:** الملفات مُقسّمة (code-split) بواسطة Parcel مع hash في الاسم للتخزين المؤقت الأبدي. الملفات ذات الأسماء المُجزّأة تُحمّل بشكل كسول.

#### الإعداد عن بُعد — البنية الفعلية المرصودة

`GET https://plugin.handtalk.me/remote-config/{token}.json`

```json
{
  "config": {
    "side": "right",
    "exceptions": [],
    "clickables": [],
    "textEnabled": true,
    "align": "default",
    "addonsEnabled": true,
    "avatar": "HUGO",
    "opacity": 100,
    "highContrast": false,
    "colorButton": "a11yColorMain",
    "customStyle": {
      "button": { "primary": "#003087", "primaryFg": "#FFFFFF", "borderRadius": "16px" },
      "light":  { "neutral1": "#EDEDED", "neutral2": "#FFFFFF", "neutralText": "#2E2E2E", "primary": "#C64F01" },
      "dark":   { "neutral1": "#121212", "neutral2": "#1F1F1F", "neutralText": "#FFFFFF", "primary": "#FF8B10" },
      "borderRadius": 16
    },
    "addonsMap": {
      "aiAssistant":  { "synonymsMeanings": true },
      "fontControl":  { "fontSize": true, "textStyle": true, "lineHeight": true, "letterSpacing": true, "highlightLetters": true },
      "navigation":   { "pageSpeech": true, "readerMode": true, "readingMask": true, "readingGuide": true,
                        "highlightLinks": true, "pageStructure": true, "magnifier": true, "hideImages": true,
                        "highlightHeadings": true, "pauseAnimations": true },
      "colorControl": { "contrastMode": true, "saturationMode": true, "pageColors": true }
    }
  },
  "isMajority": false
}
```

> **هذه ميزة تجارية أساسية:** العميل يضع `<script>` مرة واحدة، ثم يغيّر الألوان والميزات من لوحة تحكم دون لمس كوده. طبّقها من اليوم الأول.

**دمج الإعداد** يتم عبر `ParseConfigUseCase` بالأولوية: `defaultConfig` → `remoteConfig` → `userConfig` (ما يُمرَّر في `new HT({})`).

---

### المرحلة ب — عند الضغط على زر التفعيل (`After starting.har`)

**20 طلبًا فقط**، بهذا الترتيب الدقيق:

```
1. GET  plugin.handtalk.me/web/12.8.1/iframe.html          (0.6 KB)   ← إطار التحليلات
2. GET  plugin.handtalk.me/web/12.8.1/iframe-app.js        (176.8 KB)
3. GET  plugin.handtalk.me/corejs/2.4.0/core.min.js        (809.1 KB) ← محرك 3D
4. POST translation-v3.handtalk.me/api/v2/sign/auth        (319 ms)   ← بدء الجلسة
5-11. GET web/12.8.1/animations/en-ase/*.json              (~97 KB)   ← رسائل النظام
12.  GET plugin.handtalk.me/corejs/2.4.0/HUGO/HUGO.gltf    (3.57 MB)  ← الأفاتار
13.  GET plugin.handtalk.me/custom/CAMISA_LONGA_...png     (13.5 KB)  ← تخصيص الزيّ
```

#### 3.ب.1 — حقن محرك 3D

الكود المفكوك من `handtalk.min.js`:

```js
const loadCore = (coreVersion) => new Promise(resolve => {
  const existing = document.getElementById(idHTCoreScript);
  const done = () => resolve(new HTCore(coreConfig ?? {}));
  if (existing) { if (alreadyLoaded) return done(); existing.remove(); }
  const script = document.createElement('script');
  script.src = `https://plugin.handtalk.me/corejs/${coreVersion}/core.min.js`;
  script.id  = idHTCoreScript;
  document.body.appendChild(script);
  script.onload = done;
});
```

المحرك يُسجّل نفسه كـ `window.HTCore` عالميًا. القشرة تنتظر `onload` ثم تُنشئ نسخة.

#### 3.ب.2 — المصادقة

```http
POST https://translation-v3.handtalk.me/api/v2/sign/auth
Content-Type: application/json; charset=utf-8
key: 2b28e79b5d5ee2c3d6b1a531c2d1d316
X-Integrity: ccb91924028db2a5c20fb94a1a00571fff216eb031101a670c9be24fa2b43169
Origin: https://www.handtalk.me
```

```json
{
  "avatar": "HUGO",
  "lang": "ase",
  "token": "894ad581fb6306c7363cb9c32a5dd9dd",
  "doNotTrack": "false",
  "sessionId": "b46a98f1-f1f0-40b7-bbf5-75a8bf91e156"
}
```

الاستجابة:

```json
{
  "custom": { "manga_longa": "https://plugin.handtalk.me/custom/CAMISA_LONGA_HANDTALK_471faf28.png" },
  "sessionId": "b46a98f1-f1f0-40b7-bbf5-75a8bf91e156",
  "csrfToken": "TgIA8fFk-v-HYEirmrTfJjFq35bNBbStbMK0",
  "sessionExpiration": 1785340852739
}
```

> `sessionExpiration` مرصود = **12.5 دقيقة** بعد المصادقة.

#### 3.ب.3 — نموذج الأمان (مفكوك وموثّق بالتحقق العملي)

ثلاث طبقات:

1. **`key` header** — مفتاح API عام مضمّن في الكود: `2b28e79b5d5ee2c3d6b1a531c2d1d316`. يُحدّد التطبيق لا العميل.
2. **`token` في الحمولة** — معرّف العميل (المشترك).
3. **`X-Integrity` header** — توقيع HMAC-SHA256.

الخوارزمية المستخرجة حرفيًا من `handtalk.min.js`:

```js
class TranslationClientUtils {
  static INTEGRITY_KEY = "84c05a5dba7ff8c8f4a6d14cf43115d2bc15dba1302b5fc83c66c8b2f776a580";
  static generateHmac = hashWithSecret; // HMAC-SHA256 عبر WebCrypto
}

attachIntegrityHeaders = async (payload) => {
  const message = { origin: location.origin, payload };
  const secret  = TranslationClientUtils.INTEGRITY_KEY + this._token;
  this.headers["X-Integrity"] = await generateHmac(JSON.stringify(message), secret);
};
```

**تم التحقق عمليًا:** أعدنا حساب التوقيع لطلبين من الـ HAR وطابق الناتج بالضبط:

```
HMAC_SHA256(
  key = "84c05a5d...a580" + "894ad581...d9dd",
  msg = '{"origin":"https://www.handtalk.me","payload":{"q":"HAND TALK","lang":"ase","token":"894ad581...","doNotTrack":"false","sessionId":"b46a98f1-..."}}'
) = c19f5100e7a5d4104568271f46d1492462a3ce74642f5afc2def671ded916753  ✓ مطابق
```

**⚠️ تقييم أمني صريح:** `INTEGRITY_KEY` موجود في JavaScript على العميل — أي شخص يستطيع استخراجه. هذا **ليس أمانًا حقيقيًا**، بل حاجز ضد إساءة الاستخدام العابرة (casual abuse). الحماية الفعلية تأتي من:
- التحقق من `Origin` على السيرفر (السرّ يُدمج مع `location.origin`)
- تحديد المعدل: `x-ratelimit-limit: 300`
- ربط الجلسة والانتهاء بعد ~12 دقيقة

**توصيتنا:** طبّق نفس الطبقات، لكن **لا تسمِّها أمانًا**. أضف تحققًا من نطاق العميل (domain allowlist) على السيرفر مربوطًا بالـ token — هذه هي الحماية الحقيقية.

#### 3.ب.4 — إطار التحليلات المعزول

```html
<!-- plugin.handtalk.me/web/12.8.1/iframe.html -->
<!doctype html>
<html><head><meta charset="utf-8"/><title>Hand Talk - Plugin</title></head>
<body><script src="./iframe-app.js"></script></body></html>
```

يُحقن كـ `<iframe hidden width="0" height="0">` والتواصل عبر `postMessage`:

```js
iframe.contentWindow.postMessage({
  action: 'init',
  initData: { page, title, token, version, language, offRedirection, profileType, hasActivatedBefore }
});
window.onmessage = (e) => { if (e.data.action === 'ready') { flushQueue(); } };
```

> **لماذا هذا ذكي:** Google Analytics الخاص بالإضافة (`G-H8MH82K9NF`) يعمل في سياق منفصل — لا يلوّث `dataLayer` الخاص بالموقع المضيف، ولا يتعارض مع GTM لديه، ويتوافق مع سياسات الخصوصية بشكل أنظف.

---

### المرحلة ج — عند الترجمة (`When translating.har`)

```http
POST https://translation-v3.handtalk.me/api/v2/sign/translate
key: 2b28e79b5d5ee2c3d6b1a531c2d1d316
X-Integrity: c19f5100e7a5d4104568271f46d1492462a3ce74642f5afc2def671ded916753
Content-Type: application/json; charset=utf-8
```

```json
{
  "q": "HAND TALK",
  "lang": "ase",
  "token": "894ad581fb6306c7363cb9c32a5dd9dd",
  "doNotTrack": "false",
  "sessionId": "b46a98f1-f1f0-40b7-bbf5-75a8bf91e156"
}
```

ترويسات الاستجابة المهمة:

```
x-ratelimit-limit: 300
x-ratelimit-remaining: 289
x-ratelimit-reset: 1785340148
access-control-allow-origin: *
cf-cache-status: DYNAMIC          ← لا تخزين مؤقت (خلف Cloudflare)
content-encoding: gzip
```

بنية الاستجابة موثّقة بالكامل في [`02-REVERSE-ENGINEERING.md`](02-REVERSE-ENGINEERING.md).

---

## 4. آلية التقاط النص (الأهم في تجربة المستخدم)

مستخرجة من `textSelectionCollectorService` و `getSelectionData`.

### وضعان للعمل

| الوضع | المُشغّل | الاستخدام |
|---|---|---|
| `click` (افتراضي) | نقرة على أي عنصر يحوي نصًا | الأسرع |
| `selection` | تحديد نص بالماوس/اللمس | لفقرات محددة |

### الأحداث المرصودة

```js
const listeners = [
  ['mouseup',         e => schedule(e)],
  ['touchend',        e => schedule(e)],
  ['keyup',           e => { if (e.shiftKey || e.ctrlKey || e.metaKey || e.key === 'Escape') schedule(); }],
  ['selectionchange', () => hasSelection() ? schedule() : clear()],
];
// debounce 10ms عبر setTimeout قبل قراءة التحديد
```

`hasSelection()` تفحص أيضًا **داخل كل `<iframe>`** في الصفحة:

```js
const iframes = document.querySelectorAll('iframe');
for (const f of iframes) {
  try {
    const sel = f.contentWindow?.getSelection();
    if (sel && !sel.isCollapsed && sel.rangeCount > 0) return true;
  } catch (e) { /* cross-origin — تجاهل */ }
}
```

### استخراج النص من عنصر

```js
const extractElementText = (el) => {
  let text = '';
  switch (el.tagName.toLowerCase()) {
    case 'select': text = el.options?.[el.selectedIndex]?.innerText || ''; break;
    case 'img':    text = el.getAttribute('data-ht-alt') || el.alt || ''; break;
    case 'button': text = el.innerText || el.textContent || el.value || '';
    default:       text = el.innerText || el.textContent || '';
  }
  // احتياطي: سمات ARIA
  if (!text) text = firstAttr(el, ['aria-label','aria-describedby','aria-description']);
  return normalize(text);
};
```

> **`data-ht-alt`:** سمة مخصّصة تسمح لمالك الموقع بكتابة وصف بديل مُحسَّن للإشارة، مختلف عن `alt` العادي. ميزة صغيرة وذكية جدًا — انسخها.

### التطبيع (normalize)

```js
text.replace(/&nbsp;/g, ' ')
    .replace(/ +/g, ' ')
    .replace(/[\t]+/gm, '')
    .replace(/[ ]+$/gm, '').replace(/^[ ]+/gm, '')
    .replace(/\n+/g, '\n').replace(/\n+$/, '').replace(/^\n+/, '')
    .trim();
```

### الاستثناءات الافتراضية

```js
exceptions: ['.ht-translator-exception', '.ht-prompt-link-exception', 'input', 'textarea']
clickables: ['a', 'button', '[role=button]', '[role=link]']
```

عند النقر على عنصر **غير clickable وغير exception**، الإضافة تستدعي `e.preventDefault()` و `e.stopPropagation()` — تمنع سلوك الصفحة الأصلي. أما الروابط والأزرار فتُترك تعمل بشكل طبيعي (يظهر بدلًا منها prompt يسأل المستخدم).

### التغذية الراجعة البصرية

مؤشرات فأرة مخصصة كـ `data:image/svg+xml;base64` مع `text-decoration: underline`:

| الحالة | الصنف (class) |
|---|---|
| نص صالح للترجمة | `.ht-signlanguage-hover` |
| نص غير صالح (طويل جدًا/فارغ) | `.ht-invalid-text` (تسطير أحمر) |
| قارئ الصفحة | `.ht-speech-hover` |
| جارٍ التحميل | `.ht-loading-hover` |

**حدود النص:** الافتراضي **800 حرف**، الحد الأقصى المسموح **1000 حرف** (`maxTextLimit`). النص الأطول يُرفض ويُعرض أنيميشن `character_limit.json` الجاهز.

---

## 5. الإعداد الافتراضي الكامل (مستخرج حرفيًا)

```js
const defaultConfig = {
  align: 'default',
  alpha: true,                 // خلفية شفافة في WebGL
  antialias: true,
  coreVersion: '2.4.0',
  storageUrl: 'https://plugin.handtalk.me/web/12.8.1/',
  authMethod: 'default',
  avatar: 'HUGO',              // أو 'MAYA'
  doNotTrack: false,
  exceptions: ['.ht-translator-exception', '.ht-prompt-link-exception', 'input', 'textarea'],
  experimental: false,         // يفعّل التخزين المؤقت في sessionStorage
  fpsLimit: 0,                 // 0 = بدون تحديد
  height: 400,
  width: 250,
  highContrast: false,
  maxTextSize: 800,
  mobileConfig: {},
  mobileEnabled: true,
  remoteConfigEnabled: true,
  addonsEnabled: true,
  opacity: 50,
  parentElement: document.body,
  side: 'right',
  textEnabled: true,
  token: '',
  zIndex: 1000000,
  clickables: ['a', 'button', '[role=button]', '[role=link]'],
  language: 'ptBR-bzs',
  languageInterface: document.documentElement.lang || 'pt-BR',
  colorButton: 'a11yColorMain',
  offRedirection: false,
  apiEndpoint: 'https://translation-v3.handtalk.me',
  customStyle: { borderRadius: 16 },
  addonsMap: { /* انظر الإعداد عن بُعد أعلاه */ }
};

// ثوابت أخرى
const speeds       = { veryFast: 2, fast: 1.7, normal: 1, slow: 0.5, paused: 0 };
const opacities    = [0, 50, 100];
const maxTextLimit = 1000;
const portraitWidthBreakpoint  = 1024;
const landscapeWidthBreakpoint = 1366;
const maxWidth = 250, maxHeight = 400;
const ga4TrackCode = 'G-H8MH82K9NF';
```

---

## 6. حزمة التقنيات المرصودة

| الطبقة | التقنية | الدليل |
|---|---|---|
| أداة البناء | **Parcel 2** | `@parcel/transformer-js/src/esmodule-helpers.js` في كل وحدة |
| المُترجم | **SWC** | `@swc/helpers/_/_class_private_field_get` وغيرها |
| واجهة المستخدم | **Preact** | `preact`, `preact/hooks`, `react/jsx-runtime` |
| إدارة الحالة | **zustand** | `zustand/react/shallow`, `createStore` |
| تنقية HTML | **DOMPurify 3.2.5** | `o.version = "3.2.5"` في الحزمة |
| محرك 3D | **three.js r117** | `const vh = "117"` + `WebGLRenderer`, `GLTFLoader`, `SkinnedMesh` |
| صيغة الموديل | **glTF 2.0** (`.gltf` غير مضغوط) | `HUGO.gltf` بـ `mimeType: model/gltf+json` |
| التشفير | **WebCrypto HMAC-SHA256** | وحدة `crypto/hmac-sha256` |
| CDN + حماية | **Cloudflare** | `server: cloudflare`, `cf-ray`, `cf-cache-status` |
| التحليلات | **GA4** داخل iframe | `G-H8MH82K9NF` |

---

## 7. أنماط معمارية جديرة بالنسخ

| النمط | الوصف | لماذا يهم |
|---|---|---|
| **تحميل كسول ذو مرحلتين** | 546 KB دائمًا، 809 KB + 3.5 MB عند التفعيل فقط | لا يعاقب زوار الموقع الذين لا يستخدمون الميزة |
| **الإعداد عن بُعد مفتاحه الـ token** | ملف JSON ثابت على CDN | تغيير الإعدادات دون نشر جديد |
| **تحليلات معزولة في iframe** | GA4 في سياق مستقل | لا تلوّث الموقع المضيف |
| **Preact بدل React** | ~4 KB بدل ~45 KB | حجم الحزمة حرج في السكربتات الطرفية |
| **`.ht-skip` sandbox** | كل CSS الإضافة مُنطّق داخل صنف واحد | لا تسريب أنماط للموقع المضيف |
| **قاموس محلي احتياطي** | 43 مدخلًا لـ ASL / 47 للبرتغالية (a–z, 0–9, رموز) مضمّنة في `core.min.js` | التهجئة تعمل حتى لو فشل السيرفر |
| **رسائل نظام مسبقة الترميز** | `noConnection.json`, `invalid_text.json`, `character_limit.json` | الأفاتار يشرح الأخطاء بلغة الإشارة نفسها |

---

## 8. الخطوة التالية

[`02-REVERSE-ENGINEERING.md`](02-REVERSE-ENGINEERING.md) — فكّ بروتوكول الأنيميشن `HTA` بالكامل.
