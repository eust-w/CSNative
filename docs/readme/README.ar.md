<div dir="rtl">

# CS Native

<p align="center">
  <strong>لغة README</strong><br />
  <a href="../../README.md">🇨🇳 中文</a> ·
  <a href="README.en.md">🇺🇸 English</a> ·
  <a href="README.ko.md">🇰🇷 한국어</a> ·
  <a href="README.ar.md">🇸🇦 العربية</a> ·
  <a href="README.de.md">🇩🇪 Deutsch</a> ·
  <a href="README.fr.md">🇫🇷 Français</a> ·
  <a href="README.la.md">🇻🇦 Latina</a>
</p>

<p align="center">
  <a href="https://github.com/eust-w/CSNative/actions/workflows/release.yml"><img src="https://github.com/eust-w/CSNative/actions/workflows/release.yml/badge.svg" alt="Release workflow" /></a>
  <a href="https://github.com/eust-w/CSNative/actions/workflows/pages.yml"><img src="https://github.com/eust-w/CSNative/actions/workflows/pages.yml/badge.svg" alt="Pages workflow" /></a>
  <a href="https://github.com/eust-w/CSNative/releases/latest"><img src="https://img.shields.io/github/v/release/eust-w/CSNative?label=release" alt="Latest release" /></a>
</p>

<p align="center">
  <img src="../../build/appicon.png" alt="CS Native icon" width="104" />
</p>

<p align="center">
  <strong>CS Native هو provider/runtime control plane محلي لـ Claude Science.</strong>
</p>

<p align="center">
  يشغل Claude Science داخل صندوق معزول، ويوجه طلبات النماذج إلى provider خارجي قمت بإعداده، مع إبقاء وضع Claude الرسمي كمدخل مستقل.
</p>

<p align="center">
  <img src="../assets/readme/csn-flow.gif" alt="CS Native flow animation" width="860" />
</p>

## ما هو

CS Native ليس محولا بسيطا بين DeepSeek وQwen. إنه لوحة تحكم محلية لإدارة:

- provider profiles: base URL وAPI key وadapter والنموذج الافتراضي وربط نماذج Science.
- الوكيل المحلي وOAuth الافتراضي وصندوق Claude Science ومدخل fresh nonce العام.
- وضع النماذج الخارجية داخل صندوق معزول فقط.
- وضع Claude الرسمي بشكل مستقل لمسار Claude Science الحقيقي.

## الاستخدام الأساسي

1. افتح **النماذج والمفاتيح**.
2. اختر provider preset مدمجا أو أنشئ provider مخصصا.
3. املأ `base URL` و`API key` و`adapter` والنموذج الافتراضي وربط نماذج Science.
4. اضغط **تحقق**.
5. ارجع إلى **البدء**، اختر provider مفعلا مع key محفوظ، ثم اضغط **بدء**.
6. انتقل إلى وضع **Claude الرسمي** عندما تريد مسار Claude Science الحقيقي.

## i18n

يدعم CS Native اللغات `zh-CN` و`en` و`ko` و`ar` و`de` و`fr` و`la`.

- تطبيق سطح المكتب: اختيار اللغة في الشريط الجانبي.
- GitHub Pages: اختيار اللغة في أعلى الصفحة.
- العربية تستخدم تخطيط RTL في التطبيق والموقع.

## الواجهات

### البدء

![Start page](../assets/readme/hero-start.png)

### النماذج والمفاتيح

![Models and keys](../assets/readme/provider-management.png)

### الشبكة

![Network page](../assets/readme/network-public-entry.png)

### السجلات

![Logs page](../assets/readme/runtime-logs.png)

## التنزيل

صفحة المنتج: [https://eust-w.github.io/CSNative/](https://eust-w.github.io/CSNative/)

تتوفر أحدث الإصدارات في [GitHub Releases](https://github.com/eust-w/CSNative/releases/latest).

| النظام / المعمارية | الملف | المحتوى |
| --- | --- | --- |
| macOS Apple Silicon | `CSNative-v*-darwin-arm64.zip` | حزمة macOS `.app` |
| macOS Intel | `CSNative-v*-darwin-amd64.zip` | حزمة macOS `.app` |
| Windows x64 | `CSNative-v*-windows-amd64.zip` | حزمة تنفيذ Windows |
| Linux x64 | `CSNative-v*-linux-amd64.tar.gz` | حزمة تنفيذ Linux |
| كل المنصات | `checksums.txt` | فحوص SHA-256 |

## حدود التشغيل

- دليل الإعداد: `~/.csnative`
- Sandbox HOME: `~/.csnative/sandbox/home`
- السجل المحلي: `~/.csnative/logs/csnative.log`
- تحفظ مفاتيح provider في `~/.csnative/config.json` بصلاحية `0600`.
- لا يكتب CS Native في دليل اعتماد Claude Science الحقيقي.

## المصدر

الوثيقة الأساسية هي README الصينية: [README.md](../../README.md)، وملاحظات العلامة في [docs/brand.md](../brand.md).

</div>
