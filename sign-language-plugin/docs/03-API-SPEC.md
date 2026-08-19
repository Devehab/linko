# 03 — مواصفة الـ API

مواصفة مقترحة لنظامك، مبنية على البنية المرصودة مع تحسينات صريحة على نقاط ضعف Hand Talk.

---

## 1. القواعد العامة

| البند | القيمة |
|---|---|
| النقطة الأساسية | `https://api.<your-domain>/v1` |
| الترميز | `application/json; charset=utf-8` |
| الضغط | `gzip` / `br` إلزامي |
| CORS | `Access-Control-Allow-Origin` **مُقيّد بنطاقات العميل**، لا `*` |
| تحديد المعدّل | لكل token + IP، مع ترويسات `X-RateLimit-*` |

### الترويسات المطلوبة

| الترويسة | الوصف |
|---|---|
| `X-Api-Key` | مفتاح التطبيق العام |
| `X-Integrity` | `HMAC-SHA256(INTEGRITY_KEY + token, JSON({origin, payload}))` |
| `X-Session-Id` | UUID v4 للجلسة (بعد `/auth`) |
| `Origin` | يُتحقق منه على السيرفر مقابل قائمة نطاقات الـ token |

---

## 2. `POST /v1/sign/auth`

بدء جلسة وجلب بيانات التخصيص.

### الطلب

```json
{
  "avatar": "ALEX",
  "lang": "ase",
  "token": "<client-token>",
  "doNotTrack": "false",
  "sessionId": "<uuid-v4 | null>"
}
```

### الاستجابة `200`

```json
{
  "sessionId": "b46a98f1-f1f0-40b7-bbf5-75a8bf91e156",
  "csrfToken": "TgIA8fFk-v-HYEirmrTfJjFq35bNBbSt",
  "sessionExpiration": 1785340852739,
  "custom": {
    "longSleeveShirt": "https://cdn.example.com/custom/shirt_a1b2.png"
  },
  "capabilities": {
    "maxTextSize": 800,
    "supportedLanguages": ["ase"],
    "avatars": ["ALEX", "SARA"]
  }
}
```

### الأخطاء

| الرمز | المعنى | إجراء العميل |
|---|---|---|
| `401` | token غير صالح | عرض رسالة إعداد خاطئ |
| `403` | Origin غير مسموح | عرض رسالة نطاق غير مُصرّح |
| `429` | تجاوز المعدّل | تراجع أسّي (exponential backoff) |

> **تحسين عن Hand Talk:** أضفنا `capabilities` — العميل يعرف حدوده من السيرفر بدل ترميزها في الكود.

---

## 3. `POST /v1/sign/translate`

النقطة الأساسية. نص → glosses + بيانات الحركة.

### الطلب

```json
{
  "q": "Hand Talk Plugin",
  "lang": "ase",
  "token": "<client-token>",
  "doNotTrack": "false",
  "sessionId": "<uuid-v4>",
  "format": "hta"
}
```

| الحقل | النوع | ملاحظات |
|---|---|---|
| `q` | string | ≤ 1000 حرف (يُرفض ما زاد بـ `400`) |
| `lang` | string | كود ISO 639-3 للغة الإشارة (`ase`, `bzs`, `bfi`) |
| `format` | enum | `hta` (افتراضي) \| `json` (للتصحيح) \| `gloss` (نص فقط) |

### الاستجابة `200` — صيغة `hta`

نفس بنية Hand Talk (متوافقة عمدًا):

```json
[
  [
    ["BI*7#11#16#21*#11#8#5*#-1#-1#*#-8#-7#-5?BJ*..."],
    ["@p", "@l", "@u", "@g", "@i", "@n"]
  ]
]
```

### الاستجابة `200` — صيغة `json` (للتصحيح والاختبار)

```json
{
  "sentences": [{
    "source": "Hand Talk Plugin",
    "glosses": [
      { "gloss": "HAND-TALK", "type": "sign",          "signId": "sgn_8f2a", "durationMs": 1250, "hta": "BI*7#11..." },
      { "gloss": "P-L-U-G-I-N", "type": "fingerspell", "letters": ["@p","@l","@u","@g","@i","@n"], "durationMs": 2400 }
    ]
  }],
  "meta": { "engine": "rule-v1", "latencyMs": 87, "cacheHit": false }
}
```

> **أضف `format=json` من اليوم الأول.** بدونها تصحيح أخطاء الترجمة كابوس.

### الأخطاء

| الرمز | الحالة | سلوك العميل الإلزامي |
|---|---|---|
| `400` | نص فارغ أو > الحد | عرض أنيميشن `character_limit` |
| `422` | لغة غير مدعومة | رسالة واضحة |
| `429` | تجاوز المعدّل | **تهجئة محلية** كبديل |
| `5xx` / انقطاع | فشل السيرفر | **تهجئة محلية** + أنيميشن `noConnection` |

---

## 4. `GET /v1/config/{token}.json`

الإعداد عن بُعد. **يُخدَّم من CDN كملف ثابت** — لا يمسّ التطبيق.

