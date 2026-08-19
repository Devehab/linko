# 02 — الهندسة العكسية: بروتوكول الأنيميشن `HTA`

> هذا هو المستند الأهم في الحزمة. الجزء الذي يميّز Hand Talk تقنيًا هو **صيغة الضغط النصية** التي ابتكروها لنقل الحركة. فهمها = فهم النظام كله.

---

## 1. بنية استجابة `/translate`

### 1.1 المثال الأبسط — `"HAND TALK"`

```json
[[["BI*7#11#16#21*#11#8#5*#-1#-1#*#-8#-7#-5?BJ*7#11#16#21*...?WC*7#11*#15"]]]
```

ثلاثة مستويات من التداخل:

```
[                                    ← المستوى 1: الجُمل (sentences)
  [                                  ← المستوى 2: الوحدات الإشارية (glosses)
    [ "BI*7#11..." ]                 ← المستوى 3: أكواد الأنيميشن لهذا الـ gloss
  ]
]
```

### 1.2 مثال يُظهر التهجئة — `"Hand Talk Plugin"`

```json
[[
  [ "BI*7#11#16#21*..." ],                          ← gloss 1: إشارة كاملة لـ HAND TALK
  [ "@p", "@l", "@u", "@g", "@i", "@n" ]            ← gloss 2: تهجئة PLUGIN حرفًا بحرف
]]
```

> **"Plugin" لا توجد لها إشارة في القاموس ⇒ تُهجّأ.** هذا هو الـ fallback الأساسي في النظام كله.

### 1.3 مثال فقرة كاملة (487 حرفًا)

استجابة بحجم **121 KB** في **1877 ms**:

| الجملة | عدد الـ glosses | ملاحظة |
|---|---|---|
| 0 | 5 | `[@a,@d,@v,@e,@r,@t,@i,@s,@i,@n,@g]` ثم HTA ثم HTA ثم `[@s,@t,@r,@a,@t,@e,@g,@i,@c]` ثم HTA |
| 1 | 22 | يحوي `[]` فارغة (كلمات تُحذف نحويًا) و `[@u,@n,@k]` |
| 2 | 6 | — |
| 3 | 22 | `@f,@o,@r,@b,@e,@s` (اسم علم → تهجئة) |

### ملاحظتان لغويتان حاسمتان

**1. المصفوفة الفارغة `[]` = حذف نحوي.** كلمات مثل `is`, `the`, `of`, `by` تُحذف تمامًا لأنها لا وجود لها في نحو ASL. المحرك يعالجها بشكل خاص:

```js
if (phrases.length === 1 && !phrases[0].length) push(fingerspell(originalText));
```

**2. الرمز `@unk`** يظهر عند وجود مفهوم غير معروف — إشارة "غير معروف" الصريحة، وهي سلوك لغوي صحيح.

---

## 2. تشريح صيغة `HTA` (Hand Talk Animation)

### 2.1 القواعد النحوية

```
HTA        := Track ( "?" Track )*
Track      := Code "*" Frames "*" ChanX [ "*" ChanY "*" ChanZ ]
Code       := [A-Z]{2}
Frames     := Int ( "#" Int )*
ChanX/Y/Z  := Value ( "#" Value )*        ; قيمة فارغة "" ⇒ 0
```

| الفاصل | المعنى |
|---|---|
| `?` | يفصل بين **المسارات** (كل مسار = عظمة واحدة أو morph target واحد) |
| `*` | يفصل بين **الحقول** داخل المسار |
| `#` | يفصل بين **القيم** داخل الحقل |
| `""` | قيمة فارغة = صفر (توفير حاسم في الحجم) |

### 2.2 مثال محلول

```
BT*11#16#21*-194#-194#-194*63#63#63*200#201#200
│  │         │              │        └─ الدوران حول Z لكل إطار
│  │         │              └────────── الدوران حول Y
│  │         └───────────────────────── الدوران حول X
│  └─────────────────────────────────── الإطارات 11, 16, 21
└────────────────────────────────────── الكود BT = "ORG-handL" (اليد اليسرى)
```

مسار ثانٍ من نفس الاستجابة:

