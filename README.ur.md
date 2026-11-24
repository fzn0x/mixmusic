<p align="center"><a href="README.md">English</a> · <a href="README.hi.md">हिन्दी</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.id.md">Bahasa Indonesia</a> · <strong>اردو</strong> · <a href="README.ha.md">Hausa</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.bn.md">বাংলা</a> · <a href="README.ru.md">Русский</a> · <a href="README.am.md">አማርኛ</a></p>

<p align="center">
  <img src="brand/icon.png" width="112" height="112" alt="MixMusic کا آئیکن: سیاہ پس منظر پر دو گلابی حلقے اور پلے کا بٹن" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center" dir="rtl"><strong>YouTube Music اور Spotify کے لیے ایک ہی پلیئر۔</strong></p>

<p align="center" dir="rtl">macOS، Windows اور Linux کے لیے YouTube Music اور Spotify کا کلائنٹ۔ YouTube Music بغیر اکاؤنٹ کے چلتا ہے۔ ایک سرچ، ایک قطار (queue)، ملے جلے پلے لسٹس، ہم وقت بول (lyrics)، پوڈ کاسٹس، آپ کی اپنی موسیقی کی فائلیں، کاسٹنگ، تھیمز اور پلگ اِنز۔</p>

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
  <a href="https://trendshift.io/repositories/313095?utm_source=trendshift-badge&amp;utm_medium=badge&amp;utm_campaign=badge-trendshift-313095" target="_blank" rel="noopener noreferrer"><img src="https://trendshift.io/api/badge/trendshift/repositories/313095/daily?language=Go" alt="fzn0x%2Fmixmusic | Trendshift" width="250" height="55"/></a>
</p>

<p align="center">
  <a href="https://mixmusic.functionx.dev/">ویب سائٹ</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">ڈاؤن لوڈ</a> ·
  <a href="https://mixmusic.functionx.dev/docs/">دستاویزات</a> ·
  <a href="docs/features.md">ہر خصوصیت</a> ·
  <a href="CHANGELOG.md">تبدیلیوں کی فہرست</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic اپنی ڈیفالٹ شکل میں، یعنی Mix تھیم کے ساتھ Blend لے آؤٹ، ایک تاریک اسٹیج پر سیدھا کھڑا اور آپ کی طرف رخ کیے ہوئے: بیچ میں YOASOBI کا アイドル چل رہا ہے، گلابی خاکے کے ساتھ، اس کے بولوں کی ہر سطر کے نیچے romaji ہے اور اس کے سامنے منی پلیئر ہے؛ بائیں طرف سرچ اور دائیں طرف لائبریری کے مکسز" />
</p>

<div dir="rtl">

## MixMusic کیوں

اگر آپ کی موسیقی YouTube Music اور Spotify میں بٹی ہوئی ہے تو عام طور پر
آپ کو دو ایپس چاہییں، دو سرچ، دو قطاریں اور دو لائبریریاں۔ MixMusic دونوں
سروسز کو ایک ہی پلیئر میں لے آتا ہے:

- **ایک سرچ۔** یہ YouTube Music اور Spotify میں ایک ساتھ تلاش کرتا ہے، اور
  ہر نتیجے پر اس سروس کا نشان لگاتا ہے جس سے وہ آیا ہے۔
- **ایک قطار اور ایک لائبریری۔** Spotify کا ٹریک YouTube Music کے ٹریک کے
  فوراً بعد چل سکتا ہے، اور ایک پلے لسٹ میں دونوں کے ٹریکس ہو سکتے ہیں، اور
  آپ کے کمپیوٹر پر موجود موسیقی کی فائلیں بھی۔
- **اکاؤنٹ کی ضرورت نہیں۔** ایپ کھولتے ہی YouTube Music چلنے لگتا ہے۔
  سائن اِن صرف تب کریں جب آپ اپنی لائبریری چاہتے ہوں۔ Spotify اختیاری ہے۔
- **ایک چھوٹا پروگرام۔** MixMusic ایک Go پروگرام ہے جو اپنی ونڈوز GPU پر خود
  بناتا ہے۔ اس کے اندر کوئی براؤزر نہیں جسے ڈاؤن لوڈ کرنا پڑے یا میموری میں
  رکھنا پڑے۔
- **آپ کے بدلنے کے لیے۔** چار لے آؤٹس اور چودہ تھیمز ساتھ آتے ہیں، کمیونٹی
  مزید تھیمز اور پلگ اِنز شامل کرتی ہے، اور ہر کمانڈ کے لیے آپ اپنی کلید
  مقرر کر سکتے ہیں۔
- **آپ کی زبان میں۔** ونڈو انگریزی کے علاوہ 66 زبانوں میں آتی ہے، اور عربی،
  عبرانی، فارسی اور اردو کے لیے دائیں سے بائیں ترتیب میں۔
- **نجی۔** MixMusic کا اپنا کوئی اکاؤنٹ، سرور، analytics یا telemetry نہیں ہے۔
- **مفت اور اوپن سورس،** MIT لائسنس یا GPL 3.0 کے تحت۔

## خصوصیات

MixMusic کیا کرتا ہے، حصہ بہ حصہ، اور ایپ میں آپ اسے کہاں پائیں گے۔
**پہلے سے شامل** کا مطلب ہے کہ یہ خود بخود کام کرتا ہے۔
[خصوصیات](docs/features.md) 0.1.0 سے 0.5.12 تک کی تمام 363 خصوصیات ایک ایک
سطر میں گنواتی ہے، ساتھ اس ورژن کے جس میں ہر ایک آئی۔

### سرچ اور پلے

- **دونوں سروسز کے لیے ایک سرچ۔** YouTube Music اور Spotify کے نتائج
  پہلو بہ پہلو، ہر ایک پر اس کی سروس کا نشان، اور ان کے ساتھ آپ کی اپنی
  موسیقی کی فائلیں۔ YouTube Music کے نتائج میں دکھایا جاتا ہے کہ ہر ٹریک
  کتنی بار چلایا گیا، تاکہ اصل ٹریک کورز سے الگ نظر آئے۔ *اوپر کی بار*
- **ہر قسم کا نتیجہ۔** گانے، ویڈیوز، البمز، فنکار، پلے لسٹس اور پوڈ کاسٹس،
  بہترین مطابقت، سب سے زیادہ چلائے گئے یا سب سے نئے کی ترتیب میں، اور خود
  YouTube بھی، ان لائیو سیٹس اور اپ لوڈز کے لیے جو میوزک کیٹلاگ میں نہیں
  ہیں۔ سرچ باکس میں چسپاں کیا گیا YouTube یا Spotify کا لنک فوراً چلنے لگتا
  ہے۔ *سرچ*
