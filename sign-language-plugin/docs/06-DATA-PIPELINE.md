# 06 — خط أنابيب البيانات: من الممثل إلى المتصفح

هذا المستند يغطي **الجزء الذي لا يظهر في HAR** — كيف تُصنع ملفات HTA أصلًا.

---

## 1. النظرة الشاملة

```
 ممثل صمّ يؤدي الإشارة
        │
        ▼
 [التقاط الحركة]         بدلة جسم + قفازات + كاميرا وجه
        │
        ▼
 [تنظيف في Blender]      تصحيح الانزلاق، تقليل المفاتيح، ضبط التوقيت
        │
        ▼
 [Retargeting]           نقل الحركة إلى rig الأفاتار الموحّد (119 عظمة)
        │
        ▼
 [تصدير HTA]             سكربت Python داخل Blender
        │
        ▼
 [مراجعة لغوية]          مترجم صمّ يوافق/يرفض
        │
        ▼
 [قاعدة البيانات + CDN]  sign_assets
        │
        ▼
 [المتصفح]               فك ترميز → AnimationClip → عرض
```

---

## 2. مواصفة الـ Rig

هذه هي أهم قرارات المشروع. **بمجرد تثبيت الـ rig، تغييره يعني إعادة إنتاج القاموس بالكامل.**

### 2.1 بنية العظام (على نمط Rigify)

التوزيع أدناه **محسوب فعليًا** من جدول الأكواد في `core.min.js`:

| المجموعة | العدد | النسبة | ملاحظات |
|---|---|---|---|
| اليدان والأصابع (× 2) | **74** | **62%** | **معظم الميزانية هنا — غير قابل للتقليص** |
| الساقان والقدمان | 20 | 17% | حركة قليلة لكن ضرورية للتوازن البصري |
| الكتفان والذراعان (× 2) | 16 | 13% | |
| الجذع والرقبة والرأس | 9 | 8% | أساسي للانحناءات والإيماءات النحوية |
| **الإجمالي** | **119** | 100% | مطابق لمعيار Hand Talk |

> **ملاحظة:** لا توجد عظام مخصّصة للعينين في rig الـ 119. اتجاه النظر يُعالَج ضمن الـ 60 morph target.

**تفصيل اليد الواحدة (~40 عظمة):**

```
handL
├── thumb01L01 → thumb01L02 → thumb02L → thumb03L
├── finger_index01L01 → index01L02 → index02L → index03L
├── finger_middle01L01 → middle01L02 → middle02L → middle03L
├── finger_ring01L01 → ring01L02 → ring02L → ring03L
└── finger_pinky01L01 → pinky01L02 → pinky02L → pinky03L
```

> **دقة الأصابع غير قابلة للتفاوض.** شكل اليد (handshape) هو أحد المعاملات الخمسة الأساسية لأي إشارة. rig مبسّط لليد = نظام غير قابل للاستخدام.

### 2.2 اصطلاح التسمية

اتبع نمط Hand Talk: `DEF-` للعظام المُشوّهة، `ORG-` للأصلية.

```
DEF-hips, ORG-hips
DEF-upper_armL01, DEF-upper_armL02, ORG-upper_armL
DEF-finger_index01L01, DEF-finger_index01L02, ORG-finger_index01L
```

### 2.3 الـ Morph Targets (60 هدفًا)

التوزيع الفعلي في rig الـ Hand Talk (الأسماء بالبرتغالية في المصدر):

| المنطقة | الاسم في المصدر | العدد | أمثلة |
|---|---|---|---|
| الحاجبان | `SOBRANCELHA` | **12** | داخلي/وسط/خارجي × يرتفع/ينخفض × يمين/يسار |
| العينان | `OLHO` | **12** | رمش، اتساع، تضييق |
| الشفتان | `LABIO` | 10 | بروز، ضمّ، رفع، خفض |
| الأذنان | `ORELHA` | 6 | (تجميلي — لا وظيفة نحوية) |
| اللسان | `LINGUA` | 5 | خروج، رفع — مهم في بعض الإشارات |
| الأنف | `NARIZ` | 4 | اتساع، ارتفاع، فتح المنخرين |
| الذقن/الفكّ | `QUEIXO` | 4 | فتح، إزاحة جانبية |
| الفم | `BOCA` | 2 | فتح/إغلاق |
| الخدّان | `BOCHECHA` | 2 | نفخ، شفط |
| ازدراء | `SNEER` | 2 | |
| زمّ الشفاه | `BICO` | 1 | |
| **الإجمالي** | | **60** | |

> **24 من الـ 60 (40%) مخصّصة للحاجبين والعينين وحدهما** — لأنهما يحملان أثقل حمولة نحوية في لغة الإشارة.

### 2.4 الوظيفة النحوية لتعابير الوجه (ASL)

| التعبير | الوظيفة النحوية |
|---|---|
| رفع الحاجبين | سؤال نعم/لا، جملة شرطية، تحديد الموضوع (topic) |
| خفض الحاجبين | سؤال wh- (من، ماذا، أين) |
| هزّ الرأس | النفي |
| إيماء الرأس | التأكيد |
| `MM` (شفتان مضمومتان) | "بشكل طبيعي/مريح" |
| `CHA` (فم مفتوح) | "ضخم جدًا" |
| `OO` (شفتان مستديرتان) | "صغير/رفيع" |