```
UM*7#11#16*#100#
│  │        └─ القيم: 0, 100, 0  ⇒  بعد القسمة على 100:  0.0, 1.0, 0.0
│  └────────── الإطارات 7, 11, 16
└───────────── UM = "OLHO.PISCA.R" (رمشة العين اليمنى)
```

⇒ **رمشة عين كاملة في 16 بايت.**

---

## 3. خوارزمية فك الترميز (مستخرجة من `core.min.js`)

### 3.1 الثوابت

```js
const FRAME_RATE_INV = 0.041666666666666664;   // = 1/24  ⇒  24 fps
const ANGLE_DIVISOR  = 2.7;                    // مقياس الضغط الزاوي
const BONE_COUNT     = 119;                    // تحقّق صارم
const MORPH_COUNT    = 60;
const TRANSITION_SEC = 0.2;                    // مدة crossfade
```

### 3.2 الدوال (بالتسمية الأصلية المفكوكة)

```js
// الإطارات → أزمنة بالثواني
const parseTimes = (framesStr, offset) =>
  framesStr.split('#')
           .map(v => parseInt(v || '0', 10))
           .map(v => v * 0.041666666666666664)   // /24
           .map(v => v + offset);

// قيم الـ morph target → 0..1
const parseMorph = (str, len) => {
  const out = Array(len).fill(0);
  const parsed = str.split('#').map((v, i) => {
    const n = parseInt(v || '0', 10) / 100;
    out[i] = n;
    return n;
  });
  out[out.length - 1] = parsed[parsed.length - 1];   // تثبيت آخر قيمة
  return out;
};

const deg2rad   = d => d * Math.PI / 180;
const parseAxis = str => str.split('#').map(v => deg2rad(parseInt(v || '0', 10) / 2.7));

// Euler (راديان) → Quaternion
const eulerToQuat = (x = 0, y = 0, z = 0) => {
  const cx = Math.cos(x/2), cy = Math.cos(y/2), cz = Math.cos(z/2);
  const sx = Math.sin(x/2), sy = Math.sin(y/2), sz = Math.sin(z/2);
  return [
    sx*cy*cz + cx*sy*sz,
    cx*sy*cz - sx*cy*sz,
    cx*cy*sz + sx*sy*cz,
    cx*cy*cz - sx*sy*sz
  ];
};

// بناء مصفوفة الكواتيرنيونات (مع تكرار آخر قيمة عند نقص الطول)
const buildQuaternions = (xs, ys, zs, frameCount) => {
  const X = parseAxis(xs), Y = parseAxis(ys), Z = parseAxis(zs);
  if (X.length !== Y.length || Y.length !== Z.length) throw differentAxisSize();
  const quats = X.map((_, i) => eulerToQuat(X[i], Y[i], Z[i]));
  let last;
  const flat = Array(frameCount).fill(0)
    .map((_, i) => (last = quats[i] || last, last))
    .reduce((acc, q) => acc.concat(q), []);
  if (flat.length !== frameCount * 4) throw wrongSizeValues();
  return flat;
};
```

### 3.3 فكّ مسار واحد

```js
const decodeTrack = (raw, boneMap, morphMap, meshName, offset) => {
  const [code, frames, chX, chY, chZ] = raw.split('*');
  const longName = CODE_TABLE[code];
  if (!longName) throw new Error(`لا يوجد اسم طويل للكود ${code}`);

  const isMorph = 'UVWXYZ'.includes(code[0]);   // ← قاعدة التمييز
  const times   = parseTimes(frames, offset);

  return isMorph
    ? { name:  `${meshName}.morphTargetInfluences[${morphMap[longName]}]`,
        times, type: 'number',
        values: parseMorph(chX, times.length) }
    : { name:  `${boneMap[longName]}.quaternion`,
        times, type: 'quaternion',
        values: buildQuaternions(chX, chY, chZ, times.length) };
};
```

> **القاعدة الذهبية:** أول حرف من الكود يحدّد النوع. `B`–`F` = عظام، `U`–`Z` = morph targets.

### 3.4 بناء الـ Clip الكامل

