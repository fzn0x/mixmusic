<p align="center"><a href="README.md">English</a> · <a href="README.hi.md">हिन्दी</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.id.md">Bahasa Indonesia</a> · <strong>اردو</strong> · <a href="README.ha.md">Hausa</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.bn.md">বাংলা</a> · <a href="README.ru.md">Русский</a> · <a href="README.am.md">አማርኛ</a></p>

<p align="center">
  <img src="brand/icon.png" width="112" height="112" alt="MixMusic کا آئیکن: سیاہ پس منظر پر دو گلابی حلقے اور پلے کا بٹن" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center" dir="rtl"><strong>YouTube Music اور Spotify کے لیے ایک ہی پلیئر۔</strong></p>

<p align="center" dir="rtl">macOS، Windows اور Linux کے لیے YouTube Music اور Spotify کا کلائنٹ۔ YouTube Music بغیر اکاؤنٹ کے چلتا ہے۔ ایک سرچ، ایک قطار (queue)، ملے جلے پلے لسٹس، ہم وقت بول (lyrics)، تھیمز اور پلگ اِنز۔</p>

<p align="center" dir="rtl"><sub>یہ <a href="README.md">انگریزی README</a> کا ترجمہ ہے۔ کسی فرق کی صورت میں انگریزی نسخہ ہی تازہ ترین مانا جائے۔</sub></p>

<p align="center">
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml/badge.svg" alt="ویب سائٹ" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases/latest"><img src="https://img.shields.io/github/v/release/fzn0x/mixmusic" alt="تازہ ترین ریلیز" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases"><img src="https://img.shields.io/github/downloads/fzn0x/mixmusic/total" alt="ڈاؤن لوڈز" /></a>
  <a href="https://pkg.go.dev/github.com/fzn0x/mixmusic"><img src="https://pkg.go.dev/badge/github.com/fzn0x/mixmusic.svg" alt="Go Reference" /></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/fzn0x/mixmusic" alt="Go کا ورژن" /></a>
  <a href="https://awesomemygo.dev/apps/mixmusic"><img src="https://awesomemygo.dev/badge.svg" height="20" alt="Awesome MyGo میں درج" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey" alt="macOS، Windows اور Linux" />
  <a href="#لائسنس"><img src="https://img.shields.io/badge/license-MIT%20or%20GPL--3.0-blue" alt="MIT یا GPL-3.0" /></a>
</p>

<p align="center">
  <a href="https://fzn0x.github.io/mixmusic/">ویب سائٹ</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">ڈاؤن لوڈ</a> ·
  <a href="https://fzn0x.github.io/mixmusic/docs/">دستاویزات</a> ·
  <a href="CHANGELOG.md">تبدیلیوں کی فہرست</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic 0.5.9 اپنی ڈیفالٹ شکل میں، یعنی Mix تھیم کے ساتھ Blend لے آؤٹ، ایک تاریک اسٹیج پر سیدھا کھڑا اور آپ کی طرف رخ کیے ہوئے: بیچ میں YOASOBI کا アイドル چل رہا ہے، گلابی خاکے کے ساتھ، اس کے بولوں کی ہر سطر کے نیچے romaji ہے اور اس کے سامنے منی پلیئر ہے؛ بائیں طرف سرچ اور دائیں طرف لائبریری کے مکسز" />
</p>

<div dir="rtl">

## MixMusic کیوں

اگر آپ کی موسیقی YouTube Music اور Spotify میں بٹی ہوئی ہے تو عام طور پر
آپ کو دو ایپس چاہییں، دو سرچ، دو قطاریں اور دو لائبریریاں۔ MixMusic دونوں
سروسز کو ایک ہی پلیئر میں لے آتا ہے:

- **ایک سرچ۔** یہ YouTube Music اور Spotify میں ایک ساتھ تلاش کرتا ہے، اور
  ہر نتیجے پر اس سروس کا نشان لگاتا ہے جس سے وہ آیا ہے۔
- **ایک قطار اور ایک لائبریری۔** Spotify کا ٹریک YouTube Music کے ٹریک کے
  فوراً بعد چل سکتا ہے، اور ایک پلے لسٹ میں دونوں کے ٹریکس ہو سکتے ہیں۔
- **اکاؤنٹ کی ضرورت نہیں۔** ایپ کھولتے ہی YouTube Music چلنے لگتا ہے۔
  سائن اِن صرف تب کریں جب آپ اپنی لائبریری چاہتے ہوں۔ Spotify اختیاری ہے۔
- **ایک چھوٹا پروگرام۔** MixMusic ایک Go پروگرام ہے جو اپنی ونڈوز GPU پر خود
  بناتا ہے۔ اس کے اندر کوئی براؤزر نہیں جسے ڈاؤن لوڈ کرنا پڑے یا میموری میں
  رکھنا پڑے۔
- **آپ کے بدلنے کے لیے۔** چار لے آؤٹس اور چودہ تھیمز ساتھ آتے ہیں، کمیونٹی
  مزید تھیمز اور پلگ اِنز شامل کرتی ہے، اور ہر کمانڈ کے لیے آپ اپنی کلید
  مقرر کر سکتے ہیں۔
- **نجی۔** MixMusic کا اپنا کوئی اکاؤنٹ، سرور، analytics یا telemetry نہیں ہے۔
- **مفت اور اوپن سورس،** MIT لائسنس یا GPL 3.0 کے تحت۔

## خصوصیات

### سرچ اور پلے

- **دونوں سروسز کے لیے ایک سرچ۔** YouTube Music اور Spotify کے نتائج
  پہلو بہ پہلو، ہر ایک پر اس کی سروس کا نشان۔ YouTube Music کے نتائج میں
  دکھایا جاتا ہے کہ ہر ٹریک کتنی بار چلایا گیا، تاکہ اصل ٹریک کورز سے
  الگ نظر آئے۔
- **ایک قطار (queue)۔** دونوں سروسز کے ٹریکس ملائیں، ان میں شفل اور ریپیٹ
  کریں، اور قطار ختم ہونے پر ریڈیو کو آگے چلانے دیں۔