> **هذه ليست زينة.** جملة بدون رفع الحاجبين ليست سؤالًا — هي جملة خبرية. حذف التعابير يغيّر المعنى.

---

## 3. إعداد الالتقاط

### 3.1 العتاد

| العنصر | الخيار الموصى به | التكلفة التقريبية | البديل الأرخص |
|---|---|---|---|
| بدلة الجسم | Rokoko Smartsuit Pro II | ~$2,500 | Kinect + Nuitrack |
| قفازات اليد | Manus Quantum Metagloves | ~$5,000/زوج | Rokoko Smartgloves (~$2,500) |
| التقاط الوجه | iPhone 12+ مع ARKit | ~$600 | webcam + MediaPipe |
| مرجع بصري | كاميرتان 1080p60 | ~$400 | — |
| إضاءة | إضاءة softbox متساوية | ~$300 | — |
| الخلفية | شاشة خضراء | ~$100 | — |

**الإجمالي:** ~$9,000 لإعداد جيد؛ ~$4,000 لإعداد اقتصادي.

> **قفازات اليد هي أهم استثمار.** بدلة الجسم بدون قفازات دقيقة تنتج إشارات غير مقروءة.

### 3.2 بروتوكول التسجيل

1. **معايرة** (T-pose) في بداية كل جلسة.
2. **وضع الراحة** قبل وبعد كل إشارة — ضروري لنقاط الانتقال النظيفة.
3. **ثلاث محاولات** لكل إشارة؛ يختار المترجم الأفضل.
4. **تسمية موحّدة:** `{lang}_{gloss}_{take}.fbx` مثال: `ase_HOUSE_02.fbx`
5. **تسجيل بيانات وصفية:** المؤدي، التاريخ، التنويع الإقليمي، الملاحظات.

---

## 4. المعالجة في Blender

### 4.1 خطوات التنظيف

```python
# سكربت مبدئي داخل Blender
import bpy

def clean_action(action, tolerance=0.01):
    """تقليل المفاتيح مع الحفاظ على شكل المنحنى"""
    for fcurve in action.fcurves:
        fcurve.convert_to_samples(action.frame_range[0], action.frame_range[1])
        fcurve.convert_to_keyframes(action.frame_range[0], action.frame_range[1])
        # تنعيم الضجيج
        bpy.ops.graph.smooth()
    return action

def snap_to_rest(action, armature, frame):
    """تثبيت وضع الراحة في بداية ونهاية الإشارة"""
    for bone in armature.pose.bones:
        bone.rotation_quaternion = (1, 0, 0, 0)
        bone.keyframe_insert('rotation_quaternion', frame=frame)
```

### 4.2 مُصدِّر HTA

```python
import bpy, math

FPS = 24
ANGLE_DIVISOR = 2.7

# جدول الأكواد: الاسم الطويل → كود من حرفين
CODE_TABLE = {
    'DEF-hips': 'BA', 'ORG-hips': 'BB', 'DEF-spine': 'BC',
    # ... 119 عظمة
    'BROW.INNER.UP.R': 'UA', 'EYE.BLINK.R': 'UM',
    # ... 60 morph target
}

def quat_to_euler_deg(q):
    e = q.to_euler('XYZ')
    return (math.degrees(e.x), math.degrees(e.y), math.degrees(e.z))

def encode_channel(values):
    """[0, 45, -30] → '#45#-30'  (الصفر يُكتب فارغًا)"""
    return '#'.join('' if abs(v) < 0.5 else str(int(round(v))) for v in values)

def export_hta(armature, action, mesh):
    tracks = []

    # مسارات العظام
    for bone in armature.pose.bones:
        code = CODE_TABLE.get(bone.name)
        if not code:
            continue
        frames, xs, ys, zs = [], [], [], []
        for f in keyframes_of(action, bone):
            bpy.context.scene.frame_set(f)
            x, y, z = quat_to_euler_deg(bone.rotation_quaternion)
            frames.append(int(f))
            xs.append(x / ANGLE_DIVISOR)
            ys.append(y / ANGLE_DIVISOR)
            zs.append(z / ANGLE_DIVISOR)
        if not frames or is_static(xs, ys, zs):
            continue                                   # حذف المسارات الساكنة
        tracks.append(f"{code}*{'#'.join(map(str, frames))}*"
                      f"{encode_channel(xs)}*{encode_channel(ys)}*{encode_channel(zs)}")

    # مسارات الـ morph targets
    for key in mesh.data.shape_keys.key_blocks:
        code = CODE_TABLE.get(key.name)
        if not code:
            continue
        frames, vals = [], []
        for f in keyframes_of_shapekey(action, key):
            bpy.context.scene.frame_set(f)
            frames.append(int(f))
            vals.append(key.value * 100)               # 0..1 → 0..100
        if not frames or all(v < 0.5 for v in vals):
            continue
        tracks.append(f"{code}*{'#'.join(map(str, frames))}*{encode_channel(vals)}")

    return '?'.join(tracks)
```