- **ایک قطار (queue)۔** دونوں سروسز کے ٹریکس ملائیں، ان میں شفل اور ریپیٹ
  کریں، کسی ٹریک کو اگلا چلنے کے لیے آگے لے آئیں، اور قطار ختم ہونے پر
  ریڈیو کو آگے چلانے دیں۔ ری اسٹارٹ کے بعد یہ ویسی ہی لوٹ آتی ہے جیسی آپ نے
  چھوڑی تھی۔ *پلیئر بار، قطار*
- **ہر چیز کے لیے ایک صفحہ۔** البمز، فنکار اور پلے لسٹس، Play اور Shuffle
  کے ساتھ، کسی فنکار کے تمام گانے، اور جو چل رہا ہے اس سے ملتی جلتی موسیقی۔
  *سرچ، ہوم*
- **جو نہ چاہیں اسے چھوڑ دیں۔** گانوں کو ڈس لائک کریں تاکہ ریڈیو انھیں چھوڑ
  دیں، AI سے بنے گانے اور ویڈیوز کے وہ حصے چھوڑ دیں جنھیں SponsorBlock
  نشان زد کرتا ہے، اور explicit مواد چھپائیں یا صاف ورژنز کو ترجیح دیں۔
  *پلیئر بار، Settings › Playback*
- **سلیپ ٹائمر اور رفتار۔** کچھ وقت، ایک ٹریک یا پوری قطار کے بعد رک جائیں،
  آہستہ آہستہ آواز کم کرتے ہوئے، یا رات کو خود بخود، اور گانے اور پوڈ کاسٹس
  اپنی اصل پچ پر تیز یا آہستہ چلائیں۔ *پلیئر بار*
- **میوزک ویڈیوز اور حرکت کرتے کورز۔** گانے کے کور کی جگہ اس کی میوزک ویڈیو،
  اور وہ چھوٹی ویڈیوز جو کچھ البمز بار بار چلاتے ہیں۔ *اب چل رہا ہے،
  Settings › Playback*
- **چنیں، شیئر کریں اور قریب سے دیکھیں۔** کئی ٹریکس چن کر ایک ساتھ قطار میں
  لگائیں، شامل کریں یا ڈاؤن لوڈ کریں، کسی گانے یا البم کو تصویری کارڈ کے طور
  پر شیئر کریں، اور دیکھیں کہ ٹریک کہاں سے آتا ہے اور کیوں نہیں چل سکا۔
  *ٹریک کا مینو*

### ہوم اور Explore

- **ہوم** آپ کی شیلفوں کے ساتھ، Jump back in، اور جن فنکاروں کو آپ فالو کرتے
  ہیں ان کی نئی ریلیزز، اس ترتیب میں جو آپ چاہیں۔ *ہوم*
- **Explore:** نئی ریلیزز، چارٹس، موڈز اور جانرز۔ *Explore*

### بول (Lyrics)

- **ہم وقت بول** جو گانے کے ساتھ ساتھ چلتے ہیں، لفظ بہ لفظ جہاں ماخذ میں یہ
  موجود ہو، پوری اسکرین پر بھی، یا سادہ متن اگر آپ چاہیں۔ *بولوں کا پینل،
  اب چل رہا ہے*
- **ترجمہ اور لاطینی حروف:** ہر سطر کے نیچے ترجمہ، جاپانی کے لیے romaji، اور
  کورین، چینی، روسی، عربی اور بہت سی دوسری رسم الخطوں کے لیے لاطینی حروف۔
  *بولوں کا پینل*
- **ماخذ کا آپ کا انتخاب:** Better Lyrics، Unison، LRCLIB، YouTube Music اور
  YouTube کے کیپشنز، اس ترتیب میں جو آپ طے کریں، اور کوئی دوسرا ماخذ، سرچ،
  آپ کی اپنی ترامیم اور ٹائمنگ ایک کلک کی دوری پر۔ *Settings › Playback ›
  Lyrics، بولوں کا پینل*
- **بولوں میں تلاش کریں،** ان کی سطریں کارڈ کے طور پر شیئر کریں، اور وہ بول
  پڑھیں جو آپ کی اپنی گانوں کی فائلوں میں موجود ہیں۔ *بولوں کا پینل*

### پوڈ کاسٹس

- **YouTube Music اور Spotify کے پوڈ کاسٹس۔** شوز ڈھونڈیں اور فالو کریں،
  ہر قسط وہیں سے شروع کریں جہاں آپ نے چھوڑی تھی، اقساط کو سنا ہوا نشان زد
  کریں، ان کے ابواب اور ٹرانسکرپٹس پڑھیں، اور انھیں گانوں سے الگ رفتار دیں۔
  *سرچ، شو کا صفحہ، لائبریری › پوڈ کاسٹس*

### آپ کی لائبریری

- **مکسز (Mixes):** ایسے پلے لسٹس جن میں ہر سروس کے ٹریکس ہوں، آپ کی اپنی
  ترتیب میں، سرچ اور ترتیب (sort) کے ساتھ، اور آپ کے اپنے کورز کے ساتھ۔
  *لائبریری › مکسز، سائڈ بار*
- **پسندیدہ گانے** دونوں سروسز کے، ایک ہی فہرست میں، اور آپ کے YouTube Music
  اور Spotify کے پلے لسٹس، البمز اور فنکار، جنھیں آپ محفوظ کر سکتے ہیں، بنا
  سکتے ہیں، شیئر کر سکتے ہیں اور جن پر مل کر کام کر سکتے ہیں۔ لائکس اور سیوز
  سروس تک بھی پہنچتے ہیں۔ *لائبریری*
- **پنز، فولڈرز اور ٹیگز** بڑی لائبریری کو ترتیب میں رکھتے ہیں، اور جو آپ
  وہاں نہیں چاہتے وہ چھپایا جا سکتا ہے۔ *سائڈ بار، ٹائل کا مینو*
- **فنکاروں کو فالو کریں،** اکاؤنٹ کے ساتھ یا اس کے بغیر، اور ان کی نئی
  ریلیزز دیکھیں۔ *فنکار کا صفحہ، لائبریری › فنکار*
- **ہسٹری، اعداد و شمار اور آپ کا سال:** روزانہ منٹ، آپ کے سب سے زیادہ سنے
  گئے گانے اور فنکار، اور شیئر کرنے کے لیے ان کے کارڈ۔ *لائبریری › ہسٹری،
  اعداد و شمار*
- **ڈاؤن لوڈز** نیٹ ورک کے بغیر چلانے کے لیے، اور آپ کے چلائے ہوئے ٹریکس کا
  آف لائن کیش۔ *ٹریک کا مینو، لائبریری › ڈاؤن لوڈز*