```js
const htaToClipObj = (signIdOrHta, boneMap, morphMap, meshName, language, offset = 0.2) => {
  setLanguage(language);
  const hta = signIdOrHta.startsWith('@')
    ? OFFLINE_DICT[signIdOrHta.substr(1)]      // من القاموس المحلي
    : signIdOrHta;                             // HTA خام من السيرفر
  if (!hta) throw signIdNotFoundInAnimDictionary(signIdOrHta);

  const boneNames  = Object.keys(boneMap);
  const morphNames = Object.keys(morphMap);
  if (boneNames.length !== 119) throw wrongNumberOfBones;

  const tracks = hta.split('?').filter(Boolean)
                    .map(t => decodeTrack(t, boneMap, morphMap, meshName, offset));
  const present = tracks.map(t => t.name);

  // *** حاسم: أي عظمة غير مذكورة تُعاد إلى وضع الراحة ***
  boneNames.forEach(b => {
    const n = `${boneMap[b]}.quaternion`;
    if (!present.includes(n))
      tracks.push({ name: n, times: [0], type: 'quaternion', values: [0,0,0,1] });
  });
  morphNames.forEach(m => {
    const n = `${meshName}.morphTargetInfluences[${morphMap[m]}]`;
    if (!present.includes(n))
      tracks.push({ name: n, times: [0], type: 'number', values: [0] });
  });

  return { duration: -1, name: clipName, tracks };
};
```

**لماذا إعادة العظام غير المذكورة إلى الهوية؟** لأن الإشارة السابقة قد تركت عظامًا في وضع غير الراحة. بدون هذا التصفير، تتراكم الأوضاع وينهار الأفاتار بصريًا بعد بضع إشارات.

---

## 4. جدول أكواد العظام (مقتطف من 179 مدخلًا)

الجدول الكامل موجود في `core.min.js` تحت المتغيّر `qu`. **119 عظمة + 60 morph target.**

### 4.1 العظام

| الكود | الاسم | الكود | الاسم |
|---|---|---|---|
| `BA` | `DEF-hips` | `BT` | `ORG-handL` |
| `BB` | `ORG-hips` | `BU` | `DEF-finger_index01L01` |
| `BC` | `DEF-spine` | `BV` | `DEF-finger_middle01L01` |
| `BD` | `DEF-thighL01` | `BW` | `DEF-finger_pinky01L01` |
| `BE` | `DEF-thighR01` | `BX` | `DEF-finger_ring01L01` |
| `BF` | `ORG-spine` | `BY` | `DEF-handL` |
| `BG` | `DEF-ribs` | `BZ` | `DEF-thumb01L01` |
| `BH` | `ORG-ribs` | `CA` | `ORG-finger_index01L` |
| `BI` | `DEF-neck` | `CC` | `ORG-finger_index02L` |
| `BJ` | `ORG-neck` | `CE` | `ORG-finger_index03L` |
| `BK` | `DEF-head` | `DG`–`EV` | سلسلة الذراع/اليد اليمنى |
| `BN` | `DEF-upper_armL01` | ... | ... |

**بادئات التسمية:** `DEF-` = عظام التشوّه (deform)، `ORG-` = العظام الأصلية (original). هذا **اصطلاح Rigify في Blender** — دليل قاطع على أن الأفاتار مُهيّأ في Blender.

### 4.2 الـ Morph Targets (تعابير الوجه)

الأسماء **بالبرتغالية** (Hand Talk شركة برازيلية) — دليل على أن النظام بُني أولًا للغة الإشارة البرازيلية (Libras):

| الكود | الاسم الأصلي | الترجمة |
|---|---|---|
| `UA` | `SOBRANCELHA.DENTRO.SOBE.R` | الحاجب الأيمن — داخلي — يرتفع |
| `UB` | `SOBRANCELHA.DENTRO.SOBE.L` | الحاجب الأيسر — داخلي — يرتفع |
| `UE` | `SOBRANCELHA.MEIO.SOBE.R` | الحاجب الأيمن — وسط — يرتفع |
| `UM` | `OLHO.PISCA.R` | رمشة العين اليمنى |
| `UN` | `OLHO.PISCA.L` | رمشة العين اليسرى |
| `UO` | `OLHO.CRESCE.R` | اتساع العين اليمنى |
| `UQ` | `NARIZ.CRESCE` | اتساع الأنف |
| `UU` | `ORELHA.SOBE.R` | ارتفاع الأذن اليمنى |