- **Explore۔** نئی ریلیزز، ٹرینڈنگ گانے، چارٹس، موڈز اور جانرز، اور جو چل رہا
  ہے اس سے ملتے جلتے گانے۔
- **ہم وقت بول (synced lyrics)** جو گانے کے ساتھ ساتھ چلتے ہیں، یا سادہ متن
  اگر آپ چاہیں۔
- **ایک منی پلیئر** جو آپ کی دوسری ونڈوز کے اوپر رہتا ہے۔

### آپ کی لائبریری

- **مکسز (Mixes):** ایسے پلے لسٹس جن میں ہر سروس کے ٹریکس ہوں۔ کسی سروس کے
  پلے لسٹ کو مکس میں امپورٹ کریں، اور ان میں سے کسی کے اندر سرچ اور ترتیب
  (sort) کریں۔
- **پسندیدہ گانے** دونوں سروسز کے، ایک ہی فہرست میں۔
- **ہسٹری اور سننے کے اعداد و شمار،** دن بہ دن۔
- **آپ کے اکاؤنٹس۔** اپنے ہوم فیڈ، پسندیدہ گانوں، پلے لسٹس، البمز اور
  فنکاروں کے لیے Google کے اپنے صفحے پر YouTube Music میں سائن اِن کریں، اپنے
  اکاؤنٹ کے طور پر یا اس کے کسی برانڈ چینل کے طور پر۔ وہاں کی لائبریری کے لیے
  Spotify میں سائن اِن کریں۔ پسندیدہ ٹریکس، محفوظ کیے گئے البمز اور پلے لسٹس،
  اور فالو کیے گئے فنکار سروس تک بھی پہنچتے ہیں۔ جو آپ چلاتے ہیں وہ آپ کی
  YouTube Music ہسٹری میں شامل ہو جاتا ہے، جس سے اس کی سفارشات بنتی ہیں۔

### Spotify، تین طریقوں سے

- **خود MixMusic میں،** Spotify کے معیار پر، بغیر کسی Spotify ایپ کے کھلے
  (Premium درکار، Windows اور macOS پر)۔
- **Spotify Connect پر:** آپ کی Spotify ایپ، آپ کا فون یا کوئی اسپیکر۔
- **YouTube Music سے:** MixMusic Spotify ٹریک کی وہ ریکارڈنگ YouTube Music پر
  چلاتا ہے، جس کے لیے Premium کی ضرورت نہیں۔

### آواز

- کراس فیڈ، یکساں بلندی اور والیوم لیولنگ۔
- پری سیٹس کے ساتھ دس بینڈ کا ایکولائزر۔
- ایک ویژولائزر۔
- آپ کے چلائے ہوئے ٹریکس کا آف لائن کیش۔

### آپ کے ڈیسک ٹاپ پر

- Windows، macOS اور Linux پر میڈیا کیز اور سسٹم کے میڈیا کنٹرولز۔
- اپنے گلوبل شارٹ کٹس: چلانے، روکنے، اسکِپ کرنے، والیوم بدلنے، خاموش کرنے،
  جو چل رہا ہے اسے لائک یا ڈس لائک کرنے، یا MixMusic دکھانے کے لیے، چاہے
  کوئی بھی ونڈو سامنے ہو۔
- Windows پر ٹاسک بار کے تھمب نیل میں پلیئر کے بٹن۔
- ٹرے کنٹرولز، نئے ٹریکس کی اطلاعات، اور لاگ اِن پر شروع ہونا۔
- `mixmusic://` لنکس۔
- **Last.fm:** آپ کا سنا ہوا ہر ٹریک آپ کی پروفائل میں شامل ہو جاتا ہے، اور
  آف لائن ہوئے اسکروبلز آپ کے آن لائن ہوتے ہی بھیج دیے جاتے ہیں۔
- **Discord:** آپ کی پروفائل دکھاتی ہے کہ کیا چل رہا ہے، اس کے آرٹ ورک اور
  باقی وقت کے ساتھ۔
- **اپڈیٹس:** MixMusic شروع ہونے پر اور ہر چند گھنٹوں بعد نئی ریلیز دیکھتا
  ہے، اس کے دستخط جانچتا ہے، اور آپ کے کہنے پر اسے انسٹال کرتا ہے، یا اگر آپ
  یہ چنیں تو خود ہی۔

### اسے اپنا بنائیں

- **لے آؤٹس۔** Blend، جو ڈیفالٹ ہے، بڑے کورز کی گھنی قطاروں پر گول شکلیں اور
  لچکدار حرکت رکھتا ہے۔ Expressive، جو Material 3 Expressive کے طرز پر ہے،
  شوخ ہے، ٹونل فلز، پِل بٹنز اور تیرتے پلیئر بار کے ساتھ۔ Crate سادہ ہے، تاکہ
  کورز ہی کام کریں، اور Tiles MixMusic کی پہلی شکل ہے، سپاٹ ٹائلز اور بڑے
  چھوٹے حروف (lowercase) کے الفاظ کے ساتھ۔
- **تھیمز۔** چودہ پہلے سے شامل ہیں، ہر ایک کسی بھی لے آؤٹ میں، ایسے ایفیکٹس
  کے ساتھ جنھیں آپ ٹیون کر سکتے ہیں، لائٹ موڈ کے لیے ایک تھیم اور ڈارک موڈ
  کے لیے ایک، اور اپنی پسند کا یا چلتے ٹریک کے آرٹ ورک سے لیا گیا ایکسنٹ
  رنگ۔ Material اور Material Dark ہر رنگ کور سے لیتے ہیں۔
- **کمیونٹی تھیمز اور پلگ اِنز،** مارکیٹ پلیس میں ایک کلک کی دوری پر۔
- **پلگ اِنز** ایک ٹائپڈ API کے ذریعے صفحات، کمانڈز، ٹریک ایکشنز اور ویجٹس
  شامل کرتے ہیں، جو آپ کے تھیم میں بنتے ہیں۔