### اس کمپیوٹر پر موسیقی

- **آپ کی اپنی موسیقی کی فائلیں** لائبریری اور سرچ میں، ان فولڈرز سے جن پر
  MixMusic نظر رکھتا ہے: MP3، FLAC، WAV، M4A، Opus اور Vorbis، ان کے ٹیگز اور
  کورز کے ساتھ۔ *Settings › Playback › Music on this computer*
- **ان کے ٹیگز میں ترمیم کریں،** ایک ہی گانے کی کاپیاں اکٹھی کریں، اور فائلیں
  کھول کر یا ونڈو پر گرا کر چلائیں۔ *ٹریک کا مینو، ونڈو پر ڈراپ کریں*

### اکاؤنٹس اور سروسز

- **YouTube Music۔** Google کے اپنے صفحے پر سائن اِن کریں، اپنے براؤزر کے
  ذریعے یا کسی براؤزر کے سیشن سے، اپنے اکاؤنٹ کے طور پر یا اس کے کسی برانڈ
  چینل کے طور پر، اپنے ہوم فیڈ، پسندیدہ گانوں، پلے لسٹس، البمز اور فنکاروں
  کے لیے۔ جو آپ چلاتے ہیں وہ آپ کی YouTube Music ہسٹری میں شامل ہو جاتا ہے،
  جس سے اس کی سفارشات بنتی ہیں۔ *Settings › Accounts*
- **Last.fm:** آپ کا سنا ہوا ہر ٹریک، آدھا چلنے پر یا چار منٹ بعد، آپ کی
  پروفائل میں شامل ہو جاتا ہے، اور آف لائن ہوئے اسکروبلز آپ کے آن لائن ہوتے
  ہی بھیج دیے جاتے ہیں۔ *Settings › Accounts*
- **ListenBrainz:** یہی، Last.fm کے ساتھ یا اس کے بغیر، ListenBrainz پر یا
  آپ کے اپنے کسی سرور پر، جیسے Maloja۔ لائک کیے گئے ٹریکس ان میں سے کسی پر بھی
  love کے طور پر جا سکتے ہیں۔ *Settings › Accounts*
- **Libre.fm:** یہی، Libre.fm پر یا کسی دوسرے GNU FM سرور پر، بغیر کوئی API
  اکاؤنٹ بنائے۔ *Settings › Accounts*
- **Discord:** آپ کی پروفائل دکھاتی ہے کہ کیا چل رہا ہے، اس کے آرٹ ورک اور
  باقی وقت کے ساتھ، جیسا آپ طے کریں۔ *Settings › Desktop*

### Spotify، تین طریقوں سے

- **خود MixMusic میں،** Spotify کے معیار پر، بغیر کسی Spotify ایپ کے کھلے
  (Premium درکار، Windows اور macOS پر)۔
- **Spotify Connect پر:** آپ کی Spotify ایپ، آپ کا فون یا کوئی اسپیکر، لاس لیس
  معیار میں جب Spotify ایپ میں وہ آن ہو۔
- **YouTube Music سے:** MixMusic Spotify ٹریک کی وہ ریکارڈنگ YouTube Music پر
  چلاتا ہے، جس کے لیے Premium کی ضرورت نہیں۔

Settings › Playback › Spotify tracks میں چنیں۔

### آواز

- **دس بینڈ کا ایکولائزر** 23 پری سیٹس اور آپ کے اپنے کے ساتھ، Bass، Mid اور
  Treble کا ایک سادہ موڈ، اور AutoEq کے 8,800 سے زیادہ ہیڈ فونز کی اصلاح۔
  *Settings › Playback › Equalizer*
- **کراس فیڈ، بغیر وقفے کے البمز، یکساں بلندی اور والیوم لیولنگ۔**
  *Settings › Playback › Sound*
- **ریورب، ایکو، کراس فیڈ، بیلنس اور اسٹیریو چوڑائی۔** *Settings › Playback*
- **5.1 سراؤنڈ** جہاں ٹریک میں موجود ہو، اور آپ کی چنی ہوئی آؤٹ پٹ ڈیوائس۔
  *پلیئر بار، Settings › Playback › Sound*
- **ایک ویژولائزر،** جو MixMusic کے اپنے آڈیو انجن سے بنتا ہے۔ *اب چل رہا ہے*

### کاسٹنگ، دوست اور دوسرے کمپیوٹر

- **کاسٹ کریں** Chromecasts، Google Home اور Nest اسپیکرز اور ان کے گروپس،
  DLNA اور UPnP رینڈررز اور FCast ریسیورز پر، آپ کی اپنی موسیقی کی فائلیں بھی۔
  *پلیئر بار، منی پلیئر*
- **دوستوں کے ساتھ سنیں:** ایک روم چلائیں جو آپ کے دوستوں کے کمپیوٹرز پر
  ایک ساتھ چلتا ہے، آپ کے نیٹ ورک پر یا دعوتی لنک کے ذریعے کہیں سے بھی۔
  *پلیئر بار کا کاسٹ مینو*
- **ریموٹ کنٹرول:** اپنے نیٹ ورک کے کسی دوسرے کمپیوٹر پر MixMusic کو کنٹرول
  کریں، اور جو چل رہا ہے اسے ان کے درمیان منتقل کریں۔ *پلیئر بار کا کاسٹ
  مینو، Settings › Desktop*
- **AI اسسٹنٹس** آپ کے کمپیوٹر پر بتا سکتے ہیں کہ کیا چل رہا ہے، سرچ کر
  سکتے ہیں، چلا سکتے ہیں اور سلیپ ٹائمر لگا سکتے ہیں، Model Context Protocol
  کے ذریعے۔ *Settings › Desktop › AI assistants*

### آپ کے ڈیسک ٹاپ پر

- **ایک منی پلیئر** جو آپ کی دوسری ونڈوز کے اوپر رہتا ہے، اپنے پینل میں قطار
  اور بولوں کے ساتھ۔ *پلیئر بار، <kbd>Ctrl</kbd> <kbd>M</kbd>*
- **میڈیا کیز** اور Windows، macOS اور Linux پر سسٹم کے میڈیا کنٹرولز۔
  *پہلے سے شامل*
- **اپنے گلوبل شارٹ کٹس:** چلانے، روکنے، اسکِپ کرنے، والیوم بدلنے، خاموش کرنے،
  جو چل رہا ہے اسے لائک یا ڈس لائک کرنے، یا MixMusic دکھانے کے لیے، چاہے
  کوئی بھی ونڈو سامنے ہو۔ *Settings › Shortcuts*