> **التغطية التشريحية للوجه هي التي تجعل الترجمة مفهومة.** 60 هدفًا هو الحد الأدنى العملي — ليس رفاهية.

---

## 5. القاموس المحلي المضمّن (Offline Dictionary)

مضمّن حرفيًا داخل `core.min.js`:

```js
const LANGUAGES = {
  bzs: { workspaceId: 'HT-BZS', oralLanguageId: 'por', animations: { /* 47 مدخلًا */ } },
  ase: { workspaceId: 'HT-ASL', oralLanguageId: 'eng', animations: { /* 43 مدخلًا */ } }
};
```

**محتوى القاموس (43 مدخلًا لـ `ase`، 47 لـ `bzs`):**
- الأرقام `0`–`9`
- الحروف `a`–`z`
- الرموز: `espaco` (مسافة)، `arroba` (@)، `ced` (ç)، `exclamacao` (!)، `interrogacao` (?)، `ponto` (.)، `agu` (´)، `til` (~)، `cir` (^)
- `repouso` / `repouso2` — **وضع الراحة** (idle)

### خريطة الحروف مع تفكيك التشكيل

```js
const LETTER_MAP = {
  a:['@a'], b:['@b'], /* ... */ z:['@z'],
  ' ':['@espaco'], '@':['@arroba'], 'ç':['@ced'],
  '!':['@exclamacao'], '?':['@interrogacao'], '.':['@ponto'],
  // الحروف المُشكّلة تُفكّك إلى إشارتين متتاليتين
  'á':['@a','@agu'], 'é':['@e','@agu'], 'ã':['@a','@til'], 'ô':['@o','@cir'], ...
};

const fingerspell = (text) => {
  const out = [];
  text.toLocaleLowerCase().split('').forEach(ch => {
    if (LETTER_MAP[ch]) out.push(...LETTER_MAP[ch]);
  });
  out.push('@repouso');        // العودة لوضع الراحة دائمًا
  return out;
};
```

> **درس تصميمي:** التشكيل يُفكَّك إلى إشارتين (الحرف ثم العلامة). هذا يوفّر مدخلات قاموس كثيرة. مفيد جدًا لأي لغة ذات علامات تشكيل.

---

## 6. مسار معالجة الاستجابة الكامل

```js
treatAnim(originalText, error, phrases, callbacks, language = 'bzs') {
  const queue = [];
  const flatten = (item) => Array.isArray(item) ? item.forEach(flatten) : queue.push(item);

  if (phrases) {
    phrases.forEach(sentence => sentence.forEach(flatten));   // تسطيح المستويات الثلاثة
    if (phrases.length === 1 && !phrases[0].length)
      flatten(fingerspell(originalText));                     // لا نتيجة ⇒ هجّئ
    flatten(this.idleAnimationName);                          // '@repouso' في النهاية

    if (config.experimental)                                  // تخزين مؤقت اختياري
      sessionStorage.setItem(`HT${language + hash(originalText)}`, JSON.stringify(queue));
  } else if (error) {
    queue.push(...fingerspell(originalText));                 // فشل الشبكة ⇒ هجّئ
    callbacks?.onError?.();
  }
  return queue;
}
```

### شجرة القرار للتدهور اللطيف (graceful degradation)

```
 هل نجح الطلب؟
 ├─ نعم ──► هل الاستجابة فارغة؟
 │           ├─ نعم ──► تهجئة النص الأصلي حرفًا بحرف
 │           └─ لا  ──► شغّل الـ glosses بالترتيب
 └─ لا  ──► تهجئة النص الأصلي حرفًا بحرف (من القاموس المحلي — لا حاجة للشبكة)
                └─ + عرض أنيميشن noConnection.json
```

> **النظام يظل مفيدًا حتى بدون إنترنت.** هذا مبدأ تصميمي أساسي انسخه كما هو.

---

## 7. تشغيل الأنيميشن

```js
playNextAction(isFirst) {
  const next     = signEmitter.shiftFromQueue;
  const previous = signEmitter.currentSign;
  signEmitter.setCurrentSign(next);

  const action = next?.action;
  if (action) {
    if (!isFirst) action.crossFadeFrom(this.idleAction, 0.2, true);   // انتقال ناعم 0.2s
    action.play();
  } else {
    this.clearMixer();
    this.setIsPlaying(false);      // انتهى الطابور
  }
  next?.onSignalized?.();
}
```