- **کی بورڈ پہلے۔** ہر عام کام کے لیے شارٹ کٹس، ہر کمانڈ کے لیے آپ کی اپنی
  کلید، پلگ اِنز کی کمانڈز سمیت، ایسی کیز جو کسی بھی ایپ سے موسیقی کو
  کنٹرول کریں، اور باقی کے لیے ایک کمانڈ پیلیٹ (<kbd>Ctrl</kbd> <kbd>K</kbd>)۔
- **ایک سیٹنگز فائل۔** آپ کی سیٹنگز آپ کے ڈیٹا فولڈر میں `settings.json` ہیں
  (Windows پر `%AppData%\MixMusic`، macOS پر
  `~/Library/Application Support/MixMusic`، Linux پر `~/.config/MixMusic`)،
  جسے آپ اپنی dotfiles کے ساتھ رکھ سکتے ہیں۔ MixMusic کے چلتے ہوئے اسے محفوظ
  کریں تو وہ فوراً لاگو ہو جاتی ہے؛ Settings › Desktop › Settings file
  (ترتیبات › ڈیسک ٹاپ › سیٹنگز فائل) اسے کھولتا ہے۔

## ایک جھلک

</div>

<p align="center">
  <img src="docs/look-listen.webp" alt="Blend لے آؤٹ میں MixMusic کے صفحات، ایک تاریک اسٹیج پر سیدھے کھڑے اور آپ کی طرف رخ کیے ہوئے: بیچ میں YOASOBI کا アイドル چل رہا ہے، گلابی خاکے کے ساتھ، اس کے بولوں کی ہر سطر کے نیچے romaji اور انگریزی ترجمہ ہے؛ بائیں طرف البم THE BOOK 3 اور دائیں طرف فالو کیے گئے پوڈ کاسٹ کا صفحہ" />
</p>

<p align="center" dir="rtl"><sub><b>سنیں۔</b> بیچ میں YOASOBI کا アイドル چل رہا ہے: ہر سطر گاتے وقت روشن ہوتی ہے، اس کے نیچے romaji اور ترجمہ ہے۔ بائیں طرف ایک البم؛ دائیں طرف ایک پوڈ کاسٹ جسے آپ فالو کرتے ہیں۔</sub></p>

<p align="center">
  <img src="docs/look-yours.webp" alt="MixMusic کا مزید حصہ، ایک تاریک اسٹیج پر سیدھا کھڑا اور آپ کی طرف رخ کیے ہوئے: بیچ میں Settings › Appearance (ترتیبات › ظاہری شکل) جس میں چاروں لے آؤٹس گلابی خاکے کے ساتھ ہیں؛ بائیں طرف چلتے ٹریک کے کور سے رنگا ہوا Material تھیم اور دائیں طرف مارکیٹ پلیس کے کمیونٹی تھیمز" />
</p>

<p align="center" dir="rtl"><sub><b>اسے اپنا بنائیں۔</b> بیچ میں چاروں لے آؤٹس: Blend، جو ڈیفالٹ ہے، Expressive، Crate اور Tiles، تھیمز کے نیچے۔ بائیں طرف ایک Material تھیم جو اپنے رنگ چلتے کور سے لیتا ہے۔ دائیں طرف مارکیٹ پلیس کے کمیونٹی تھیمز۔</sub></p>

<div dir="rtl">

## انسٹال کریں

