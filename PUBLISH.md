# نشر linko على GitHub

المستودع مهيّأ محليًا وفيه commit جاهزان. تبقّى ثلاث خطوات تنفّذها من الطرفية.

---

## 1. تحقّق من البناء والاختبارات (مهم)

لم أتمكّن من تشغيل مترجم Go داخل بيئتي، لذا شغّل هذا أولًا:

```bash
cd ~/CodeHub/Go/linko

make deps      # go mod tidy — ينزّل Cobra وينشئ go.sum
make verify    # gofmt + go vet + كل الاختبارات
make build     # ينتج ./linko
./linko --help
```

إن ظهر أي خطأ ترجمة، أرسله لي وأصلحه فورًا.

بعد نجاح `make deps` احفظ `go.sum`:

```bash
git add go.sum
git commit -m "chore: add go.sum"
```

---

## 2. أنشئ المستودع وادفع الكود

### إن كان لديك `gh` CLI

```bash
gh auth status || gh auth login

gh repo create Devehab/linko \
  --public \
  --source . \
  --remote origin \
  --description "Turn any local port into a public HTTPS URL on your own domain, via Cloudflare Tunnel" \
  --push
```

### أو يدويًا

أنشئ مستودعًا عامًا فارغًا باسم `linko` من
[github.com/new](https://github.com/new) — **بدون** README أو .gitignore أو LICENSE
(كلها موجودة أصلًا) — ثم:

```bash
git remote add origin https://github.com/Devehab/linko.git
git push -u origin main
```

---

## 3. اضبط المستودع بعد الدفع

```bash
# الوسوم والوصف (يحسّن الظهور في البحث)
gh repo edit Devehab/linko \
  --add-topic cloudflare --add-topic cloudflare-tunnel \
  --add-topic tunnel --add-topic ngrok-alternative \
  --add-topic golang --add-topic cli --add-topic devtools \
  --enable-issues --enable-wiki=false
```

**GitHub Pages للتوثيق:** من `Settings → Pages` اختر **Source: GitHub Actions**.
سينشر workflow الجاهز صفحة `docs/index.html` على
`https://devehab.github.io/linko/`.

---

## 4. أول إصدار

بعد نجاح CI:

```bash
git tag -a v0.1.0 -m "linko v0.1.0"
git push origin v0.1.0
```

يبني workflow الإصدار الملفات التنفيذية لكل المنصات ويرفعها بأسماء
`linko_0.1.0_darwin_arm64.tar.gz` وأمثالها — وهي بالضبط الصيغة التي يتوقّعها
`install.sh`. عندها يعمل أمر التثبيت في README مباشرة:

```bash
curl -fsSL https://raw.githubusercontent.com/Devehab/linko/main/install.sh | bash
```

---

## ما الموجود في المستودع

| الملف | الغرض |
| --- | --- |
| `.github/workflows/ci.yml` | اختبارات على Linux/macOS/Windows + بناء لكل المنصات |
| `.github/workflows/release.yml` | بناء ونشر الإصدارات عند دفع وسم `v*` |
| `.github/workflows/pages.yml` | نشر `docs/` على GitHub Pages |
| `install.sh` | سكربت التثبيت (يقرأ من صفحة الإصدارات) |
| `docs/index.html` | دليل الاستخدام الكامل بالعربية |
| `Makefile` | `deps` · `test` · `race` · `cover` · `verify` · `build` · `release` |

---

> بعد الدفع، احذف هذا الملف — كل ما يهم القارئ موجود في `README.md`.
>
> ```bash
> git rm PUBLISH.md && git commit -m "chore: remove publishing notes"
> ```