### 4.3 التحقق من الصحة

قبل النشر، كل ملف HTA يجب أن يجتاز:

```python
def validate_hta(hta: str) -> list[str]:
    errors = []
    for track in hta.split('?'):
        if not track: continue
        parts = track.split('*')
        code = parts[0]
        if code not in CODE_TABLE.values():
            errors.append(f'كود غير معروف: {code}')
        frames = parts[1].split('#')
        is_morph = code[0] in 'UVWXYZ'
        expected = 2 if is_morph else 4        # code+frames+1  |  code+frames+3
        if len(parts) != expected:
            errors.append(f'{code}: عدد حقول خاطئ {len(parts)} (متوقع {expected})')
        for ch in parts[2:]:
            if len(ch.split('#')) != len(frames):
                errors.append(f'{code}: طول القناة لا يطابق عدد الإطارات')
    return errors
```

**فحوص إضافية إلزامية:**

- [ ] المدة بين **0.4s** و **4.0s** (خارج ذلك = خطأ في التسجيل غالبًا)
- [ ] الإطار الأول والأخير قريبان من وضع الراحة
- [ ] لا قيم دوران تتجاوز الحدود التشريحية للمفصل
- [ ] الحجم < 8 KB لإشارة مفردة

---

## 5. سير عمل المراجعة

### حالات الأصل

```
draft ──► pending_review ──► approved ──► published
             │                              │
             └──► rejected ──► (إعادة تسجيل)  └──► deprecated
```

### قائمة فحص المراجع (مترجم صمّ)

| # | البند |
|---|---|
| 1 | شكل اليد صحيح ومقروء |
| 2 | موقع الإشارة صحيح بالنسبة للجسم |
| 3 | الحركة والاتجاه صحيحان |
| 4 | تعابير الوجه موجودة وصحيحة |
| 5 | التوقيت طبيعي (لا سريع/بطيء بشكل غير طبيعي) |
| 6 | لا التباس مع إشارة أخرى |
| 7 | تعبير محايد إقليميًا (أو موسوم بالمنطقة) |

**قاعدة صارمة:** لا نشر بدون موافقة مراجعَين مستقلَين.

---

## 6. تنظيم البيانات

```
data/
├── raw/                          # التقاط خام (لا يُرفع لـ git)
│   └── ase/
│       └── HOUSE/
│           ├── take_01.fbx
│           ├── take_02.fbx
│           └── metadata.json
├── processed/                    # ملفات .blend بعد التنظيف
│   └── ase/HOUSE.blend
├── assets/                       # مخرجات HTA
│   └── ase/
│       ├── HOUSE.hta
│       └── HOUSE.meta.json
├── dictionary/
│   └── ase.json                  # gloss → sign_id + lemmas
└── rig/
    ├── base_rig.blend            # مصدر الحقيقة الوحيد
    ├── bone_codes.json           # جدول الأكواد الـ 119
    └── morph_codes.json          # جدول الـ 60
```

### `metadata.json` لكل إشارة

```json
{
  "gloss": "HOUSE",
  "lang": "ase",
  "lemmas": ["house", "home", "residence"],
  "pos": "noun",
  "region": "US-general",
  "performer_id": "p_004",
  "captured_at": "2026-03-14",
  "take_selected": 2,
  "duration_ms": 1180,
  "rig_version": "rig-v1",
  "reviewers": ["r_001", "r_007"],
  "status": "published",
  "notes": "شكل يد مسطّح، يبدأ عند مستوى الصدر"
}
```

---

## 7. مسار بديل: الاشتقاق من الفيديو (Video-to-Pose)

إذا كان الالتقاط بالعتاد مكلفًا جدًا، هناك مسار أرخص وأقل دقة:

```
فيديو لإشارة  →  MediaPipe Holistic / OpenPose  →  نقاط ثلاثية الأبعاد
              →  IK لتحويلها إلى دوران عظام     →  تنظيف يدوي في Blender
```

| البعد | التقاط بالعتاد | الاشتقاق من الفيديو |
|---|---|---|
| دقة الأصابع | ممتازة | **ضعيفة** ← المشكلة القاتلة |
| دقة الوجه | ممتازة | جيدة (MediaPipe FaceMesh) |
| التكلفة | ~$9,000 مقدمًا | ~$0 |
| الوقت لكل إشارة | ~20 دقيقة | ~60 دقيقة (تنظيف يدوي كثيف) |
| قابلية التوسّع | خطية | قابلة للأتمتة جزئيًا |

**الحكم:** استخدم الاشتقاق من الفيديو للنماذج الأولية فقط. للإنتاج، دقة الأصابع تفرض عتاد الالتقاط.

---

## 8. الخطوة التالية

عد إلى [`05-IMPLEMENTATION-ROADMAP.md`](05-IMPLEMENTATION-ROADMAP.md) للجدول الزمني، أو [`03-API-SPEC.md`](03-API-SPEC.md) لمواصفة الخلفية.