- **پلیئر کے بٹن** Windows پر ٹاسک بار کے تھمب نیل میں، اور MacBook کے نوچ کے
  گرد ایک پلیئر۔ *پہلے سے شامل، Settings › Desktop*
- **ٹرے کنٹرولز،** نئے ٹریکس کی اطلاعات، ونڈو کو اوپر رکھنا اور لاگ اِن پر
  شروع ہونا۔ *Settings › Desktop*
- **`mixmusic://` لنکس** اور `--next` جیسے فلیگز، Stream Deck کی کسی کلید یا
  اسکرپٹ کے لیے۔ *کمانڈ لائن*
- **اوورلیز کے لیے اب چل رہا ہے:** OBS کے براؤزر سورس کے لیے جو چل رہا ہے اس کا
  کارڈ، اور آپ کے اپنے صفحے کے لیے اس کا ڈیٹا۔ *Settings › Desktop*

### اسے اپنا بنائیں

- **لے آؤٹس۔** Blend، جو ڈیفالٹ ہے، بڑے کورز کی گھنی قطاروں پر گول شکلیں اور
  لچکدار حرکت رکھتا ہے۔ Expressive، جو Material 3 Expressive کے طرز پر ہے،
  شوخ ہے، ٹونل فلز، پِل بٹنز اور تیرتے پلیئر بار کے ساتھ۔ Crate سادہ ہے، تاکہ
  کورز ہی کام کریں، اور Tiles MixMusic کی پہلی شکل ہے، سپاٹ ٹائلز اور بڑے
  چھوٹے حروف (lowercase) کے الفاظ کے ساتھ۔ *Settings › Appearance*
- **تھیمز۔** چودہ پہلے سے شامل ہیں، ہر ایک کسی بھی لے آؤٹ میں، ایسے ایفیکٹس
  کے ساتھ جنھیں آپ ٹیون کر سکتے ہیں، لائٹ موڈ کے لیے ایک تھیم اور ڈارک موڈ
  کے لیے ایک، اور اپنی پسند کا یا چلتے ٹریک کے آرٹ ورک سے لیا گیا ایکسنٹ
  رنگ۔ Material اور Material Dark ہر رنگ کور سے لیتے ہیں۔
  *Settings › Appearance*
- **کمیونٹی تھیمز اور پلگ اِنز،** مارکیٹ پلیس میں ایک کلک کی دوری پر۔
  *مارکیٹ پلیس*
- **پلگ اِنز** ایک ٹائپڈ API کے ذریعے صفحات، کمانڈز، ٹریک ایکشنز اور ویجٹس
  شامل کرتے ہیں، جو آپ کے تھیم میں بنتے ہیں۔ *Settings › Plugins*
- **کی بورڈ پہلے۔** ہر عام کام کے لیے شارٹ کٹس، ہر کمانڈ کے لیے آپ کی اپنی
  کلید، پلگ اِنز کی کمانڈز سمیت، ایسی کیز جو کسی بھی ایپ سے موسیقی کو
  کنٹرول کریں، اور باقی کے لیے ایک کمانڈ پیلیٹ (<kbd>Ctrl</kbd> <kbd>K</kbd>)۔
- **آپ کی زبان۔** انگریزی کے علاوہ 66 زبانیں، دائیں سے بائیں جہاں زبان اسی
  طرح پڑھی جاتی ہو، اور تاریخیں اور اعداد ویسے جیسے آپ کا علاقہ لکھتا ہے۔
  *Settings › Appearance › Language*
- **اسکرین ریڈرز اور کی بورڈ کے لیے۔** اسکرین ریڈرز کے لیے نام اور اعلانات،
  ہر کنٹرول کی بورڈ کی پہنچ میں، متن کا سائز، زوم اور کم حرکت۔
  *پہلے سے شامل، Settings › Appearance*
- **ایک سیٹنگز فائل۔** آپ کی سیٹنگز آپ کے ڈیٹا فولڈر میں `settings.json` ہیں
  (Windows پر `%AppData%\MixMusic`، macOS پر
  `~/Library/Application Support/MixMusic`، Linux پر `~/.config/MixMusic`)،
  جسے آپ اپنی dotfiles کے ساتھ رکھ سکتے ہیں۔ MixMusic کے چلتے ہوئے اسے محفوظ
  کریں تو وہ فوراً لاگو ہو جاتی ہے۔ *Settings › Desktop › Settings file*

### بیک اپ، امپورٹ اور ایکسپورٹ

- **بیک اپ اور بحالی** اپنی لائبریری، سیٹنگز اور ایڈ آنز کی، ہاتھ سے یا خود
  بخود، اس کمپیوٹر پر یا کسی دوسرے پر۔ *Settings › Desktop › Backups*
- **پلے لسٹس امپورٹ کریں** فائلوں سے اور اپنے Spotify اور Google ڈیٹا سے، اور
  فہرستیں اور اپنی لائبریری ایسی فائلوں میں ایکسپورٹ کریں جو دوسرے پلیئرز
  پڑھتے ہیں۔ *لائبریری › مکسز، صفحے کا More مینو*

### اپڈیٹس

- MixMusic شروع ہونے پر اور ہر چند گھنٹوں بعد نئی ریلیز دیکھتا ہے، اس کے
  دستخط جانچتا ہے، اور آپ کے کہنے پر اسے انسٹال کرتا ہے، یا اگر آپ یہ چنیں
  تو خود ہی۔ *Settings › About*

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

## دستاویزات