```json
{
  "config": {
    "side": "right",
    "avatar": "ALEX",
    "opacity": 100,
    "textEnabled": true,
    "addonsEnabled": true,
    "exceptions": [".no-sign", "#chat-widget"],
    "clickables": ["a", "button", "[role=button]"],
    "customStyle": {
      "button": { "primary": "#003087", "primaryFg": "#FFFFFF", "borderRadius": "16px" },
      "light":  { "neutral1": "#EDEDED", "neutralText": "#2E2E2E", "primary": "#C64F01" },
      "dark":   { "neutral1": "#121212", "neutralText": "#FFFFFF", "primary": "#FF8B10" }
    },
    "addonsMap": { "fontControl": { "fontSize": true }, "navigation": { "readerMode": true } }
  },
  "version": 3,
  "updatedAt": "2026-07-29T15:48:00Z"
}
```

**ترويسات التخزين المؤقت:**

```
Cache-Control: public, max-age=300, stale-while-revalidate=86400
ETag: "cfg-v3-a1b2c3"
```

---

## 5. `GET /v1/assets/{lang}/{signId}.hta` (اختياري — تحسين)

تقديم إشارة مفردة قابلة للتخزين المؤقت الطويل.

```
Cache-Control: public, max-age=31536000, immutable
```

> **فرصة تحسين لم تستغلها Hand Talk:** استجاباتهم `cf-cache-status: DYNAMIC` — لا تخزين مؤقت إطلاقًا. إذا فصلت الإشارات الفردية عن استجابة الترجمة، تستطيع تخزينها مؤقتًا للأبد. الأمر يتطلب طلبين لكن الثاني يأتي من الذاكرة المحلية بعد أول مرة.

**البنية المقترحة:**

```
POST /translate  →  { "glosses": ["sgn_8f2a", "@p", "@l", ...] }   ← خفيف جدًا، ديناميكي
GET  /assets/ase/sgn_8f2a.hta  →  "BI*7#11..."                     ← مُخزَّن للأبد
```

---

## 6. `POST /v1/addons/ai/simplify` (اختياري)

تبسيط لغوي بالذكاء الاصطناعي قبل الترجمة — Hand Talk لديها `synonyms-meanings` مماثلة.

```json
{ "phrase": "The aforementioned stipulations", "language": "ase", "token": "..." }
```

```json
{ "simplified": "These rules", "synonyms": ["rules", "conditions"], "confidence": 0.88 }
```

---

## 7. نموذج البيانات الخلفي

```sql
-- المستأجرون (العملاء)
CREATE TABLE tenants (
  id            UUID PRIMARY KEY,
  token         CHAR(32) UNIQUE NOT NULL,
  name          TEXT NOT NULL,
  allowed_origins TEXT[] NOT NULL,        -- ← الحماية الحقيقية
  plan          TEXT NOT NULL DEFAULT 'free',
  rate_limit    INT  NOT NULL DEFAULT 300,
  created_at    TIMESTAMPTZ DEFAULT now()
);

-- الوحدات الإشارية
CREATE TABLE glosses (
  id            UUID PRIMARY KEY,
  lang          CHAR(3) NOT NULL,          -- ISO 639-3
  gloss         TEXT NOT NULL,             -- 'HAND-TALK'
  lemmas        TEXT[] NOT NULL,           -- ['hand talk','handtalk']
  pos           TEXT,                      -- noun/verb/adj
  region        TEXT,                      -- تنويعات إقليمية
  UNIQUE (lang, gloss, region)
);
CREATE INDEX ON glosses USING GIN (lemmas);

-- أصول الأنيميشن
CREATE TABLE sign_assets (
  id            UUID PRIMARY KEY,
  gloss_id      UUID REFERENCES glosses(id) ON DELETE CASCADE,
  rig_version   TEXT NOT NULL,             -- 'rig-v1' (119 عظمة)
  hta           TEXT NOT NULL,             -- الحمولة المضغوطة
  duration_ms   INT  NOT NULL,
  fps           INT  NOT NULL DEFAULT 24,
  checksum      CHAR(64) NOT NULL,
  reviewed_by   UUID,                      -- ← مراجعة مترجم صمّ
  reviewed_at   TIMESTAMPTZ,
  UNIQUE (gloss_id, rig_version)
);

-- تخزين مؤقت للترجمات
CREATE TABLE translation_cache (
  key           CHAR(64) PRIMARY KEY,      -- sha256(lang + normalized_text)
  gloss_ids     UUID[] NOT NULL,
  hits          BIGINT DEFAULT 0,
  created_at    TIMESTAMPTZ DEFAULT now()
);
```

> **`reviewed_by` ليس حقلاً إداريًا.** لا تنشر إشارة لم يراجعها مترجم من مجتمع الصمّ. هذا شرط جودة، وشرط قبول.

---

## 8. اعتبارات الأداء

| الهدف | القيمة | كيف |
|---|---|---|
| زمن `/translate` (p95) | < 300 ms | تخزين مؤقت للجُمل + فهرس GIN |
| زمن `/auth` (p95) | < 200 ms | جلسات في Redis |
| حجم الاستجابة لجملة | < 2 KB | HTA + gzip |
| معدّل إصابة الذاكرة المؤقتة | > 70% | النصوص على المواقع متكررة جدًا |

**استراتيجية التخزين المؤقت متعدد الطبقات:**

```
المتصفح (sessionStorage)  →  CDN (للأصول الثابتة)  →  Redis (للجُمل)  →  PostgreSQL
```

---

## 9. الخطوة التالية

[`04-REQUIREMENTS.md`](04-REQUIREMENTS.md)
