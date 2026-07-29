<div align="center">

<img src="docs/brand/logotype.png" alt="linko" width="252">

### الدليل الكامل

**حوّل أي منفذ محلي إلى رابط HTTPS عام — على دومينك أنت.**

[التثبيت](#التثبيت) · [البداية السريعة](#البداية-السريعة) · [الأوامر](#مرجع-الأوامر) · [وصفات عملية](#وصفات-عملية) · [حل المشاكل](#حل-المشاكل) · [English](README.md) · [الموقع](https://devehab.github.io/linko/)

</div>

---

## ما هو linko

**‏linko أداة سطر أوامر تمنح خدمة تعمل على جهازك عنوانًا عامًا بـ HTTPS على دومين
تملكه أنت.** تكتب أمرًا واحدًا، فيستطيع أي شخص في العالم فتح عملك في متصفحه.

```console
$ npm run dev          # مشروعك يعمل على المنفذ 3000
$ linko 3000           # في نافذة طرفية أخرى

✓ DNS record created (x92ka.example.com)
✓ Route published (x92ka.example.com -> http://localhost:3000)
✓ Tunnel connected

  Public URL  https://x92ka.example.com
  Forwards    http://localhost:3000
  Tunnel      example-linko-tunnel

  Press Ctrl+C to stop
```

يفعل ذلك بقيادة **Cloudflare Tunnel** نيابةً عنك. ما كنت ستفعله يدويًا — إنشاء سجل
DNS، وإعداد reverse proxy، والحصول على شهادة، وفتح منفذ في الراوتر — يقوم به linko
عبر واجهة Cloudflare في ثانيتين تقريبًا، ويتراجع عنه بنفس النظافة.

**وما ليس هو:** ليس خدمة استضافة — كودك يبقى يعمل على جهازك. وليس مزوّد أنفاق
مدفوعًا — يستخدم حسابك أنت على Cloudflare بالخطة المجانية. وليس بديلًا عن النشر
الإنتاجي — مصمّم للتطوير والعروض وwebhooks والمشاركة.

## لماذا لا ngrok

| | أدوات الأنفاق المعتادة | linko |
| --- | --- | --- |
| الدومين | دومينهم | **دومينك أنت** |
| الرابط بعد إعادة التشغيل | عشوائي جديد | **الرابط نفسه** |
| صفحة تحذير للزوار | غالبًا | **أبدًا** |
| حدود الجلسات | في الخطة المجانية | **لا يوجد** |
| يصمد بعد إعادة التشغيل | يدويًا | **`linko service install`** |
| يعمل عبر | بنيتهم التحتية | **شبكة Cloudflare، بحسابك أنت** |

## الخوادم المنزلية وكل ما لا يملك IP عامًا

**هذه هي الحالة التي بُني لها linko.** الخادم المنزلي خلف CGNAT أو عنوان متغيّر:
فتح المنافذ إمّا يمنعه مزوّد الخدمة أو ينكسر مع أول تجديد للعنوان. الـ Raspberry Pi
في الزاوية، واللابتوب القديم، والـ NAS — لا يصل إليها أحد من الخارج، مهما كان
الدومين الذي تملكه جيدًا.

لا يطلب `linko` أي منفذ وارد. يتصل `cloudflared` من جهازك **إلى الخارج**، فالعنوان
الوحيد الذي يراه العالم هو عنوان Cloudflare. عنوانك يبقى خاصًا، وراوترك لا يُمسّ.

وهذا ما يستبدله، لكل مشروع:

| يدويًا | مع linko |
| --- | --- |
| إنشاء نفق Zero Trust وربطه | `linko 8096 --name media` |
| كتابة قواعد التوجيه مع إبقاء الأخيرة في مكانها | — |
| إضافة سجل `CNAME` مُوكَّل في الـ zone الصحيحة | — |
| ضبط الشهادة ثم اكتشاف أن الاسم بمستويين غير مغطّى | — |
| كتابة ملف خدمة ليصمد بعد إعادة التشغيل | `linko service install` |

وكل مشروع آخر على ذلك الجهاز ينضم إلى النفق نفسه — اسم لكل واحد، موجّه إلى منفذ
محلي مختلف.

## المحتويات

| | |
| --- | --- |
| [ما هو linko](#ما-هو-linko) | تعريف مختصر وما يفعله بالضبط |
| [لماذا لا ngrok](#لماذا-لا-ngrok) | مقارنة مباشرة |
| [الخوادم المنزلية](#الخوادم-المنزلية-وكل-ما-لا-يملك-ip-عامًا) | بلا IP عام، وبلا فتح منافذ |
| [ما تحتاجه](#ما-تحتاجه) | المتطلبات قبل البدء |
| [التثبيت](#التثبيت) | سطر واحد، وطرق أخرى |
| [البداية السريعة](#البداية-السريعة) | التوكن ← الإعداد ← أول رابط |
| [الفكرة الأساسية](#الفكرة-الأساسية-المنفذ-يحتفظ-برابطه) | لماذا لا يتغيّر رابطك |
| [مرجع الأوامر](#مرجع-الأوامر) | كل أمر وكل خيار |
| [وصفات عملية](#وصفات-عملية) | حالات استخدام جاهزة |
| [قاعدة المستوى الواحد](#قاعدة-المستوى-الواحد) | أصعب عطل في النظام |
| [الإعدادات](#الإعدادات) | الملفات ومتغيرات البيئة |
| [كيف يعمل](#كيف-يعمل) | المعمارية |
| [التعافي الذاتي](#حين-يتغيّر-شيء-من-خلفك) | توكن ميت أو نفق محذوف |
| [حل المشاكل](#حل-المشاكل) | كل خطأ وسببه وحله |
| [الأمان](#الأمان) | ما يجب أن تعرفه |

## ما تحتاجه

| | مطلوب؟ | ملاحظات |
| --- | --- | --- |
| حساب Cloudflare | ✅ | الخطة المجانية تكفي — [أنشئ حسابًا](https://dash.cloudflare.com/sign-up) |
| دومين تملكه | ✅ | من أي مكان اشتريته: Namecheap أو GoDaddy أو غيرهما |
| **أن تكون إدارة DNS عند Cloudflare** | ✅ | اقرأ الفقرة التالية — مجاني، وتبقى مسجّلًا عند بائعك |
| `cloudflared` | ❌ | يُنزَّل تلقائيًا عند أول استخدام |
| Go | ❌ | فقط إن أردت البناء من المصدر |
| Node أو Python أو Docker | ❌ | غير مستخدمة إطلاقًا |

> [!IMPORTANT]
> **امتلاك الدومين وحده لا يكفي — يجب أن تشير nameservers الخاصة به إلى
> Cloudflare.** يُنشئ `linko` سجلات DNS عبر واجهة Cloudflare، فلا بد أن تكون
> Cloudflare هي المرجع الرسمي للـ DNS. العملية مجانية وتستغرق عشر دقائق
> معظمها انتظار.

<details>
<summary><b>كيف تنقل إدارة DNS إلى Cloudflare (مجانًا، وتبقى مسجّلًا عند بائعك)</b></summary>

أنت **لا تنقل ملكية** الدومين. يبقى مسجّلًا حيث اشتريته، وتستمر بالدفع لنفس
الجهة، ويمكنك التراجع في أي وقت. الذي يتغيّر هو **nameservers** فقط.

**١. أنشئ حسابًا مجانيًا** من
[dash.cloudflare.com/sign-up](https://dash.cloudflare.com/sign-up).

**٢. أضف دومينك.** من اللوحة اضغط **Add a site** ← اكتب `example.com` ←
اختر خطة **Free**.

**٣. دع Cloudflare تفحص سجلاتك.** ستنسخ سجلات DNS الحالية تلقائيًا. راجع
القائمة وقارنها بمزوّدك الحالي — **خصوصًا سجلات `MX`** إن كان لديك بريد على
هذا الدومين، وإلا توقّف بريدك.

**٤. انسخ الـ nameservers** اللذين تعرضهما لك، مثل:

```
dana.ns.cloudflare.com
rick.ns.cloudflare.com
```

**٥. ضعهما عند بائع الدومين.** سجّل الدخول حيث اشتريت الدومين، وابحث عن
*Nameservers* أو *DNS settings* أو *Custom DNS*، واحذف الموجود وضع اللذين
نسختهما. أدلة حسب البائع:
[Namecheap](https://www.namecheap.com/support/knowledgebase/article.aspx/767/10/how-to-change-dns-for-a-domain/) ·
[GoDaddy](https://www.godaddy.com/help/change-nameservers-for-my-domain-664) ·
[Google Domains / Squarespace](https://support.squarespace.com/hc/en-us/articles/4404183898125) ·
[Hostinger](https://support.hostinger.com/en/articles/1583227-how-to-change-nameservers-at-hostinger)

**٦. انتظر.** عادة أقل من ساعة، وقد تصل إلى ٢٤ ساعة. تصلك رسالة من Cloudflare
حين يصبح الدومين **Active**.

للتحقق بنفسك في أي وقت:

```bash
dig NS example.com +short          # يجب أن تظهر *.ns.cloudflare.com
```

وحين يظهر الدومين **Active** في اللوحة، شغّل `linko init`.

**ليس لديك دومين أصلًا؟** Cloudflare
[تبيع الدومينات بسعر التكلفة](https://developers.cloudflare.com/registrar/)
مع إدارة DNS جاهزة — فلا تحتاج لنقل أي شيء.

</details>

## التثبيت

### macOS ولينكس وويندوز عبر Git Bash — سطر واحد

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

**عبر Go** — تحتاج Go 1.23 أو أحدث:

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

**ويندوز بلا Git Bash** — نزّل ملف `.zip` من
[صفحة الإصدارات](https://github.com/Devehab/linko/releases)، فك الضغط، وضع
`linko.exe` في مجلد ضمن `PATH`. ‏PowerShell وCMD لا يشغّلان سطر `curl … | bash`.

**تحديد إصدار أو مجلد**

```bash
LINKO_VERSION=v0.3.0 LINKO_INSTALL="$HOME/bin" \
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

الصق التوكن، ثم **اختر دومينك برقم** — يعرض لك `linko` الدومينات التي يراها
توكنك فعلًا، فلا شيء تكتبه ولا شيء تخطئ في كتابته:

```console
Cloudflare credentials
Cloudflare API token: ················
✓ Cloudflare connected

Domain
Which domain should linko use?
  1. example.com
  2. another.dev
  3. staging.io  (pending)
Choose: 1
✓ URLs will look like https://abc12.example.com

Tunnel
Tunnel name: [example-linko-tunnel]
✓ Tunnel created
✓ Configuration saved to ~/.linko/config.json
✓ cloudflared installed

You're ready.
```

الدومين الذي تختاره **هو** الأساس — لا سؤال ثانٍ بعده. هذا بالضبط ما يبقي كل
رابط على عمق مستوى واحد، وهو كل ما تغطّيه شهادة Cloudflare المجانية. التفصيل
في [قاعدة المستوى الواحد](#قاعدة-المستوى-الواحد).

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
| `--domain <d>` | تخطَّ القائمة واستخدم هذا الدومين |
| `--base <b>` | انشر تحت subdomain أعمق — يحتاج شهادة مدفوعة |
| `--tunnel <n>` | اسم النفق (الافتراضي `<domain>-linko-tunnel`) |
| `--force` | إعادة الإعداد فوق إعداد موجود |
| `-y`, `--yes` | بلا أسئلة: يفشل بدل أن يسأل |
| `--skip-download` | لا تنزّل `cloudflared` الآن |

```bash
# إعداد كامل بلا تفاعل — مناسب لـ CI أو جهاز جديد
export LINKO_API_TOKEN='cfut_…'
linko init --yes --domain example.com
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

| الخيار | الأثر |
| --- | --- |
| `--fix` | يصلح توكنًا ميتًا أو نفقًا محذوفًا أو سجلات DNS ومسارات ناقصة |
| `-y`, `--yes` | يصلح بلا أسئلة |

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

### `linko domain`

يغيّر الدومين الذي ينشر عليه `linko`، من نفس القائمة المرقّمة.

```console
$ linko domain

Currently publishing to example.com

Which domain should linko publish to?
  1. example.com  (current)
  2. another.dev
Choose: 2

! You have 2 URLs on example.com:
    https://web.example.com -> http://localhost:3000
    https://api.example.com -> http://localhost:8080

Delete them before switching? [Y/n] y
✓ Removed web.example.com
✓ Removed api.example.com

✓ Now publishing to another.dev
```

| الخيار | الأثر |
| --- | --- |
| `--list` | اعرض الدومينات التي يراها التوكن ثم اخرج |
| `-y`, `--yes` | بدّل بلا أسئلة |

و`linko domain another.dev` ينتقل مباشرة. وإن كان الدومين في حساب Cloudflare
آخر فلا مشكلة — يهيّئ `linko` نفقًا هناك.

### `linko token`

يستبدل توكن الـ API المحفوظ — بعد تدويره، أو انتهائه، أو تصحيح صلاحياته.

```console
$ linko token

New Cloudflare API token: ················
✓ Token updated and saved to ~/.linko/config.json

✓ Can reach example.com
✓ Can reach the tunnel example-linko-tunnel

All good.
```

يُفحص التوكن الجديد **قبل** حفظه، فلا يمكن لخطأ مطبعي أن يقفل عليك إعدادك.
و`--token cfut_…` للاستخدام غير التفاعلي.

### `linko uninstall`

يزيل `linko` من الجهاز، وآثاره من Cloudflare، بهذا الترتيب:

1. يوقف الأنفاق العاملة بالخلفية
2. يزيل الخدمات التي تشغّلها عند تسجيل الدخول
3. يحذف روابطك المنشورة وسجلات DNS الخاصة بها
4. يحذف النفق من حساب Cloudflare
5. يحذف `~/.linko` — الإعداد والسجلات ونسخة `cloudflared`
6. يحذف ملف `linko` التنفيذي نفسه

| الخيار | الأثر |
| --- | --- |
| `-y`, `--yes` | بلا سؤال تأكيد |
| `--keep-cloud` | اترك الروابط وسجلات DNS والنفق على Cloudflare |
| `--keep-binary` | اترك الملف التنفيذي مكانه |

حسابك في Cloudflare ودومينك وتوكنك لا يُمسّون — ألغِ التوكن بنفسك إن لم تعد
تريده.

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
| `ERR_SSL_VERSION_OR_CIPHER_MISMATCH` | الرابط بعمق مستويين | `linko init --force` واختر الدومين نفسه، أو اشترِ ACM. |
| `no API token: pass --token or set LINKO_API_TOKEN` | شغّلت `linko init --yes` بلا رمز | `export LINKO_API_TOKEN='…'` أو مرّر `--token`. |
| انتهى التوكن أو دُوِّر | التوكن المحفوظ لم يعد يصادق | `linko token` — ويُفحص الجديد قبل حفظه. |
| تريد النشر على دومين آخر | إعدادك يشير إلى القديم | `linko domain` — اختر الجديد من القائمة. |
| `command not found: linko` | مجلد التثبيت ليس في مسارك | أعد فتح الطرفية، أو `exec $SHELL`. |
| `HTTP 502` · `Error 1033` | النفق يعمل لكن لا شيء يستمع على المنفذ | تحقق بـ `curl http://localhost:3000`. وإن كان تطبيقك يستمع على الـ loopback فقط، استخدم `linko 127.0.0.1:3000`. |

### حين يتغيّر شيء من خلفك

التوكنات تنتهي صلاحيتها، والأنفاق تُحذف من لوحة Zero Trust، وسجلات DNS تُمسح
يدويًا. يكتشف `linko` الثلاثة، ويشرح ما حدث بوضوح، **ثم يصلحه** — لا يكتفي
بطباعة رمز خطأ من Cloudflare.

**التوكن توقّف عن العمل**

```console
$ linko 3000

✗ Cloudflare is no longer accepting the stored API token.
· It was most likely deleted, edited, or it expired.

  Create a Cloudflare API token
  1. https://dash.cloudflare.com/profile/api-tokens
  2. Add both permission rows (+ Add more):
       Zone     →  DNS               →  Edit
       Account  →  Cloudflare Tunnel →  Edit

New Cloudflare API token: ················
✓ Token updated and saved to ~/.linko/config.json
✓ Tunnel connected
```

يتحقق من التوكن الجديد **قبل** حفظه، فلا يمكن لخطأ مطبعي أن يقفل عليك إعدادك.

**النفق حُذف**

يعيد `linko` إنشاءه (أو يتبنّى واحدًا بنفس الاسم)، ويجلب توكن نفق جديدًا،
**ويعيد توجيه كل سجلات DNS إلى النفق الجديد** — لأن النفق الجديد يعني هدف
CNAME جديدًا — ثم يستعيد كل المسارات.

**سجل DNS حُذف**

يُعاد إنشاؤه عند أول `linko <port>`، أو كلها دفعة واحدة:

```bash
linko doctor --fix
```

يصلح `--fix` التوكن الميت، والنفق المحذوف، وسجلات DNS الناقصة، والمسارات
المفقودة، ثم يعيد الفحص كاملًا. أضف `--yes` لتخطّي أسئلة التأكيد.

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

ولإزالة الأداة نفسها بالكامل — بالخلفية والخدمات والروابط والنفق والملف
التنفيذي — استخدم `linko uninstall`.

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