اپنے سسٹم کے لیے فائل
[تازہ ترین ریلیز](https://github.com/fzn0x/mixmusic/releases/latest) سے
ڈاؤن لوڈ کریں:

| سسٹم | ڈاؤن لوڈ |
|---|---|
| macOS 12 اور بعد کے (Apple silicon اور Intel) | [`mixmusic-<version>-macos-universal.dmg`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows 10 اور 11 (x64، Arm)، انسٹالر | [`mixmusic-<version>-windows-<arch>-setup.exe`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows، پورٹیبل، انسٹالیشن کے بغیر | [`mixmusic-<version>-windows-<arch>-portable.zip`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Debian، Ubuntu اور ان پر مبنی | [`mixmusic-<version>-linux-<arch>.deb`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Fedora، openSUSE اور ان پر مبنی | [`mixmusic-<version>-linux-<arch>.rpm`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Arch Linux اور اس پر مبنی (جلد آ رہا ہے) | AUR پر [`mixmusic-bin`](https://aur.archlinux.org/packages/mixmusic-bin): `yay -S mixmusic-bin` |
| کوئی بھی Linux، ایک فائل | [`mixmusic-<version>-linux-<arch>.AppImage`](https://github.com/fzn0x/mixmusic/releases/latest) |
| کوئی بھی Linux، سینڈ باکس میں (Flatpak) | [`mixmusic-<version>-linux-<arch>.flatpak`](https://github.com/fzn0x/mixmusic/releases/latest) |
| کوئی بھی Linux، آپ کے ہوم فولڈر میں | [`mixmusic-<version>-linux-<arch>.tar.gz`](https://github.com/fzn0x/mixmusic/releases/latest)، `install.sh` کے ساتھ |

`<arch>` x64 کے لیے `amd64` اور Arm کے لیے `arm64` ہے۔ ہر ریلیز میں
`SHA256SUMS` درج ہوتا ہے جس سے آپ ڈاؤن لوڈز جانچ سکتے ہیں۔ آپ کو `update-…`،
`….delta`، `mixmusic-<version>-macos-…` یا `-windows-<arch>.tar.gz` نام کی
فائلوں کی ضرورت نہیں: انسٹال شدہ ایپس انھی سے اپڈیٹ ہوتی ہیں۔

انسٹالرز، پورٹیبل zip اور Linux آرکائیو خود کو اپڈیٹ کر لیتے ہیں۔ `.deb`،
`.rpm`، AppImage اور Flatpak اپنی فائلیں خود نہیں بدل سکتے، اس لیے نیا ورژن
آنے پر MixMusic آپ کو بتاتا ہے اور اسے آپ کے لیے ڈاؤن لوڈ کر دیتا ہے۔ AUR کا
`mixmusic-bin` آپ کے باقی سسٹم کے ساتھ اپڈیٹ ہوتا ہے۔ پورٹیبل zip کو کہیں بھی
کھولیں اور `MixMusic.exe` چلائیں۔ AppImage کو executable بنائیں (`chmod +x`)
اور چلائیں۔ Flatpak کو اپنے سافٹ ویئر سینٹر سے کھولیں، یا
`flatpak install ./mixmusic-<version>-linux-<arch>.flatpak` چلائیں؛ پہلی بار
Flatpak، Flathub سے GNOME runtime لاتا ہے۔

جلد آ رہا ہے، جب اس کا tap تیار ہو جائے: Mac پر [Homebrew](https://brew.sh)
MixMusic انسٹال کرتا ہے، اور `brew upgrade` اسے اپڈیٹ کرتا ہے۔

</div>

```sh
brew install --cask fzn0x/tap/mixmusic
```

<div dir="rtl">

### پہلی بار چلانا

- **macOS:** ایپ پر ابھی Apple کی notarization نہیں ہے، اس لیے پہلی بار
  کھولنے پر macOS کہتا ہے کہ وہ اسے جانچ نہیں سکتا۔ اسے پھر بھی کھولیں: System
  Settings › Privacy & Security › Open Anyway سے، یا Terminal میں ایک بار یہ
  چلائیں:

  <div dir="ltr">

  ```sh
  xattr -dr com.apple.quarantine /Applications/MixMusic.app
  ```

  </div>

- **Windows:** انسٹالر پر ابھی دستخط نہیں ہیں، اس لیے SmartScreen خبردار کر سکتا
  ہے: More info چنیں، پھر Run anyway۔ YouTube Music میں سائن اِن کرنے اور
  MixMusic میں Spotify چلانے کے لیے WebView2 Runtime چاہیے، جو Windows 11 میں
  شامل ہے۔
- **Linux:** MixMusic کو GTK 3، اور PulseAudio (PipeWire بھی یہی فراہم کرتا ہے)
  یا ALSA چاہیے۔ YouTube Music میں سائن اِن کے لیے WebKitGTK 4.1، ٹرے آئیکن کے
  لیے `libayatana-appindicator3`، اور آپ کے سائن اِنز کو سسٹم کی keyring میں
  رکھنے کے لیے `libsecret` چاہیے۔ `.deb` اور `.rpm` انھیں انسٹال کر دیتے ہیں،
  اور Flatpak انھیں ساتھ لاتا ہے؛ AppImage اور آرکائیو کے لیے:

  <div dir="ltr">

  ```sh
  # Debian and Ubuntu
  sudo apt install libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
  # Fedora
  sudo dnf install gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
  ```

  </div>

- **ہر سسٹم:** MixMusic YouTube Music کے زیادہ تر ٹریکس کی آواز خود ڈھونڈ لیتا
  ہے۔ باقی کے لیے، جیسے بچوں کے لیے بنے ٹریکس، یہ
  [yt-dlp](https://github.com/yt-dlp/yt-dlp) استعمال کرتا ہے، جسے وہ آپ کے لیے
  Settings › Playback (ترتیبات › پلے بیک) سے انسٹال کرتا ہے اور اپڈیٹ رکھتا
  ہے۔ yt-dlp کو ایک JavaScript runtime چاہیے: [Deno](https://deno.com)،
  Node.js، Bun یا QuickJS۔ Flatpak میں Node.js موجود ہے۔

### Flatpak

Flatpak، MixMusic کو ایک سینڈ باکس میں چلاتا ہے، جسے نیٹ ورک، ساؤنڈ سرور،
ڈسپلے اور GPU نظر آتے ہیں، اور آپ کا ہوم فولڈر بالکل نہیں۔ MixMusic اپنی
سیٹنگز اور لائبریری `~/.var/app/io.github.fzn0x.mixmusic` میں رکھتا ہے، جو
`.deb`، `.rpm`، AppImage یا آرکائیو والی کاپی سے الگ ہے، اور دونوں ایک ساتھ
چل سکتی ہیں۔ کسی کاپی کی سیٹنگز اور لائبریری ساتھ لانے کے لیے دونوں بند
کریں، `settings.json`، `library.json`، `queue.json` اور `addons` فولڈر کو
`~/.config/MixMusic` سے
`~/.var/app/io.github.fzn0x.mixmusic/config/MixMusic` میں کاپی کریں، اور
اپنے اکاؤنٹس میں دوبارہ سائن اِن کریں: سائن اِنز دوسری کاپی کی keyring میں
رہتے ہیں۔

Flatpak میں لاگ اِن پر شروع ہونا آپ کے ڈیسک ٹاپ کے ذریعے ہوتا ہے، جو پہلے
آپ سے پوچھ سکتا ہے، اور آپ کا اپنا yt-dlp
`~/.var/app/io.github.fzn0x.mixmusic` میں ہونا چاہیے، کیونکہ سینڈ باکس کو کوئی
اور فولڈر نظر نہیں آتا۔ جو Discord ایپ Flatpak نہیں ہے وہ جو چل رہا ہے اسے
صرف تب دکھاتی ہے جب وہ MixMusic کے شروع ہونے کے وقت چل رہی ہو، ورنہ اپنے
دوبارہ شروع ہونے تک نہیں؛ Flatpak کے طور پر Discord دونوں صورتوں میں کام کرتا
ہے۔ Flatpak نیا ہے: ہمیں [issue](https://github.com/fzn0x/mixmusic/issues)
میں بتائیں کہ یہ آپ کے ڈیسک ٹاپ پر کیسا چلتا ہے۔

### اکاؤنٹس (سب اختیاری)

- **YouTube Music:** Settings › Accounts (ترتیبات › اکاؤنٹس) میں Sign in to
  YouTube Music چنیں اور کھلنے والی ونڈو میں Google میں سائن اِن کریں۔ آپ جو
  ٹائپ کرتے ہیں وہ صرف Google کو جاتا ہے۔ MixMusic سیشن کو انکرپٹ کر کے رکھتا
  ہے، جہاں ہو وہاں آپ کے سسٹم کی keyring میں رکھی کلید کے ساتھ۔ جہاں وہ ونڈو
  کھل نہ سکے، یا Google اسے رد کر دے، وہاں اپنے براؤزر سے سائن اِن کریں، جسے
  MixMusic ایک نئی اپنی پروفائل کے ساتھ کھولتا ہے اور آپ کے سائن اِن ہوتے ہی
  مٹا دیتا ہے؛ یا اس براؤزر کا سیشن امپورٹ کریں جس میں آپ سائن اِن ہیں
  (MixMusic اسے اپنے yt-dlp سے پڑھتا ہے، اور صرف YouTube کی کوکیز رکھتا ہے)؛ یا
  music.youtube.com کے سائن اِن شدہ ٹیب کا Cookie header پیسٹ کریں۔
- **Spotify:** Spotify ایک ایپ کو زیادہ سے زیادہ پانچ صارفین کی خدمت کرنے دیتا
  ہے، اور صرف تب تک جب تک اس کے مالک کے پاس Premium ہو، اس لیے آپ اپنی
  ذاتی Spotify ایپ سے سائن اِن کرتے ہیں۔ اس میں دو منٹ لگتے ہیں:
  [docs/spotify.md](docs/spotify.md) آپ کو قدم بہ قدم بتاتا ہے۔
- **Last.fm:** [last.fm/api/account/create](https://www.last.fm/api/account/create)
  پر ایک API اکاؤنٹ بنائیں (کوئی بھی نام، کوئی callback URL نہیں)، اس کی API
  key اور shared secret کو Settings › Accounts میں پیسٹ کریں، اور اپنے براؤزر
  میں MixMusic کو منظوری دیں۔
- **Discord:** جب تک Discord ایپ چل رہی ہو، آپ کی پروفائل دکھاتی ہے کہ آپ کیا
  سن رہے ہیں۔ اسے Settings › Desktop (ترتیبات › ڈیسک ٹاپ) میں بند کریں۔

کچھ کام نہیں کر رہا؟ [docs/troubleshooting.md](docs/troubleshooting.md) عام
مسائل اور معلوم خامیوں کا احاطہ کرتا ہے۔

## سورس سے بنائیں

آپ کو [Go](https://go.dev/dl/) 1.27.3 یا اس سے نیا، [Bun](https://bun.sh) 1.2
یا اس سے نیا، جو MyGo کی کمانڈ لائن چلاتا ہے، اور Git چاہیے۔ MyGo کو کسی C
کمپائلر کی ضرورت نہیں، اس لیے کوئی بھی سسٹم MixMusic کو ہر سسٹم کے لیے بنا
سکتا ہے۔

### Windows

</div>

```powershell
winget install GoLang.Go
winget install Oven-sh.Bun
winget install Git.Git
```

<div dir="rtl">

ایک نیا ٹرمینل کھولیں، پھر:

</div>

```powershell
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

<div dir="rtl">

سائن اِن اور Spotify پلے بیک کے لیے WebView2 Runtime چاہیے، جو Windows 11
میں شامل ہے۔ Windows 10 پر اسے
[Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/) سے
انسٹال کریں۔

### macOS

[Homebrew](https://brew.sh) کے ساتھ:

</div>

```sh
brew install go oven-sh/bun/bun
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

<div dir="rtl">

### Linux

Go کو [go.dev/dl](https://go.dev/dl/) سے انسٹال کریں (ڈسٹری بیوشن کے پیکیجز
اکثر 1.27.3 سے پرانے ہوتے ہیں)، پھر Bun اور وہ لائبریریاں جن کے ساتھ MixMusic
چلتا ہے:

</div>

```sh
curl -fsSL https://bun.sh/install | bash
# Debian and Ubuntu
sudo apt install git libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
# Fedora
sudo dnf install git gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
```

<div dir="rtl">

پھر:

</div>

```sh
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

<div dir="rtl">

### Nix

[Nix](https://nixos.org) کے ساتھ، `nix develop` آپ کو وہ ٹولز دیتا ہے جن سے
ریلیزز بنتی ہیں، انھی ورژنز پر جو `flake.lock` طے کرتا ہے: Go، Bun، NSIS،
GoReleaser اور Zola۔ `nix build` MixMusic کو Linux کے لیے سورس سے `result/` میں
بناتا ہے۔

### ڈیولپ کریں

`bun run dev` ایپ بناتا ہے، اسے چلاتا ہے، اور آپ کے ایڈٹ کرتے ہی دوبارہ
بناتا ہے۔ اس کا ڈیٹا انسٹال شدہ MixMusic سے الگ رہتا ہے، `MixMusic Dev` کے
نام سے۔

</div>

```sh
bun run build:plugins          # the plugins' JavaScript, from their TypeScript
go test ./...                  # the app's tests
bun run typecheck              # the plugin SDK and the community's plugins
go run ./tools/registry -check # the community registry
bun run build                  # the packaged app for this system, in build/
bun run build -- -platform windows/amd64,linux/amd64,linux/arm64
```

<div dir="rtl">

Nix شیل میں Linux ڈاؤن لوڈز
`.github/scripts/build.sh linux/amd64,linux/arm64` سے بنائیں، جیسے ریلیزز بنتی
ہیں۔ وہاں سادہ `bun run build` ایسا Linux پروگرام بناتا ہے جو Nix کا اپنا
loader مانگتا ہے، اور وہ صرف NixOS پر چلتا ہے۔
`go run ./tools/elfloader build/linux-amd64` کسی بلڈ کو جانچتا ہے۔

[ویب سائٹ](https://fzn0x.github.io/mixmusic/) `site/` میں ہے، جو
[Zola](https://www.getzola.org) سے بنتی ہے، اور `docs/` کی گائیڈز اس کے `/docs/`
صفحات ہیں۔ `go run ./tools/sitedocs && zola --root site serve` اس کا پیش نظارہ
دکھاتا ہے۔ ریلیزز Nix شیل میں بنتی ہیں اور
[GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`) سے شائع ہوتی ہیں۔

[docs/architecture](docs/architecture/README.md) بتاتا ہے کہ حصے آپس میں کیسے
جڑتے ہیں۔

### ساخت

ایپ روٹ پیکیج ہے، اور اس کے نیچے `internal/` کے پیکیجز ہیں۔ `internal/` سے
باہر کے پیکیجز ہر کسی کے امپورٹ کرنے کے لیے ہیں، اور
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic) پر دستاویزی ہیں۔

</div>

```
*.go                 the app and its windows: the pages, the player bar, the
                     mini player, the tray, media keys, deep links
fonts/               the typefaces the windows draw with
internal/app         the services the windows call, and the desktop integration
internal/player      the queue, played by the audio engine or on Spotify's devices
internal/audio       MixMusic's audio engine, and the system's audio output
internal/plugins     plugins, each in a JavaScript engine of its own
internal/youtube     YouTube Music's catalog, and your account there
internal/spotify     the Spotify Web API, its sign-in and Spotify Connect
internal/stream      audio streams, from YouTube or yt-dlp, opened for the engine
internal/match       Spotify tracks matched to their YouTube Music recordings
internal/romaji      Japanese lyrics in romaji, kanji read by kagome
internal/library     liked and disliked songs, mixes, history and stats
internal/lastfm      scrobbling to Last.fm
internal/discord     what plays, on your Discord profile
internal/diag        the log, without secrets, and the reports of crashes
internal/addons      themes and plugins, and the community registry
builtin/themes       the themes MixMusic ships with
public/              the page of Spotify's player, on Windows
sdk/                 the types of the plugin API
community/           community themes and plugins, and their registry
```

<div dir="rtl">

جو پیکیجز آپ امپورٹ کر سکتے ہیں:

</div>

```
opus/                an Opus decoder in pure Go, a port of libopus's
aac/                 an AAC-LC decoder in pure Go
ac3/                 an AC-3 and E-AC-3 decoder in pure Go, written from ATSC A/52
webm/                the audio of WebM files: packets, times and seeking
mp4/                 the audio of MP4 files: packets, times and seeking
resample/            sample rate conversion, with Kaiser-windowed sincs
lyrics/              synced lyrics from Unison and LRCLIB, and translations
brand/               MixMusic's logo, for code that draws it
```

<div dir="rtl">

## کمیونٹی

کوئی بھی مارکیٹ پلیس سے، یعنی سائڈ بار میں اسٹور کے آئیکن سے، کمیونٹی تھیمز
اور پلگ اِنز انسٹال کر سکتا ہے، اور آپ اپنے بھی شامل کر سکتے ہیں۔

### تھیم بنائیں

تھیم ایک `theme.json` فائل ہے جو رنگ، فونٹس اور کونے طے کرتی ہے، جو صارف کے
چنے ہوئے لے آؤٹ میں نظر آتے ہیں۔
[docs/themes.md](docs/themes.md) ہر ٹوکن کی فہرست دیتا ہے اور ایک مکمل تھیم
دکھاتا ہے۔ اپنا تھیم آزمانے کے لیے اس کا فولڈر تھیمز فولڈر میں رکھیں
(Settings › Appearance › open themes folder، یعنی ترتیبات › ظاہری شکل ›
تھیمز فولڈر کھولیں) اور Settings › Appearance میں اسے چنیں۔

### پلگ اِن بنائیں

پلگ اِن ایک چھوٹا ماڈیول ہے، جو TypeScript یا JavaScript میں لکھا جاتا ہے،
اور صفحات، کمانڈز، ٹریک ایکشنز اور ویجٹس شامل کرتا ہے۔
[docs/plugins.md](docs/plugins.md) API کو بیان کرتا ہے، آپ کے ایڈیٹر کے لیے
اس کی [types](sdk/index.d.ts) کے ساتھ۔ کمیونٹی کے
[sleep timer](community/plugins/sleep-timer) اور
[spectrum](community/plugins/spectrum) کام کرتی مثالیں ہیں۔

### شیئر کریں

1. اس repository کو fork کریں۔
2. ایک فولڈر شامل کریں: تھیم کے لیے `community/themes/<id>/`، یا پلگ اِن کے
   لیے `community/plugins/<id>/`، اس کی `plugin.json` اور اس کی `index.ts` کے
   ساتھ۔
3. `bun run build:plugins`، `go run ./tools/registry` اور
   `go run ./tools/registry -check` چلائیں۔
4. ایک pull request کھولیں۔

جب یہ merge ہو جاتا ہے تو CI اسے
[registry branch](https://github.com/fzn0x/mixmusic/tree/registry) پر شائع کر
دیتا ہے، اور یہ سب کے مارکیٹ پلیس میں نظر آنے لگتا ہے۔
[community/README.md](community/README.md) میں تفصیل ہے۔

## پرائیویسی

MixMusic کا اپنا کوئی اکاؤنٹ، سرور، analytics یا telemetry نہیں ہے۔ یہ ان سے
بات کرتا ہے:

- **YouTube اور Google:** YouTube Music کا کیٹلاگ، اس کی اسٹریمز اور ہر ٹریک
  کتنا بلند ہے، سائن آؤٹ حالت میں مانگی گئی، اور آرٹ ورک؛ کچھ اسٹریمز
  yt-dlp کے ذریعے۔ سائن اِن ہونے پر، وہاں کی آپ کی لائبریری، اور، جب تک آپ اسے
  Settings › Accounts (ترتیبات › اکاؤنٹس) میں بند نہ کریں، وہ ٹریکس جو آپ
  چلاتے ہیں، جو آپ کی YouTube Music ہسٹری میں شامل ہوتے ہیں۔
- **Spotify،** سائن اِن کرنے کے بعد: آپ کی لائبریری، آپ کے Spotify Connect
  ڈیوائسز، اور Spotify کا پلیئر، جو `sdk.scdn.co` سے لوڈ ہوتا ہے، جب MixMusic
  میں Spotify چلتا ہے۔
- **Unison:** چلتے ٹریک کی video ID، یا، اس کے بغیر، اس کا عنوان، فنکار،
  البم اور دورانیہ، تاکہ اس کے بول ملیں۔ صرف تب جب آپ بولوں کے ساتھ ترجمہ
  (translate) یا رومن (romanize) آن کریں: چلتے ٹریک کے بول، اس کی video ID کے
  ساتھ، جنھیں Unison، Google سے ترجمہ کرواتا ہے۔
- **LRCLIB:** چلتے ٹریک کا عنوان، فنکار، البم اور دورانیہ، تاکہ اس کے بول ملیں۔
- **GitHub:** شروع ہونے پر اور ہر چند گھنٹوں بعد نئے ورژن کی جانچ، جسے آپ
  Settings › About (ترتیبات › تعارف) میں بند کر سکتے ہیں؛ مارکیٹ پلیس کھولنے پر
  کمیونٹی registry؛ yt-dlp کے ڈاؤن لوڈز؛ اور، جاپانی بولوں کے ساتھ romaji پہلی
  بار آن کرنے پر، kanji کی ریڈنگز کی ڈکشنری (11.7 MB)۔ Romaji آپ کے کمپیوٹر پر
  بنتا ہے: بول بھیجے نہیں جاتے۔
- **Last.fm،** اسے جوڑنے کے بعد: جو آپ چلاتے ہیں۔
- **Discord،** جب تک آپ اسے بند نہ کریں: جو چل رہا ہے، آپ کے کمپیوٹر پر
  Discord ایپ کو بھیجا جاتا ہے، جو اسے آپ کی پروفائل پر دکھاتی ہے۔

جو کچھ یہ آپ کے کمپیوٹر پر رکھتا ہے:

- آپ کی سیٹنگز اور لائبریری (پسندیدہ اور ناپسندیدہ گانے، مکسز، ہسٹری اور
  حالیہ سرچز)، اس فولڈر میں جس کا نام Settings › About بتاتا ہے۔
- آپ کے چلائے ہوئے ٹریکس، کیش میں، اتنے جتنے آپ Settings › Playback
  (ترتیبات › پلے بیک) میں اجازت دیں، اور یہ کہ YouTube نے انھیں کتنا بلند
  ناپا۔
- آپ کے دیکھے ہوئے کورز، 256 MB تک۔
- بولوں کے وہ ترجمے جو آپ نے آن کیے، ٹریک اور زبان کے حساب سے، اور وہ
  ڈکشنری جس سے romaji، kanji پڑھتا ہے، جب آپ اسے آن کر لیں۔
- آپ کے سائن اِنز، YouTube Music کے سیشن سمیت، سسٹم کی keyring میں۔ Windows پر،
  جب Credential Manager کسی کو نہ رکھ سکے تو وہ آپ کے Windows اکاؤنٹ کے لیے
  انکرپٹ کی گئی فائل میں جاتا ہے؛ keyring کے بغیر Linux پر، ایسی فائلوں میں جو
  صرف آپ کا صارف پڑھ سکتا ہے۔
- لاگ، آپ کے سائن اِنز کے بغیر۔

لائبریری کا ہسٹری صفحہ آپ کا چلایا ہوا صاف کرتا ہے، Settings › Playback کیش
اور کورز صاف کرتا ہے، اور کسی سروس سے سائن آؤٹ کرنا اس کا سائن اِن مٹا دیتا
ہے۔

Windows پر، ویژولائزر اس کمپیوٹر پر چلتے Spotify کی آواز سنتا ہے، چاہے
MixMusic میں ہو یا Spotify ایپ میں، Windows کے process loopback کے ذریعے، جو
صرف اسی ایپ کو سنتا ہے۔ macOS 14.2 اور بعد کے ورژنز پر، یہ MixMusic میں چلتے
Spotify کو process tap کے ذریعے سنتا ہے، جب آپ اجازت دیں، جیسا کہ macOS پوچھتا
ہے۔ یہ آواز کا تجزیہ میموری میں کرتا ہے اور اسے کبھی ریکارڈ یا ارسال نہیں
کرتا۔

## کریڈٹس

MixMusic ان پروجیکٹس پر بنا ہے:

- **[MyGo](https://github.com/egoist/mygo)،** وہ ڈیسک ٹاپ فریم ورک جس سے
  MixMusic بنا ہے۔ MixMusic، MyGo کا native UI استعمال کرتا ہے: MyGo ہر ونڈو
  خود Go میں بناتا ہے، webview کے بغیر، صفحات اور پلیئر بار سے لے کر منی
  پلیئر تک۔ اس کی کمانڈ لائن ایپ کو انسٹالرز کے ساتھ پیک کرتی ہے، اور ان
  دستخط شدہ اپڈیٹس کے ساتھ جن سے وہ خود کو اپڈیٹ کرتی ہے۔
- **[Go](https://go.dev)،** ایپ کی زبان، [purego](https://github.com/ebitengine/purego)
  کے ساتھ، جو cgo کے بغیر سسٹم کی لائبریریوں کو کال کرتا ہے، اور
  [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) اور
  [golang.org/x/text](https://pkg.go.dev/golang.org/x/text)۔
- **[Sobek](https://github.com/grafana/sobek)،** وہ JavaScript انجن جس میں ہر
  پلگ اِن چلتا ہے۔
- **[Nix](https://nixos.org)،** جو ریلیزز بنانے والے ٹولز کو طے کرتا ہے، اور
  MixMusic کو Linux کے لیے سورس سے بناتا ہے۔
- **[GoReleaser](https://goreleaser.com)،** جو ریلیزز شائع کرتا ہے، اور اس کا
  [nFPM](https://nfpm.goreleaser.com)، جو RPM پیکیجز بناتا ہے۔
- **[Bun](https://bun.sh)،** جو بلڈ چلاتا ہے، اور
  **[TypeScript](https://www.typescriptlang.org)،** جو پلگ اِنز کمپائل کرتا ہے
  اور پلگ اِن SDK کی types جانچتا ہے۔
- **[Zola](https://www.getzola.org)،** جو ویب سائٹ بناتا ہے۔
- **[NSIS](https://nsis.sourceforge.io)،** جو Windows انسٹالرز بناتا ہے، اور
  [appimagetool](https://github.com/AppImage/appimagetool)، AppImages۔
- **[yt-dlp](https://github.com/yt-dlp/yt-dlp)،** جو ان ٹریکس کی آواز ڈھونڈتا
  ہے جو MixMusic خود نہیں ڈھونڈ پاتا۔
- **[Unison](https://unison.betterlyrics.org)،** بولوں کا کھلا، عوامی تعاون سے
  بنا ڈیٹا بیس، ODbL کے تحت، اور **[LRCLIB](https://lrclib.net)،** ہم وقت
  بولوں کا کھلا ڈیٹا بیس۔
- **[kagome](https://github.com/ikawaha/kagome)،** جاپانی morphological
  analyser جو بولوں کے kanji کو ان کے romaji کے لیے پڑھتا ہے، اور
  [kagome-dict](https://github.com/ikawaha/kagome-dict) کی IPA ڈکشنری کے
  ساتھ، جو mecab-ipadic سے بنی ہے، اور جسے MixMusic پہلی بار romaji آن ہونے
  پر ڈاؤن لوڈ کرتا ہے۔
- **[libopus](https://opus-codec.org)،** جس کے ڈیکوڈر کا Opus ڈیکوڈر پورٹ ہے،
  اور **[SharpJaad](https://github.com/jimm98y/SharpJaad)،** [JAAD](https://sourceforge.net/projects/jaadec/)
  کا C# پورٹ جس سے AAC ڈیکوڈر اخذ کیا گیا ہے۔
- **[FFmpeg](https://ffmpeg.org)،** جس سے ڈیکوڈرز کے ٹیسٹ موازنہ کرتے ہیں۔
- **[Selawik](https://github.com/microsoft/Selawik)،**
  **[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily)،**
  **[Roboto Flex](https://github.com/googlefonts/roboto-flex)** اور
  **[Archivo](https://github.com/Omnibus-Type/Archivo)،** ونڈوز کے ٹائپ فیسز۔
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities)،**
  جس کی HCT رنگ سائنس کا Material تھیمز کے رنگ پورٹ ہیں۔
- **[Kevin MacLeod](https://incompetech.com)،** جن کی موسیقی،
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) کے تحت، ویب سائٹ
  پر چلتی ہے۔

## تعاون

اصلاحات، نئی خصوصیات، تھیمز، پلگ اِنز اور بگ رپورٹس سب کا خیر مقدم ہے۔

- **مسئلے کی رپورٹ دیں** Settings › About › report a problem
  (ترتیبات › تعارف › مسئلے کی رپورٹ) سے۔ یہ GitHub پر ایک نیا issue کھولتا ہے
  جس میں آپ کا ورژن، سسٹم، تھیم اور پلگ اِنز پہلے سے بھرے ہوتے ہیں۔ یا خود
  [issue کھولیں](https://github.com/fzn0x/mixmusic/issues/new)۔
- **تبدیلی بھیجیں** pull request کے طور پر۔ [CONTRIBUTING.md](CONTRIBUTING.md)
  بتاتی ہے کہ کون سی جانچیں چلانی ہیں، commit کا انداز کیا ہے، اور وہ changelog
  اندراج کیا ہے جو لوگوں کو نظر آنے والی تبدیلی کے لیے چاہیے۔
- **تھیم یا پلگ اِن شیئر کریں:** دیکھیں [کمیونٹی](#کمیونٹی)۔

[CONTRIBUTORS.md](CONTRIBUTORS.md) ان لوگوں کا شکریہ ادا کرتی ہے جن کے کوڈ،
خیالات اور رپورٹس نے MixMusic کو شکل دی۔

## لائسنس

MixMusic دوہرے لائسنس کے تحت ہے: اسے [MIT لائسنس](LICENSE-MIT) یا
[GNU General Public License، ورژن 3](LICENSE-GPL) کے تحت استعمال کریں، جو آپ
کو پسند ہو۔ شراکتیں دونوں کے تحت قبول کی جاتی ہیں۔ `fonts` کے ٹائپ فیسز SIL
Open Font License کے تحت ہیں: [Selawik کا](fonts/OFL.txt)،
[Quicksand کا](fonts/quicksand-OFL.txt)،
[Roboto Flex کا](fonts/robotoflex-OFL.txt) اور
[Archivo کا](fonts/archivo-OFL.txt)۔ Opus ڈیکوڈر libopus کا پورٹ ہے،
[اس کے BSD لائسنس](opus/LICENSE.libopus) کے تحت، AAC ڈیکوڈر SharpJaad سے
اخذ کیا گیا ہے، [اس کے MIT لائسنس](aac/LICENSE.sharpjaad) کے تحت، اور HCT رنگ
سائنس Material Color Utilities کا پورٹ ہے،
[اس کے Apache لائسنس](internal/hct/LICENSE.material-color-utilities) کے تحت۔

MixMusic کا Spotify یا Google سے کوئی الحاق نہیں، نہ انھوں نے اسے تصدیق کیا ہے
اور نہ ہی یہ ان کا اسپانسر کردہ ہے۔ یہ وہ چلاتا ہے جو یہ سروسز اپنے صارفین کو
پیش کرتی ہیں، ان سروسز کے اپنے انٹرفیسز کے ذریعے؛ اسے ان کی شرائط کی اجازت کے
مطابق استعمال کریں۔ Spotify اور YouTube Music اپنے مالکان کے ٹریڈ مارکس ہیں۔

</div>
