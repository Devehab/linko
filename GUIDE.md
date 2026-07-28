<div align="center">

# linko — الدليل الكامل

**حوّل أي منفذ محلي إلى رابط HTTPS عام — على دومينك أنت.**

[التثبيت](#التثبيت) · [البداية السريعة](#البداية-السريعة) · [الأوامر](#مرجع-الأوامر) · [وصفات عملية](#وصفات-عملية) · [حل المشاكل](#حل-المشاكل) · [English](README.md)

</div>

---

```console
$ linko 3000

✓ DNS record created (x92ka.example.com)
✓ Route published (x92ka.example.com -> http://localhost:3000)
✓ Tunnel connected

  Public URL  https://x92ka.example.com
  Forwards    http://localhost:3000
  Tunnel      example-linko-tunnel

  Press Ctrl+C to stop
```

بدون فتح بورت، بدون Port Forwarding، بدون شهادات SSL، بدون إنشاء سجلات DNS يدويًا.

## المحتويات

| | |
| --- | --- |
| [لماذا linko](#لماذا-linko) | ما يميّزه عن ngrok |
| [ما تحتاجه](#ما-تحتاجه) | المتطلبات قبل البدء |
| [التثبيت](#التثبيت) | سطر واحد، وطرق أخرى |
| [البداية السريعة](#البداية-السريعة) | التوكن ← الإعداد ← أول رابط |
| [الفكرة الأساسية](#الفكرة-الأساسية-المنفذ-يحتفظ-برابطه) | لماذا لا يتغيّر رابطك |
| [مرجع الأوامر](#مرجع-الأوامر) | كل أمر وكل خيار |
| [وصفات عملية](#وصفات-عملية) | حالات استخدام جاهزة |
| [قاعدة المستوى الواحد](#قاعدة-المستوى-الواحد) | أصعب عطل في النظام |
| [الإعدادات](#الإعدادات) | الملفات ومتغيرات البيئة |
| [كيف يعمل](#كيف-يعمل) | المعمارية |
| [حل المشاكل](#حل-المشاكل) | كل خطأ وسببه وحله |
| [الأمان](#الأمان) | ما يجب أن تعرفه |

## لماذا linko

`ngrok` ممتاز — حتى تريد دومينك أنت، بلا حدود جلسات وبلا صفحة تحذير وسيطة.
‏`linko` يعطيك ذلك فوق حسابك في Cloudflare: أمر واحد يدخل، ورابط عام يخرج.

| | |
| --- | --- |
| **دومينك أنت** | كل رابط تحت دومين تملكه. |
| **روابط ثابتة** | المنفذ يحتفظ برابطه. إعادة تشغيل مشروعك لا تغيّر الرابط الذي أرسلته. |
| **HTTPS تلقائي** | Cloudflare تصدر الشهادة وتجدّدها. |
| **ملف تنفيذي واحد** | مكتوب بـ Go. لا Node ولا Python ولا أي بيئة تشغيل. |
| **نفق واحد لكل مشاريعك** | كل مشروع اسم داخل النفق موجّه إلى منفذ مختلف. |
| **يعمل كما تشاء** | في المقدمة، أو بالخلفية، أو يبدأ مع كل إقلاع للجهاز. |
| **ينظّف نفسه** | روابط `--temp` وسجلات DNS الخاصة بها تُحذف عند الإيقاف. |

## ما تحتاجه

- حساب [Cloudflare](https://dash.cloudflare.com) مجاني
- دومين موجّهة nameservers الخاصة به إلى Cloudflare

**لا تحتاج تثبيت `cloudflared`** — يُنزَّل تلقائيًا عند أول استخدام.
**ولا تحتاج Go** إلا إن أردت البناء من المصدر.

## التثبيت

### macOS و Linux — سطر واحد

```bash
curl -fsSL https://raw.githubusercontent.com/Devehab/linko/main/install.sh | bash
```

يكتشف السكربت نظامك، ينزّل الملف المناسب، ويضعه في مجلد موجود في مسارك أصلًا.
وإن اضطر لاستخدام `~/.local/bin` فسيضيف السطر إلى ملف الشِل بنفسه.

```bash
linko --version
```

<details>
<summary><b>طرق أخرى للتثبيت</b></summary>

**عبر Go** — تحتاج Go 1.22 أو أحدث:

```bash
go install github.com/Devehab/linko@latest
```

إن لم يكن Go مثبتًا:

```bash
brew install go                 # macOS
sudo apt install golang-go      # Debian / Ubuntu
sudo dnf install golang         # Fedora
# أو نزّل الحزمة من https://go.dev/dl/
```

**من المصدر**

```bash
git clone https://github.com/Devehab/linko.git
cd linko
make deps && make verify && make build
```

**ويندوز** — نزّل ملف `.zip` من
[صفحة الإصدارات](https://github.com/Devehab/linko/releases)، فك الضغط، وضع
`linko.exe` في مجلد ضمن `PATH`.

**تحديد إصدار أو مجلد**

```bash
LINKO_VERSION=v0.2.0 LINKO_INSTALL="$HOME/bin" \
  bash -c "$(curl -fsSL https://raw.githubusercontent.com/Devehab/linko/main/install.sh)"
```

</details>

## البداية السريعة

### ١ · أنشئ توكن Cloudflare

هذه الخطوة التي تتعثّر فيها الأغلبية. القاعدة في سطر واحد:

> **توكن واحد، صلاحيتان معًا.**
> ليس توكنًا للـ DNS وآخر للنفق — بل توكن واحد يحمل الاثنتين، وكلتاهما **Edit** لا Read.

افتح [dash.cloudflare.com/profile/api-tokens](https://dash.cloudflare.com/profile/api-tokens)
← **Create Token** ← انزل إلى أسفل الصفحة واختر **Create Custom Token**.

في قسم **Permissions** أضف السطرين. اضغط **+ Add more** لإضافة الثاني:

| # | الأول | الثاني | الثالث |
| --- | --- | --- | --- |
| ١ | `Zone` | `DNS` | **Edit** |
| ٢ | `Account` | `Cloudflare Tunnel` | **Edit** |

ثم حدّد الموارد:

- **Account Resources** ← اختر حسابك
- **Zone Resources** ← `Include` → `Specific zone` → دومينك

> [!WARNING]
> ترك **Zone Resources** فارغًا ينتج توكنًا يتصل بنجاح لكنه لا يرى أي دومين،
> فيفشل `linko init` برسالة `no zone named …`. هذا أشيع خطأ في الإعداد كله.

ثم **Continue to summary** ← **Create Token**، وانسخ الرمز — يُعرض مرة واحدة فقط.

> نسيت الخطوات؟ اكتب `linko docs` وستظهر أمامك في الطرفية.

### ٢ · اربط linko بحسابك

```bash
linko init
```

يسألك ثلاثة أسئلة فقط. اضغط Enter لقبول الافتراضي:

```console
Cloudflare credentials
Cloudflare API token: ················
✓ Cloudflare connected

Domain
Domain: [example.com]
✓ DNS zone found (example.com)
Base subdomain: [example.com]
✓ URLs will look like https://abc12.example.com

Tunnel
Tunnel name: [example-linko-tunnel]
✓ Tunnel created
✓ Configuration saved to ~/.linko/config.json
✓ cloudflared installed

You're ready.
```

> [!IMPORTANT]
> عند سؤال **Base subdomain** اكتب دومينك مجرّدًا (`example.com`) — لا
> `demo.example.com`. السبب في [قاعدة المستوى الواحد](#قاعدة-المستوى-الواحد).

### ٣ · انشر أول مشروع

```bash
npm run dev          # مشروعك على المنفذ 3000
linko 3000           # في نافذة طرفية أخرى
```

## الفكرة الأساسية: المنفذ يحتفظ برابطه

هذا هو السلوك الذي يجعل استخدام `linko` مريحًا يومًا بعد يوم.

```bash
linko 3000     # أول مرة  → https://x92ka.example.com
# Ctrl+C
linko 3000     # المرة التالية → https://x92ka.example.com  (نفسه تمامًا)
```

الرابط الذي أرسلته لزميلك يبقى يعمل بعد كل إعادة تشغيل، ولا تتراكم سجلات DNS
ميتة في حسابك. وحين تريد التغيير فعلًا:

| الأمر | ماذا يحدث |
| --- | --- |
| `linko 3000 --new` | رابط عشوائي جديد، **ويحذف القديم** |
| `linko 3000 --name crm` | اسم تختاره أنت → `crm.example.com` |
| `linko 3000 --temp` | رابط لمرة واحدة، يُحذف لحظة الإيقاف |

## مرجع الأوامر

### خيارات عامة

| الخيار | الأثر |
| --- | --- |
| `--no-color` | تعطيل الألوان |
| `--version` | عرض الإصدار |
| `-h`, `--help` | مساعدة أي أمر |

### `linko init`

الإعداد لمرة واحدة: يتحقق من التوكن، يجد الـ zone، ينشئ النفق، ويحفظ كل شيء
في `~/.linko/config.json` بصلاحيات `0600`.

| الخيار | الأثر |
| --- | --- |
| `--token <t>` | توكن Cloudflare (أو استخدم `LINKO_API_TOKEN`) |
| `--domain <d>` | الدومين المُدار في Cloudflare |
| `--base <b>` | الـ subdomain الأساس للروابط |
| `--tunnel <n>` | اسم النفق (الافتراضي `<domain>-linko-tunnel`) |
| `--force` | إعادة الإعداد فوق إعداد موجود |
| `-y`, `--yes` | بلا أسئلة: يفشل بدل أن يسأل |
| `--skip-download` | لا تنزّل `cloudflared` الآن |

```bash
# إعداد كامل بلا تفاعل — مناسب لـ CI أو جهاز جديد
export LINKO_API_TOKEN='cfut_…'
linko init --yes --domain example.com --base example.com
```

### `linko <port>` · `linko start <port>`

نشر خدمة محلية. ‏`linko 3000` اختصار لـ `linko start 3000`.

| الخيار | الأثر |
| --- | --- |
| `-n`, `--name <n>` | الاسم المطلوب (الافتراضي: رابط هذا المنفذ الحالي) |
| `-r`, `--new` | رابط عشوائي جديد بدل الحالي |
| `--temp` | يُحذف الاسم عند إيقاف النفق |
| `-d`, `--detach` | يعمل بالخلفية ويرجعك للسطر |
| `-o`, `--open` | يفتح المتصفح حين يصبح الرابط جاهزًا فعلًا |
| `--replace` | استبدال الاسم إن كان يشير لمكان آخر |
| `-y`, `--yes` | بلا أسئلة |
| `-v`, `--verbose` | عرض سجلات `cloudflared` |
| `--loglevel <l>` | `debug` · `info` · `warn` · `error` · `fatal` |

**صيغ المنفذ المقبولة**

| ما تكتبه | ما يعنيه |
| --- | --- |
| `linko 3000` | `http://localhost:3000` |
| `linko :3000` | `http://localhost:3000` |
| `linko 127.0.0.1:8080` | عنوان محدد |
| `linko https://localhost:8443` | أصل يعمل بـ HTTPS |
| `linko tcp://localhost:22` | خدمة TCP خام |

### `linko list`

عرض ما نشرته.

```console
$ linko list

NAME    URL                        TARGET                  KIND
api     https://api.example.com -> http://localhost:8080   persistent
web     https://web.example.com -> http://localhost:3000   persistent

· 2 route(s) · tunnel example-linko-tunnel · ~/.linko/config.json
```

| الخيار | الأثر |
| --- | --- |
| `--remote` | يقرأ المسارات الحيّة من Cloudflare بدل الملف المحلي |

### `linko ps` و `linko stop`

إدارة الأنفاق العاملة بالخلفية.

```console
$ linko ps

NAME   URL                        TARGET                  PROCESS
web    https://web.example.com -> http://localhost:3000   pid 41288
```

```bash
linko stop web       # إيقاف واحد
linko stop --all     # إيقاف كل شيء
```

### `linko status`

حالة النفق، والاتصالات الحيّة، والمسارات المنشورة، وما يعمل بالخلفية.

```console
$ linko status

Cloudflare Tunnel
  Name        example-linko-tunnel
  ID          8f3c1a92-…
  Account     Acme
  Domain      example.com
  Status      connected (4 connections via AMS, FRA)

Routes
  · https://web.example.com -> http://localhost:3000
```

### `linko remove <name…>`

حذف رابط: قاعدة التوجيه، وسجل الـ DNS، والسطر المحلي — الثلاثة معًا.

| الخيار | الأثر |
| --- | --- |
| `--all` | حذف كل الروابط |
| `-y`, `--yes` | بلا تأكيد |

> [!NOTE]
> لا يحذف `linko` أي سجل DNS لا يشير إلى نفق Cloudflare، فلا يمكنه أن يمسّ
> سجلات موقعك الحقيقي بالخطأ.

### `linko service`

إبقاء النفق يعمل عبر إعادات التشغيل — `launchd` على macOS و`systemd --user`
على لينكس، مع إعادة تشغيل تلقائية إن توقف.

```bash
linko service install 3000 --name crm
linko service list
linko service uninstall crm
```

### `linko doctor`

ثمانية فحوص بالترتيب، من وجود `cloudflared` إلى الاتصالات الحيّة. يخرج برمز
غير صفري عند الفشل، فيصلح للاستخدام داخل السكربتات.

```console
$ linko doctor

✓ cloudflared installed (2025.2.0)
✓ config valid (~/.linko/config.json)
✓ API token valid
✓ DNS zone reachable (example.com)
✓ tunnel exists (example-linko-tunnel)
✓ tunnel configuration readable (2 routes)
✓ DNS configured for 2 routes
✓ connection active (4 edge connections)

Everything looks good.
```

### `linko docs`

الدليل في الطرفية. و`--open` يفتح الصفحة الكاملة في المتصفح.

## وصفات عملية

**تُري عميلًا عملك الحالي**

```bash
linko 3000 --name preview --open
```

**خدمتان معًا في نفق واحد**

```bash
linko 3000 -n web   # نافذة ١
linko 8080 -n api   # نافذة ٢
```

**بيئة تجريبية يجب أن تصمد بعد إعادة التشغيل**

```bash
linko service install 8080 --name staging
linko service list
```

**نقطة استقبال webhooks تعمل طوال الأسبوع**

```bash
linko 4000 -n hooks -d
linko ps
```

**رابط لمرة واحدة ينظّف نفسه**

```bash
linko 5173 --temp
```

**تطبيقك يستمع على الـ loopback فقط**

```bash
linko 127.0.0.1:3000
```

**تدوير رابط انتشر أكثر مما ينبغي**

```bash
linko 3000 --new
```

## قاعدة المستوى الواحد

شهادة Cloudflare المجانية تغطي اسمين اثنين بالضبط:

```
example.com        الجذر
*.example.com      مستوى واحد
```

و`*` في الشهادات **لا تعبر النقطة**:

| الرابط | يعمل؟ |
| --- | --- |
| `demo.example.com` | ✅ يطابق `*.example.com` |
| `demo.app.example.com` | ❌ مستويان — لا شهادة مجانية تغطيه |

> [!CAUTION]
> هذا أصعب عطل في النظام كله. الـ DNS صحيح، والنفق متصل، و`linko doctor` أخضر
> بالكامل — ومع ذلك يرفض المتصفح الاتصال بـ
> `ERR_SSL_VERSION_OR_CIPHER_MISMATCH`، لأن الرفض يحدث **قبل** وصول أي طلب
> إلى نفقك. لهذا يحذّرك `linko init` منه مسبقًا.

تريد مستويين؟ يلزمك
[Advanced Certificate Manager](https://developers.cloudflare.com/ssl/edge-certificates/advanced-certificate-manager/)
من Cloudflare (‏١٠ دولارات شهريًا للدومين) ثم تفعيل **Total TLS**. البدائل
المجانية — النطاقات الفرعية المستقلة وإعداد CNAME الجزئي — محصورة بخطط
Enterprise وBusiness.

**أبسط حل مجاني:** اجعل التمييز في الاسم نفسه — `myapp-dev.example.com` بدل
`myapp.dev.example.com`.

## الإعدادات

كل شيء في `~/.linko/`:

```
~/.linko/
├── config.json      البيانات والمسارات  (صلاحيات 0600)
├── bin/cloudflared  نسخة خاصة، تُنزَّل عند أول استخدام
├── run/<name>.pid   ملف لكل نفق يعمل بالخلفية
└── logs/<name>.log  سجل لكل نفق يعمل بالخلفية
```

### متغيرات البيئة

| المتغير | الأثر |
| --- | --- |
| `LINKO_API_TOKEN` | يتجاوز التوكن المحفوظ |
| `LINKO_HOME` | مجلد الإعدادات (الافتراضي `~/.linko`) |
| `NO_COLOR` | تعطيل الألوان |

## كيف يعمل

ينشئ `linko` نفقًا واحدًا في حسابك، وكل مشروع يصبح اسمًا داخله موجّهًا إلى
منفذ مختلف.

```
  crm.example.com   ──┐
                      │                     ┌──▶ localhost:3000
  api.example.com   ──┼──▶ Cloudflare Edge ─┼──▶ localhost:8080
                      │      (نفق واحد)      │
  test.example.com  ──┘                     └──▶ localhost:5173
                                ▲
                                │  اتصال صادر فقط
                          cloudflared على جهازك
```

عند تشغيل `linko 3000` تحدث ثلاثة أشياء:

1. يُنشأ سجل `CNAME` مُوكَّل يشير إلى `<tunnel-id>.cfargotunnel.com`
2. تُضاف قاعدة توجيه تربط الاسم بـ `http://localhost:3000`
3. يُشغَّل `cloudflared` فيفتح اتصالًا **صادرًا** نحو Cloudflare

لا شيء على جهازك ينتظر اتصالات واردة — لهذا لا تحتاج لمسّ الراوتر أو جدار
الحماية إطلاقًا.

## حل المشاكل

كل ما يلي أخطاء حقيقية ظهرت أثناء بناء الأداة واختبارها.

| الرسالة | السبب | الحل |
| --- | --- | --- |
| `no zone named "example.com"` | التوكن ينقصه صلاحية `Zone`، أو أن Zone Resources فارغة | أضف `Zone → DNS → Edit` مع تضمين دومينك. ‏`linko` يعرض لك النطاقات التي **يراها** التوكن. |
| `Cloudflare refused to create the DNS record` `code 10000` | صلاحية DNS على `Read` بدل `Edit` | العثور على الـ zone يحتاج قراءة فقط، لذلك يمر الإعداد كاملًا ثم يفشل عند أول كتابة. غيّرها إلى **Edit**. |
| `Could not create the tunnel` `code 10000` | التوكن يغطي DNS ولا يغطي الأنفاق | أضف `Account → Cloudflare Tunnel → Edit` على **نفس** التوكن. |
| `ERR_SSL_VERSION_OR_CIPHER_MISMATCH` | الرابط بعمق مستويين | `linko init --force --base example.com` أو اشترِ ACM. |
| `no API token: pass --token or set LINKO_API_TOKEN` | شغّلت `linko init --yes` بلا رمز | `export LINKO_API_TOKEN='…'` أو مرّر `--token`. |
| `command not found: linko` | مجلد التثبيت ليس في مسارك | أعد فتح الطرفية، أو `exec $SHELL`. |
| `HTTP 502` · `Error 1033` | النفق يعمل لكن لا شيء يستمع على المنفذ | تحقق بـ `curl http://localhost:3000`. وإن كان تطبيقك يستمع على الـ loopback فقط، استخدم `linko 127.0.0.1:3000`. |

عند الشك:

```bash
linko doctor
```

### بداية نظيفة تمامًا

```bash
linko remove --all --yes
rm -rf ~/.linko
linko init
```

## الأمان

- **الروابط المنشورة عامة.** أي شخص يملك الرابط يصل إلى مشروعك. لا تنشر بيانات
  حساسة، وإن احتجت مصادقة فضع
  [Cloudflare Access](https://developers.cloudflare.com/cloudflare-one/policies/access/)
  أمام الاسم.
- **التوكن في `~/.linko/config.json`** بصلاحيات `0600`. لا ترفعه إلى Git أبدًا؛
  للفرق وCI استخدم `LINKO_API_TOKEN`.
- **توكن النفق يُمرَّر إلى `cloudflared` عبر البيئة** لا عبر سطر الأوامر، فلا
  يظهر في `ps`.
- **لا يمسّ سجلات غيره.** إن كان الاسم يشير إلى موقعك الحقيقي فلن يستبدله أو
  يحذفه.

---

<div align="center">

[github.com/Devehab/linko](https://github.com/Devehab/linko) · رخصة MIT

</div>
