# دليل linko

> أمر واحد يحوّل مشروعك المحلي إلى رابط HTTPS عام على دومينك.

بدون فتح بورت، بدون Port Forwarding، بدون شهادات SSL، بدون إنشاء سجلات DNS يدويًا.

```console
$ linko 3000

✓ DNS record created (x92ka.example.com)
✓ Route published (x92ka.example.com -> http://localhost:3000)
✓ Tunnel connected

  Public URL  https://x92ka.example.com
  Forwards    http://localhost:3000

  Press Ctrl+C to stop
```

---

## المحتويات

1. [ما تحتاجه](#ما-تحتاجه)
2. [التثبيت](#١--التثبيت)
3. [توكن Cloudflare](#٢--توكن-cloudflare) ← أهم خطوة
4. [الإعداد](#٣--الإعداد)
5. [قاعدة المستوى الواحد](#٤--قاعدة-المستوى-الواحد) ← اقرأها قبل أن تحتاجها
6. [الاستخدام والأوامر](#٥--الاستخدام)
7. [حل المشاكل](#٦--حل-المشاكل)

---

## ما تحتاجه

- حساب Cloudflare مجاني
- دومين موجّهة nameservers الخاصة به إلى Cloudflare

**لا تحتاج Go**، ولا تحتاج تثبيت `cloudflared` — يُنزَّل تلقائيًا عند أول استخدام.

---

## ١ · التثبيت

### macOS و Linux — سطر واحد

```bash
curl -fsSL https://raw.githubusercontent.com/Devehab/linko/main/install.sh | bash
```

يكتشف السكربت نظامك، ينزّل الملف التنفيذي الجاهز، ويضعه في مجلد موجود في مسارك.
للتحقق:

```bash
linko --version
```

### Windows

نزّل ملف `.zip` من [صفحة الإصدارات](https://github.com/Devehab/linko/releases)،
فك الضغط، وضع `linko.exe` في مجلد ضمن `PATH`.

### عبر Go (اختياري)

هذه الطريقة **تحتاج Go**. إن لم يكن مثبتًا:

```bash
# macOS
brew install go

# Ubuntu / Debian
sudo apt install golang-go

# أو نزّل الحزمة الرسمية من https://go.dev/dl/
```

ثم:

```bash
go install github.com/Devehab/linko@latest
```

---

## ٢ · توكن Cloudflare

هذه الخطوة التي تتعثّر فيها الأغلبية. القاعدة في سطر واحد:

> **توكن واحد، صلاحيتان معًا.**
> ليس توكنًا للـ DNS وآخر للنفق — بل توكن واحد يحمل الاثنتين. وكلتاهما **Edit** لا Read.

### الخطوات

**١.** افتح [dash.cloudflare.com/profile/api-tokens](https://dash.cloudflare.com/profile/api-tokens)
← **Create Token** ← انزل إلى أسفل الصفحة واختر **Create Custom Token**.

**٢.** في قسم **Permissions** أضف السطرين. اضغط **+ Add more** لإضافة الثاني:

| # | الأول | الثاني | الثالث |
| --- | --- | --- | --- |
| ١ | `Zone` | `DNS` | `Edit` |
| ٢ | `Account` | `Cloudflare Tunnel` | `Edit` |

**٣.** حدّد الموارد:

- **Account Resources** ← اختر حسابك
- **Zone Resources** ← `Include` → `Specific zone` → دومينك

> ⚠️ إن تركت **Zone Resources** فارغًا فسيتصل التوكن بنجاح لكنه لن يرى أي دومين،
> وستحصل على `no zone named …`.

**٤.** اضغط **Continue to summary** ← **Create Token**، وانسخ الرمز — يُعرض مرة واحدة فقط.

---

## ٣ · الإعداد

```bash
linko init
```

يسألك ثلاثة أسئلة فقط: الرمز، الدومين، اسم النفق. اضغط Enter لقبول الافتراضي.

```console
Cloudflare credentials
Cloudflare API token: (مخفي أثناء الكتابة)
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

عند سؤال **Base subdomain** اترك الإجابة على الدومين نفسه: `example.com`.
لا تكتب `demo.example.com` — السبب في القسم التالي.

### إعداد غير تفاعلي

```bash
export LINKO_API_TOKEN='...'
linko init --yes --domain example.com --base example.com
```

---

## ٤ · قاعدة المستوى الواحد

شهادة Cloudflare المجانية تُصدَر لاسمين اثنين فقط:

```
example.com        ← الجذر
*.example.com      ← مستوى واحد
```

و`*` في الشهادات **لا تعبر النقطة** — تطابق تسمية واحدة فقط:

| الرابط | يعمل؟ |
| --- | --- |
| `demo.example.com` | ✅ يطابق `*.example.com` |
| `demo.app.example.com` | ❌ مستويان — لا شهادة تغطيه |

> ⚠️ **هذا أصعب عطل في التشخيص كله.**
> الـ DNS صحيح، والنفق متصل، و`linko doctor` أخضر بالكامل — ومع ذلك يرفض المتصفح
> الاتصال بـ `ERR_SSL_VERSION_OR_CIPHER_MISMATCH`، لأن الرفض يحدث قبل وصول أي طلب
> إلى نفقك. لهذا يحذّرك `linko init` منه مسبقًا.

تريد مستويين؟ يلزمك **Advanced Certificate Manager** من Cloudflare
(‏١٠ دولارات شهريًا للدومين) ثم تفعيل **Total TLS**. البدائل المجانية
(Subdomain setup، إعداد CNAME جزئي) محصورة بخطط Enterprise وBusiness.

**الأبسط والمجاني:** اجعل التمييز في الاسم نفسه —
`myapp-dev.example.com` بدلًا من `myapp.dev.example.com`.

---

## ٥ · الاستخدام

شغّل مشروعك أولًا، ثم:

```bash
linko 3000
```

تحصل على اسم عشوائي مثل `x92ka`. الرابط يعيش ما دامت الأداة تعمل، وعند `Ctrl+C`
يُحذف الاسم وسجل الـ DNS تلقائيًا — لا تتراكم لديك سجلات.

### رابط ثابت

```bash
linko 3000 --name crm
```

النتيجة `https://crm.example.com`، ويبقى محفوظًا بعد الإيقاف.

### كل الأوامر

| الأمر | الوظيفة |
| --- | --- |
| `linko init` | الإعداد لمرة واحدة |
| `linko 3000` | رابط عشوائي مؤقت |
| `linko 3000 --name crm` | رابط باسم ثابت |
| `linko 3000 --keep` | يبقي الاسم العشوائي بعد الإيقاف |
| `linko list` | عرض روابطك |
| `linko status` | حالة النفق وعدد الاتصالات |
| `linko remove crm` | حذف رابط (المسار + سجل DNS) |
| `linko remove --all` | حذف كل الروابط |
| `linko doctor` | فحص شامل للإعداد |
| `linko docs` | هذا الدليل في الطرفية |

### صيغ المنفذ

| ما تكتبه | ما يعنيه |
| --- | --- |
| `linko 3000` | `http://localhost:3000` |
| `linko 127.0.0.1:8080` | عنوان محدد |
| `linko https://localhost:8443` | أصل يعمل بـ HTTPS |

### عدة مشاريع معًا

```bash
# نافذة طرفية لكل مشروع
linko 3000 -n web
linko 8080 -n api
```

كلاهما يمرّ عبر نفس النفق.

---

## ٦ · حل المشاكل

كل ما يلي أخطاء حقيقية ظهرت أثناء التطوير، وهذه أسبابها الفعلية.

### `no zone named "example.com" on this account`

**السبب:** التوكن ينقصه صلاحية `Zone`، أو أن **Zone Resources** فيه فارغة.
**الحل:** أضف `Zone → DNS → Edit` مع `Zone Resources → Include → Specific zone → دومينك`.

`linko` يعرض لك النطاقات التي يراها التوكن فعلًا، فتعرف فورًا أين الخلل.

### `Cloudflare refused to create the DNS record` · `code 10000`

**السبب:** صلاحية DNS مضبوطة على `Read` بدل `Edit`. العثور على الدومين يحتاج قراءة
فقط، لذلك يمر الإعداد كاملًا ثم يفشل عند أول كتابة.
**الحل:** غيّر القائمة المنسدلة من **Read** إلى **Edit**.

### `Could not create the tunnel` · `code 10000`

**السبب:** التوكن يغطي DNS فقط ولا يملك صلاحية النفق.
**الحل:** أضف `Account → Cloudflare Tunnel → Edit` على **نفس** التوكن.

### `ERR_SSL_VERSION_OR_CIPHER_MISMATCH`

**السبب:** رابطك بعمق مستويين. لا شهادة مجانية تغطيه.
**الحل:** أعد الإعداد بـ `linko init --force --base example.com`.

### `no API token: pass --token or set LINKO_API_TOKEN`

**السبب:** شغّلت `linko init --yes` بلا رمز.
**الحل:** `export LINKO_API_TOKEN='...'` قبل الأمر، أو مرّر `--token '...'`.

### `command not found: linko`

**السبب:** مجلد التثبيت ليس في مسارك.
**الحل:** أعد فتح الطرفية، أو `exec $SHELL`. المثبّت يضيف السطر تلقائيًا.

### HTTP 502 · Error 1033

**السبب:** النفق يعمل لكن لا شيء يستمع على المنفذ.
**الحل:** تأكد بـ `curl http://localhost:3000`. وإن كان تطبيقك يستمع على `127.0.0.1`
فقط، جرّب `linko 127.0.0.1:3000`.

### عند الشك

```bash
linko doctor
```

يفحص ثمانية أشياء بالترتيب — من وجود `cloudflared` إلى الاتصالات الحيّة — ويقول لك
أين توقّف بالضبط.

### بداية نظيفة تمامًا

```bash
linko remove --all --yes
rm -rf ~/.linko
linko init
```

---

## كيف يعمل

`linko` ينشئ نفقًا واحدًا في حسابك، وكل مشروع يصبح اسمًا داخله موجّهًا إلى منفذ مختلف.

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

عند تشغيل `linko 3000` تحدث ثلاثة أشياء: يُنشأ سجل `CNAME` يشير إلى نفقك، وتُضاف
قاعدة توجيه تربط الاسم بـ `localhost:3000`، ثم يُشغَّل `cloudflared` فيفتح اتصالًا
**صادرًا** نحو Cloudflare. لا شيء على جهازك ينتظر اتصالات واردة — لهذا لا تحتاج
لمسّ الراوتر أو جدار الحماية.

---

## أمان

- **الرابط عام.** أي شخص يملكه يصل إلى مشروعك. لا تنشر بيانات حساسة. للحماية أضف
  Cloudflare Access من لوحة Zero Trust.
- **الرمز محفوظ في `~/.linko/config.json`** بصلاحيات `0600` — لا ترفعه إلى Git.
  للفرق وCI استخدم `LINKO_API_TOKEN`.
- **لا يحذف سجلًا ليس له.** إن كان الاسم يشير إلى موقعك الحقيقي فلن يلمسه `linko`.

---

[github.com/Devehab/linko](https://github.com/Devehab/linko) · رخصة MIT