**سرعات التشغيل (`AnimationMixer.timeScale`):**

```js
const speeds = { veryFast: 2, fast: 1.7, normal: 1, slow: 0.5, paused: 0 };
```

تُحفظ في `localStorage.HTSpeed`.

**إعداد الـ Renderer:**

```js
new THREE.WebGLRenderer({ alpha: true, antialias: true });
renderer.toneMappingExposure    = 0.1;
renderer.physicallyCorrectLights = true;
renderer.outputEncoding          = THREE.sRGBEncoding;

// الكاميرا
{ fov: 17, aspect: 0.625, near: 0.1, far: 1000, frameSteps: 16 }
```

**التعامل مع الشاشات عالية الكثافة:**

```js
let dpr = window.devicePixelRatio || 1;
dpr = Math.min(Math.max(dpr, 1), 1.75);   // مُقيّد بين 1 و 1.75
renderer.setSize(el.offsetWidth * dpr, el.offsetHeight * dpr);
// debounce 250ms على resize
```

---

## 8. تحليل كفاءة الضغط

| النص | حجم استجابة HTA | التقدير كـ glTF/BVH | نسبة التوفير |
|---|---|---|---|
| `"HAND TALK"` | **1.0 KB** | ~80–150 KB | **~100×** |
| `"Hand Talk Plugin"` | **1.1 KB** | ~200 KB | **~180×** |
| فقرة 487 حرفًا | **121 KB** | ~5–8 MB | **~50×** |

### مصادر الكفاءة الخمسة

1. **مفاتيح مرجعية (keyframes)** فقط — لا إطارات مُستوفاة (interpolated). مثال: `BT*11#16#21` = ثلاثة مفاتيح لمسار كامل.
2. **حذف المسارات الساكنة** — العظمة التي لا تتحرك لا تُرسل أصلًا (يعيدها المُفكِّك إلى الهوية).
3. **القيم الفارغة = صفر** — `BQ*11*215**` يعني: الإطار 11، X=215، Y=0، Z=0. حرفان بدلًا من رقمين.
4. **أعداد صحيحة مضغوطة** — الزوايا محفوظة كأعداد صحيحة مقسومة على `2.7`، لا أعداد عشرية.
5. **رموز من حرفين** — `BT` بدلًا من `"ORG-handL"` (2 بايت بدل 10).

### مقارنة صيغ نقل الحركة

| الصيغة | الحجم لجملة قصيرة | الملاءمة |
|---|---|---|
| **HTA (المُتّبع)** | **~1 KB** | ✅ مثالي للويب |
| glTF مع أنيميشن | 80–200 KB | ثقيل جدًا لكل جملة |
| BVH | 50–150 KB | نصي لكنه مُسهب |
| فيديو WebM | 500 KB – 2 MB | لا يقبل التركيب أو تغيير السرعة |
| تدفق بيانات ثنائي مخصص | ~0.4 KB | أصغر لكن أصعب في التصحيح |

> **خلاصة:** ابنِ صيغة مماثلة. HTA هو التوازن الصحيح بين الحجم وسهولة التصحيح. المتصفح يضغطه بـ gzip فوق ذلك (`content-encoding: gzip`).

---

## 9. مسائل مفتوحة (ما لم نستطع رؤيته من HAR)

| السؤال | ما نعرفه | ما لا نعرفه |
|---|---|---|
| كيف يعمل NLP على السيرفر؟ | يُنتج glosses + يحذف كلمات وظيفية + يهجّئ المجهول | القواعد/النموذج المستخدم |
| كيف تُبنى ملفات HTA أصلًا؟ | مُنتَجة من rig بـ Rigify/Blender بـ 119 عظمة | أداة التصدير |
| هل هناك co-articulation؟ | crossfade ثابت 0.2s بين الإشارات | لا يبدو أن هناك مزجًا سياقيًا متقدمًا |
| كيف يُختار الـ gloss عند تعدد المعاني؟ | — | منطق إزالة الغموض (disambiguation) |

---

## 10. الخطوة التالية

[`03-API-SPEC.md`](03-API-SPEC.md) — مواصفة الـ API لنظامك.