گائیڈز ویب سائٹ پر ہیں،
[mixmusic.functionx.dev/docs](https://mixmusic.functionx.dev/docs/) پر، اور
[fzn0x.github.io/mixmusic/docs](https://fzn0x.github.io/mixmusic/docs/) پر بھی،
جہاں سرچ ان میں سے کوئی بھی ڈھونڈ دیتی ہے، اور یہاں [`docs/`](docs) میں بھی۔

- **شروعات:** [شروعات](docs/getting-started.md)،
  [خصوصیات](docs/features.md)، [اکاؤنٹس](docs/accounts.md)،
  [Spotify](docs/spotify.md)، [مسائل کا حل](docs/troubleshooting.md)
- **MixMusic کا استعمال:** [موسیقی چلانا](docs/playing-music.md)،
  [ہوم](docs/home.md)، [آپ کی لائبریری](docs/library.md)،
  [پلے لسٹس امپورٹ اور ایکسپورٹ](docs/import-export.md)،
  [اس کمپیوٹر پر موسیقی](docs/local-music.md)، [بول](docs/lyrics.md)،
  [پوڈ کاسٹس](docs/podcasts.md)، [شکل و صورت](docs/look-and-feel.md)،
  [ڈیسک ٹاپ پر](docs/desktop.md)،
  [اسٹریمز کے لیے اوورلیز](docs/stream-overlay.md)،
  [کاسٹنگ](docs/casting.md)، [ریموٹ کنٹرول](docs/remote-control.md)،
  [دوستوں کے ساتھ سنیں](docs/listen-with-friends.md)،
  [AI اسسٹنٹس](docs/ai-assistants.md)،
  [مارکیٹ پلیس](docs/marketplace.md)، [سیٹنگز](docs/settings.md)
- **یہ کیسے کام کرتا ہے:** [MixMusic کیسے کام کرتا ہے](docs/how-it-works.md)،
  [آڈیو انجن](docs/audio-engine.md)،
  [آڈیو اسٹریمز کہاں سے آتی ہیں](docs/streams.md)،
  [Spotify کے ٹریکس کیسے چلتے ہیں](docs/spotify-playback.md)،
  [آپ کا ڈیٹا اور پرائیویسی](docs/data-and-privacy.md)
- **اپنا بنائیں:** [تھیمز](docs/themes.md)، [پلگ اِنز](docs/plugins.md)
- **پروجیکٹ:** [کریڈٹس](docs/credits.md)، [لائسنس](docs/license.md)،
  [ڈھانچہ](docs/architecture/README.md)،
  [تعاون](CONTRIBUTING.md)، [ضابطہ اخلاق](CODE_OF_CONDUCT.md)،
  [سیکیورٹی](SECURITY.md)

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
`….delta`، `mixmusic-<version>-darwin-universal.tar.gz` یا
`-windows-<arch>.tar.gz` نام کی فائلوں کی ضرورت نہیں: انسٹال شدہ ایپس انھی
سے اپڈیٹ ہوتی ہیں۔

انسٹالرز، پورٹیبل zip اور Linux آرکائیو خود کو اپڈیٹ کر لیتے ہیں۔ `.deb`،
`.rpm`، AppImage اور Flatpak اپنی فائلیں خود نہیں بدل سکتے، اس لیے نیا ورژن
آنے پر MixMusic آپ کو بتاتا ہے اور اسے آپ کے لیے ڈاؤن لوڈ کر دیتا ہے۔ AUR کا
`mixmusic-bin` آپ کے باقی سسٹم کے ساتھ اپڈیٹ ہوتا ہے۔ پورٹیبل zip کو کہیں بھی
کھولیں اور `MixMusic.exe` چلائیں۔ AppImage کو executable بنائیں (`chmod +x`)
اور چلائیں۔ Flatpak کو اپنے سافٹ ویئر سینٹر سے کھولیں، یا
`flatpak install ./mixmusic-<version>-linux-<arch>.flatpak` چلائیں؛ پہلی بار
Flatpak، Flathub سے GNOME runtime لاتا ہے۔

Mac پر [Homebrew](https://brew.sh)
بھی MixMusic انسٹال کرتا ہے، اور `brew upgrade` اسے اپڈیٹ کرتا ہے:

</div>

```sh
brew install --cask fzn0x/tap/mixmusic
```

<div dir="rtl">

[شروعات](docs/getting-started.md) ہر سسٹم پر انسٹال اور پہلی بار چلانے کا
طریقہ بتاتی ہے۔

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

  MixMusic کے ساتھ چینی، جاپانی اور کورین حروف کا کوئی فونٹ نہیں آتا، اور
  Linux پر جس حرف کا فونٹ انسٹال نہ ہو وہ خالی جگہ کی طرح نظر آتا ہے۔ `.deb`
  اور `.rpm` ایک فونٹ مانگ لیتے ہیں؛ باقی کے ساتھ، خود ایک انسٹال کریں اور
  MixMusic دوبارہ شروع کریں:

  <div dir="ltr">

  ```sh
  # Debian and Ubuntu
  sudo apt install fonts-noto-cjk
  # Fedora
  sudo dnf install google-noto-sans-cjk-vf-fonts
  # Arch
  sudo pacman -S noto-fonts-cjk
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
ڈسپلے اور GPU نظر آتے ہیں، اور آپ کے ہوم فولڈر میں سے کچھ نہیں، سوائے ان
براؤزرز کی پروفائلز کے جو Flathub سے انسٹال ہوں، صرف پڑھنے کے لیے، جن سے
YouTube Music کا سائن اِن امپورٹ کیا جا سکتا ہے۔ یہ x64 اور Arm64 دونوں کے
لیے آتا ہے، جیسے Raspberry Pi 5، Asahi Linux اور Snapdragon لیپ ٹاپس۔ MixMusic
فی الحال Flathub پر نہیں ہے: ریلیز کا `.flatpak` انسٹال کریں۔

Flatpak اپنی سیٹنگز اور لائبریری `~/.var/app/io.github.fzn0x.mixmusic` میں
رکھتا ہے، جو `.deb`، `.rpm`، AppImage یا آرکائیو والی کاپی سے الگ ہے، اور
دونوں ایک ساتھ چل سکتی ہیں۔ کسی کاپی کی سیٹنگز اور لائبریری ساتھ لانے کے لیے
اس کا Settings › Desktop › Backups میں بیک اپ لیں اور وہ فائل Flatpak میں بحال
کریں۔ فائلیں ہاتھ سے کاپی کرنی ہوں تو دونوں بند کریں، پھر
`settings.json`، `library.json`، `plays.jsonl`، `local-copies.json`،
`queue.json`، `positions.json` اور `addons` و `mix-covers` فولڈرز کو
`~/.config/MixMusic` سے
`~/.var/app/io.github.fzn0x.mixmusic/config/MixMusic` میں کاپی کریں۔ پہلے
دونوں بند کریں: چلتا ہوا MixMusic `library.json` اور زیادہ تر دوسری فائلیں
میموری میں رکھتا ہے اور اگلی بار محفوظ کرتے وقت کاپی کی گئی فائل کی جگہ
اپنی فائل لکھ دیتا ہے۔ پھر اپنے اکاؤنٹس میں دوبارہ سائن اِن کریں: سائن اِنز
دوسری کاپی کی keyring میں رہتے ہیں۔
[ڈیٹا اور پرائیویسی کا صفحہ](docs/data-and-privacy.md#the-data-folder) بتاتا
ہے کہ ہر فائل میں کیا ہوتا ہے۔

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
  مٹا دیتا ہے؛ اس براؤزر کا سیشن امپورٹ کریں جس میں آپ سائن اِن ہیں
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
- **ListenBrainz:** اپنی [ListenBrainz ترتیبات](https://listenbrainz.org/settings/)
  کا، یا اپنے سرور کا، user token Settings › Accounts میں پیسٹ کریں۔
- **Libre.fm:** Settings › Accounts میں جوڑیں، اور اپنے براؤزر میں Libre.fm کے
  صفحے پر، یا کسی دوسرے GNU FM سرور کے صفحے پر، MixMusic کو منظوری دیں۔
- **Discord:** جب تک Discord ایپ چل رہی ہو، آپ کی پروفائل دکھاتی ہے کہ آپ کیا
  سن رہے ہیں۔ اسے Settings › Desktop (ترتیبات › ڈیسک ٹاپ) میں بند کریں۔

[اکاؤنٹس](docs/accounts.md) ہر سائن اِن قدم بہ قدم بتاتا ہے۔

### معلوم حدود

- **Arm پر Windows** کی جانچ x64 سے کم ہوئی ہے، اور پورٹیبل zip
  `mixmusic://` لنکس رجسٹر نہیں کرتا۔
- **ڈیولپمنٹ موڈ میں Spotify ایپس،** جیسی وہ جس سے آپ سائن اِن کرتے ہیں، ہر
  قسم کے دس سرچ نتائج دیتی ہیں، اور وہ پلے لسٹس جو آپ کی نہیں ہیں ان کے
  ٹریکس چھپے رہتے ہیں۔
- **Linux پر،** Spotify بطور ڈیفالٹ Spotify Connect کے ذریعے چلتا ہے، کیونکہ
  WebKitGTK، Spotify کی محفوظ آڈیو نہیں چلا سکتا۔
- **YouTube** اپنے انٹرفیسز بدل سکتا ہے اور سرچ، پلے بیک یا سائن اِن کو توڑ
  سکتا ہے، جب تک MixMusic یا yt-dlp اس کا ساتھ نہ پکڑ لے۔

کچھ کام نہیں کر رہا؟ [مسائل کا حل](docs/troubleshooting.md) عام مسائل اور
معلوم خامیوں کا احاطہ کرتا ہے۔

## سورس سے بنائیں

آپ کو [Go](https://go.dev/dl/) 1.27.3 یا اس سے نیا، [Bun](https://bun.sh) 1.4
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
ریلیزز بنتی ہیں، انھی ورژنز پر جو `flake.lock` طے کرتا ہے: Go 1.27، Bun،
GoReleaser، jq، zip اور Zola، اور Linux پر NSIS، osslsigncode اور binutils۔
`nix build` MixMusic کو Linux کے لیے سورس سے `result/` میں بناتا ہے، اور
`nix run github:fzn0x/mixmusic` اسے چلاتا ہے۔

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

[ویب سائٹ](https://mixmusic.functionx.dev/) `site/` میں ہے، جو
[Zola](https://www.getzola.org) سے بنتی ہے، اور `docs/` کی گائیڈز اس کے `/docs/`
صفحات ہیں۔ `go run ./tools/sitedocs && zola --root site serve` اس کا پیش نظارہ
دکھاتا ہے۔ ریلیزز Nix شیل میں بنتی ہیں اور
[GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`) سے شائع ہوتی ہیں۔

[CONTRIBUTING.md](CONTRIBUTING.md) میں وہ جانچیں ہیں جو pull request سے پہلے
چلانی ہیں، اور [docs/architecture](docs/architecture/README.md) بتاتا ہے کہ
حصے آپس میں کیسے جڑتے ہیں، ہر حصے کے لیے ایک صفحہ۔

### ساخت

ایپ روٹ پیکیج ہے، اور اس کے نیچے `internal/` کے پیکیجز ہیں۔ `internal/` سے
باہر کے پیکیجز ہر کسی کے امپورٹ کرنے کے لیے ہیں، اور
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic) پر دستاویزی ہیں۔

</div>

```
*.go                 the app and its windows: the pages, the player bar, the
                     mini player, the tray, media keys, deep links
fonts/               the typefaces the windows draw with
locales/             the app's words, in English and 66 other languages
internal/app         the services the windows call, and the desktop integration
internal/player      the queue, played by the audio engine or on Spotify's devices
internal/audio       MixMusic's audio engine, and the system's audio output
internal/stream      audio streams, from YouTube or yt-dlp, opened for the engine
internal/youtube     YouTube Music's catalog, and your account there
internal/spotify     the Spotify Web API, its sign-in and Spotify Connect
internal/match       Spotify tracks matched to their YouTube Music recordings
internal/library     liked and disliked songs, mixes, history and stats
internal/localfiles  the music on this computer; tags and tagwrite, its tags
internal/romaji      Japanese lyrics in romaji, kanji read by kagome;
                     romanize, the Latin letters of other scripts
internal/cast        casting to Chromecasts; upnp, to DLNA renderers; fcast,
                     to FCast receivers; dnssd finds them on the network
internal/room        Listen with friends; remote, remote control; portmap,
                     the router's forwarded port
internal/mcp         AI assistants, over the Model Context Protocol
internal/lastfm      scrobbling to Last.fm; listenbrainz and librefm, to
                     theirs; scrobble, which plays count
internal/discord     what plays, on your Discord profile
internal/plugins     plugins, each in a JavaScript engine of its own
internal/addons      themes and plugins, and the community registry
internal/backup      backups and restores; playlistio and export, the files
                     of lists and tracks in and out
internal/diag        the log, without secrets, and the reports of crashes
internal/settings    your preferences and their defaults
builtin/themes       the themes MixMusic ships with
public/              the pages MyGo embeds: Spotify's player, on Windows and
                     macOS, and the picture of music videos
sdk/                 the types of the plugin API
community/           community themes and plugins, and their registry
docs/                the guides, the website's /docs/ pages
site/                the website, built with Zola
tools/               the commands that build and check the repository's parts
packaging/, nix/     the Flatpak, Flathub, AUR and Homebrew packages, and Nix
changelog.d/         the changelog entries of what isn't released yet
```

<div dir="rtl">

[docs/architecture](docs/architecture/README.md) `internal/` کے باقی پیکیجز کے
بارے میں بتاتا ہے، ایپ کے ہر حصے کے لیے ایک صفحہ۔

جو پیکیجز آپ امپورٹ کر سکتے ہیں:

</div>

```
opus/                an Opus decoder in pure Go, a port of libopus's
aac/                 an AAC-LC decoder in pure Go
ac3/                 an AC-3 and E-AC-3 decoder in pure Go, written from ATSC A/52
flac/                a FLAC decoder in pure Go, as RFC 9639 has it
ogg/                 the packets of Ogg files, as RFC 3533 has them
webm/                the audio of WebM files: packets, times and seeking
mp4/                 the audio of MP4 files: packets, times and seeking
resample/            sample rate conversion, with Kaiser-windowed sincs
lyrics/              synced lyrics from Better Lyrics, Unison and LRCLIB, and translations
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
2. ایک فولڈر شامل کریں: تھیم کے لیے `community/themes/<id>/` اس کی
   `theme.json` کے ساتھ، یا پلگ اِن کے لیے `community/plugins/<id>/` اس کی
   `plugin.json` اور اس کی `index.ts` کے ساتھ۔
3. `bun install`، `bun run build:plugins`، `go run ./tools/registry` اور
   `go run ./tools/registry -check` چلائیں۔
4. ایک pull request کھولیں، ایک اسکرین شاٹ کے ساتھ۔

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
  چلاتے ہیں، جو آپ کی YouTube Music ہسٹری میں شامل ہوتے ہیں۔ خود YouTube،
  سائن آؤٹ حالت میں اور بغیر کوکیز کے، سرچ کے YouTube والے انتخاب، AI سے بنے
  گانوں کی جانچ، بولوں اور ٹرانسکرپٹس کے طور پر کیپشنز، اور میوزک ویڈیوز سے
  ہم وقت بولوں کے لیے۔
- **Spotify،** سائن اِن کرنے کے بعد: آپ کی لائبریری، آپ کے Spotify Connect
  ڈیوائسز، اور Spotify کا پلیئر، جو `sdk.scdn.co` سے لوڈ ہوتا ہے، جب MixMusic
  میں Spotify چلتا ہے۔
- **Better Lyrics:** چلتے ٹریک کا عنوان، پہلا فنکار، البم اور دورانیہ، تاکہ
  اس کے بول ملیں، جب اس کا البم اور دورانیہ ہو۔
- **Unison:** چلتے ٹریک کی video ID، یا، اس کے بغیر، اس کا عنوان، فنکار،
  البم اور دورانیہ، تاکہ اس کے بول ملیں۔ صرف تب جب آپ بولوں کے ساتھ ترجمہ
  (translate) یا عربی کو رومن (romanize) آن کریں: چلتے ٹریک کے بول، اس کی
  video ID کے ساتھ، جنھیں Unison، Google سے ترجمہ یا رومن کرواتا ہے۔
- **LRCLIB:** چلتے ٹریک کا عنوان، فنکار، البم اور دورانیہ، تاکہ اس کے بول ملیں۔
- **SponsorBlock:** صرف تب جب آپ Skip the parts SponsorBlock marks
  (SponsorBlock کے نشان زدہ حصے چھوڑیں) آن کریں: ہر video ID کے hash کے پہلے
  4 حروف، ID کبھی نہیں، تاکہ اس کے نشان زدہ حصے ملیں۔
- **GitHub:** شروع ہونے پر اور ہر چند گھنٹوں بعد نئے ورژن کی جانچ، جسے آپ
  Settings › About (ترتیبات › تعارف) میں بند کر سکتے ہیں؛ مارکیٹ پلیس کھولنے پر
  کمیونٹی registry؛ yt-dlp کے ڈاؤن لوڈز؛ AutoEq کی ہیڈ فونز کی فہرست اور وہ
  پروفائل جو آپ چنیں؛ اور، جاپانی بولوں کے ساتھ romaji پہلی بار آن کرنے پر،
  kanji کی ریڈنگز کی ڈکشنری (11.7 MB)۔ Romaji، اور عربی کے سوا دوسری رسم
  الخطوں کے لاطینی حروف، آپ کے کمپیوٹر پر بنتے ہیں: بول بھیجے نہیں جاتے۔
- **Last.fm،** اسے جوڑنے کے بعد: جو آپ چلاتے ہیں۔
- **ListenBrainz،** یا اس کی API والا آپ کا اپنا سرور، اسے جوڑنے کے بعد: جو
  آپ چلاتے ہیں۔
- **Libre.fm،** یا کوئی دوسرا GNU FM سرور، اسے جوڑنے کے بعد: جو آپ چلاتے ہیں۔
- **Discord،** جب تک آپ اسے بند نہ کریں: جو چل رہا ہے، آپ کے کمپیوٹر پر
  Discord ایپ کو بھیجا جاتا ہے، جو اسے آپ کی پروفائل پر دکھاتی ہے۔
- **وہ ڈیوائسز جن پر آپ کاسٹ کرتے ہیں،** آپ کے نیٹ ورک پر: جو چل رہا ہے اس کا
  عنوان، فنکار اور کور۔ وہ آڈیو MixMusic سے لیتی ہیں۔
- **MixMusic والے دوسرے کمپیوٹرز،** ریموٹ کنٹرول اور دوستوں کے ساتھ سننے کے
  لیے، انکرپٹڈ، آپ کے نیٹ ورک پر یا، جب آپ باہر کے دوستوں کو شامل ہونے دیں،
  انٹرنیٹ پر۔ تب آپ کے راؤٹر سے ایک پورٹ فارورڈ کرنے کو کہا جاتا ہے، اور
  آپ کے دعوتی لنک میں آپ کا انٹرنیٹ ایڈریس ہوتا ہے۔
- **AI اسسٹنٹس** آپ کے کمپیوٹر پر، جب تک آپ انھیں اجازت دیں: جو چل رہا ہے،
  قطار، ان کی سرچز کے نتائج اور آپ کے پلے لسٹس کے نام، جنھیں اسسٹنٹ اپنے
  استعمال کیے گئے AI ماڈل کو آگے بھیج سکتا ہے۔
- **YouTube اور Spotify،** یہ جاننے کے لیے کہ نیٹ ورک جواب دیتا ہے یا نہیں:
  HTTPS پر ایک ایسی درخواست جس کا جواب خالی ہوتا ہے۔

پلگ اِنز جو چاہیں بھیج سکتے ہیں: پلگ اِن کی اجازتیں آپ کو دکھائی جاتی ہیں،
نافذ نہیں کی جاتیں، اسی لیے MixMusic پلگ اِن کو فعال کرنے سے پہلے آپ سے اس پر
بھروسا کرنے کو کہتا ہے۔
[آپ کا ڈیٹا اور پرائیویسی](docs/data-and-privacy.md#what-each-feature-sends-where)
میں ایک جدول ہے کہ ہر خصوصیت کیا کہاں بھیجتی ہے۔

جو کچھ یہ آپ کے کمپیوٹر پر رکھتا ہے:

- آپ کی سیٹنگز اور لائبریری (پسندیدہ اور ناپسندیدہ گانے، مکسز، ہر وہ پلے جو
  آپ کے اعداد و شمار گنتے ہیں، فالو کیے گئے فنکار اور شوز، پنز، فولڈرز اور
  ٹیگز، ہسٹری، حالیہ سرچز، اور آپ کی موسیقی کی فائلوں کا انڈیکس)، اس فولڈر
  میں جس کا نام Settings › About بتاتا ہے۔
- آپ کے چلائے ہوئے ٹریکس، کیش میں، اتنے جتنے آپ Settings › Playback
  (ترتیبات › پلے بیک) میں اجازت دیں، اور یہ کہ YouTube نے انھیں کتنا بلند
  ناپا۔ آپ کے ڈاؤن لوڈ کیے ہوئے گانے، ڈاؤن لوڈز فولڈر میں۔
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
ہے۔ [صاف کریں، منتقل کریں یا ری سیٹ کریں](docs/data-and-privacy.md#clear-move-or-reset)
بتاتا ہے کہ اپنا ڈیٹا کیسے منتقل کریں، `MIXMUSIC_DATA_DIR` سے پورٹیبل کاپی کیسے
رکھیں، یا نئے سرے سے کیسے شروع کریں۔

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
- **[SponsorBlock](https://sponsor.ajay.app)،** YouTube ویڈیوز کے حصوں کا کھلا،
  عوامی تعاون سے بنا ڈیٹا بیس، Ajay Ramachandran اور اس کے شراکت داروں کی طرف
  سے، جس کے حصے MixMusic اس وقت چھوڑتا ہے جب آپ اسے کہیں۔ Uses
  SponsorBlock data licensed under
  [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/)
  from https://sponsor.ajay.app/.
- **[Better Lyrics](https://betterlyrics.org)،** ایک سروس جو بول لفظ بہ لفظ ہم وقت رکھتی ہے۔
- **[Unison](https://unison.betterlyrics.org)،** بولوں کا کھلا، عوامی تعاون سے
  بنا ڈیٹا بیس، Unison Community License کے تحت، اور **[LRCLIB](https://lrclib.net)،** ہم وقت
  بولوں کا کھلا ڈیٹا بیس۔
- **[kagome](https://github.com/ikawaha/kagome)،** جاپانی morphological
  analyser جو بولوں کے kanji کو ان کے romaji کے لیے پڑھتا ہے، اور
  [kagome-dict](https://github.com/ikawaha/kagome-dict) کی IPA ڈکشنری کے
  ساتھ، جو mecab-ipadic سے بنی ہے، اور جسے MixMusic پہلی بار romaji آن ہونے
  پر ڈاؤن لوڈ کرتا ہے۔
- **[libopus](https://opus-codec.org)،** جس کے ڈیکوڈر کا Opus ڈیکوڈر پورٹ ہے،
  اور **[SharpJaad](https://github.com/jimm98y/SharpJaad)،** [JAAD](https://sourceforge.net/projects/jaadec/)
  کا C# پورٹ جس سے AAC ڈیکوڈر اخذ کیا گیا ہے۔
- **[Chromium](https://www.chromium.org)** کا Web Audio، جس کے biquad،
  DynamicsCompressor اور AnalyserNode کے ایکولائزر کے فلٹرز، والیوم لیولنگ اور
  ویژولائزر پورٹ ہیں۔
- **[AutoEq](https://github.com/jaakkopasanen/AutoEq)،** Jaakko Pasanen اور اس
  کے شراکت داروں کی طرف سے، جس کے پروفائلز ہیڈ فونز کی آواز درست کرتے ہیں،
  [Equalizer APO](https://sourceforge.net/projects/equalizerapo/) کے فارمیٹ
  میں۔
- **[go-mp3](https://github.com/hajimehoshi/go-mp3)** اور
  **[jfreymuth/vorbis](https://github.com/jfreymuth/vorbis)،** جو آپ کے
  کمپیوٹر پر MP3 اور Vorbis فائلیں ڈیکوڈ کرتے ہیں، اور
  **[Xiph.Org](https://xiph.org)،** جس کے FLAC اور Ogg کو `flac` اور `ogg`
  پڑھتے ہیں۔
- **[FFmpeg](https://ffmpeg.org)،** جس سے ڈیکوڈرز کے ٹیسٹ موازنہ کرتے ہیں۔
- **[Selawik](https://github.com/microsoft/Selawik)،**
  **[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily)،**
  **[Roboto Flex](https://github.com/googlefonts/roboto-flex)** اور
  **[Archivo](https://github.com/Omnibus-Type/Archivo)،** ونڈوز کے ٹائپ فیسز۔
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities)،**
  جس کی HCT رنگ سائنس کا Material تھیمز کے رنگ پورٹ ہیں۔
- **[FCast](https://docs.fcast.org)،** وہ کھلا پروٹوکول جس سے MixMusic FCast
  ریسیورز پر چلاتا ہے۔ MixMusic اس کا ایک آزاد نفاذ ہے، اور FUTO سے وابستہ نہیں؛
  FCast FUTO کا رجسٹرڈ ٹریڈ مارک ہے۔
- **[Kevin MacLeod](https://incompetech.com)،** جن کی موسیقی،
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) کے تحت، ویب سائٹ
  پر چلتی ہے۔

[کریڈٹس](docs/credits.md) ہر اس پروجیکٹ، مقالے اور معیار کا نام لیتا ہے جن پر
MixMusic بنا ہے۔

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
- **سیکیورٹی کی کمزوری کی اطلاع** نجی طور پر دیں، جیسا کہ
  [SECURITY.md](SECURITY.md) بتاتی ہے۔

حصہ لینے والا ہر شخص [ضابطہ اخلاق](CODE_OF_CONDUCT.md) کی پابندی کرتا ہے۔
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
دوسرے حصے ان کے لائسنس رکھتے ہیں جن سے وہ اخذ کیے گئے ہیں، جیسے Chromium کے
Web Audio کے فلٹرز، go-mp3، jfreymuth/vorbis، kagome اور Unihan کا ڈیٹا:
[لائسنس](docs/license.md) ہر ایک کی فہرست دیتا ہے۔

MixMusic کا Spotify یا Google سے کوئی الحاق نہیں، نہ انھوں نے اسے تصدیق کیا ہے
اور نہ ہی یہ ان کا اسپانسر کردہ ہے۔ یہ وہ چلاتا ہے جو یہ سروسز اپنے صارفین کو
پیش کرتی ہیں، ان سروسز کے اپنے انٹرفیسز کے ذریعے؛ اسے ان کی شرائط کی اجازت کے
مطابق استعمال کریں۔ Spotify اور YouTube Music اپنے مالکان کے ٹریڈ مارکس ہیں۔

</div>
