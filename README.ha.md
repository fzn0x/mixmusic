<p align="center"><a href="README.md">English</a> · <a href="README.hi.md">हिन्दी</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ur.md">اردو</a> · <strong>Hausa</strong> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.bn.md">বাংলা</a> · <a href="README.ru.md">Русский</a> · <a href="README.am.md">አማርኛ</a></p>

<p align="center">
  <img src="brand/icon.png" width="112" height="112" alt="Alamar MixMusic: zobba biyu masu launin hoda da maɓallin kunnawa a kan baƙin fage" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center"><strong>Mai kunnawa guda ɗaya don YouTube Music da Spotify.</strong></p>

<p align="center">Manhajar YouTube Music da Spotify don macOS, Windows da Linux. YouTube Music yana aiki ba tare da asusu ba. Bincike ɗaya, jerin gaba ɗaya, playlist ɗin da aka haɗa, kalmomin waƙa masu daidaita da waƙar, podcast, fayilolin kiɗanka na kanka, kunna a wata naʼura, jigogi da plugin.</p>

<p align="center"><sub>Wannan fassara ce ta <a href="README.md">README na Turanci</a>. Idan akwai bambanci, na Turanci shi ne mafi sabo.</sub></p>

<p align="center">
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml/badge.svg" alt="Shafin yanar gizo" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases/latest"><img src="https://img.shields.io/github/v/release/fzn0x/mixmusic" alt="Sabuwar fitarwa" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases"><img src="https://img.shields.io/github/downloads/fzn0x/mixmusic/total" alt="Zazzagewa" /></a>
  <a href="https://pkg.go.dev/github.com/fzn0x/mixmusic"><img src="https://pkg.go.dev/badge/github.com/fzn0x/mixmusic.svg" alt="Go Reference" /></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/fzn0x/mixmusic" alt="Sigar Go" /></a>
  <a href="https://awesomemygo.dev/apps/mixmusic"><img src="https://awesomemygo.dev/badge.svg" height="20" alt="An saka shi a Awesome MyGo" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey" alt="macOS, Windows da Linux" />
  <a href="#lasisi"><img src="https://img.shields.io/badge/license-MIT%20or%20GPL--3.0-blue" alt="MIT ko GPL-3.0" /></a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/313095?utm_source=trendshift-badge&amp;utm_medium=badge&amp;utm_campaign=badge-trendshift-313095" target="_blank" rel="noopener noreferrer"><img src="https://trendshift.io/api/badge/trendshift/repositories/313095/daily?language=Go" alt="fzn0x%2Fmixmusic | Trendshift" width="250" height="55"/></a>
</p>

<p align="center">
  <a href="https://mixmusic.functionx.dev/">Shafin yanar gizo</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">Zazzage</a> ·
  <a href="https://mixmusic.functionx.dev/docs/">Takardu</a> ·
  <a href="docs/features.md">Kowace siffa</a> ·
  <a href="CHANGELOG.md">Jerin canje-canje</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic a yanayinsa na asali, tsarin Blend tare da jigon Mix, a tsaye a kan wani fage mai duhu yana fuskantarka: a tsakiya ana kunna アイドル na YOASOBI, an zagaye shi da hoda, tare da romaji a ƙasan kowane layi na kalmomin waƙar da ƙaramin mai kunnawa a gabansa; a hagu akwai bincike, a dama kuma mix ɗin da ke cikin ɗakin waƙoƙi" />
</p>

## Dalilin MixMusic

Idan waƙoƙinka sun rabu tsakanin YouTube Music da Spotify, yawanci kana
buƙatar manhajoji biyu, da bincike biyu, jerin gaba biyu da ɗakunan waƙoƙi
biyu. MixMusic yana haɗa ayyukan biyu a cikin mai kunnawa guda ɗaya:

- **Bincike ɗaya.** Yana bincika YouTube Music da Spotify a lokaci guda, kuma
  yana alamta kowane sakamako da aikin da ya fito daga gare shi.
- **Jerin gaba ɗaya da ɗakin waƙoƙi ɗaya.** Waƙar Spotify na iya biyo bayan
  waƙar YouTube Music kai tsaye, kuma playlist ɗaya na iya ɗaukar waƙoƙi daga
  duka biyun, da kuma daga fayilolin kiɗan da ke kwamfutarka.
- **Ba ya buƙatar asusu.** YouTube Music yana kunnawa da zarar ka buɗe
  manhajar. Ka shiga ne kawai idan kana son ɗakin waƙoƙinka na kanka. Spotify
  na zaɓi ne.
- **Ƙaramin shiri.** MixMusic shiri ne guda ɗaya na Go wanda yake zana
  tagoginsa da kansa a kan GPU. Babu burauza a cikinsa da za a zazzage ko a
  riƙe a cikin ƙwaƙwalwa.
- **Naka ne ka sauya shi.** Yana zuwa da tsari huɗu da jigogi goma sha huɗu,
  alʼumma na ƙara jigogi da plugin, kuma kowane umarni na iya samun maɓalli
  naka na kanka.
- **A harshenka.** Taga tana zuwa da harsuna 66 baya ga Turanci, kuma ana
  shimfiɗa ta daga dama zuwa hagu don Larabci, Ibrananci, Farisanci da Urdu.
- **Sirri.** MixMusic ba shi da asusu, sabar, nazari ko telemetry na kansa.
- **Kyauta kuma open source,** a ƙarƙashin lasisin MIT ko GPL 3.0.

## Siffofi

Abin da MixMusic yake yi, fanni da fanni, da inda za ka same shi a cikin
manhajar. **Built in** (a ciki) yana nufin yana aiki da kansa.
[Siffofi](docs/features.md) yana lissafa duka siffofi 363 daga 0.1.0 zuwa
0.5.12, layi ɗaya kowacce, tare da sigar da ta shigo a cikinta.

### Bincike da kunnawa

- **Bincike ɗaya don ayyukan biyu.** Sakamakon YouTube Music da Spotify a
  gefe da gefe, kowanne da alamar aikinsa, tare da fayilolin kiɗanka na kanka.
  Sakamakon YouTube Music yana nuna sau nawa aka kunna kowace waƙa, don a
  rarrabe ta asali daga waɗanda aka sake rerawa. *Sandar sama*
- **Kowane irin sakamako.** Waƙoƙi, bidiyoyi, albam, mawaƙa, playlist da
  podcast, a jere bisa mafi dacewa, mafi yawan kunnawa ko mafi sabo, da
  YouTube kansa don waɗansu kide-kide kai tsaye da abubuwan da aka ɗora waɗanda
  kundin kiɗa ba shi da su. Hanyar YouTube ko ta Spotify da aka liƙa a
  akwatin bincike tana kunnawa nan take. *Bincike*
- **Jerin gaba ɗaya.** Haɗa waƙoƙin ayyukan biyu, ka gauraya ko ka maimaita a
  tsakaninsu, ka mayar da waƙa ta zama ta gaba, kuma ka bar radio ya ci gaba
  idan jerin gaba ya ƙare. Yana dawowa yadda ka bar shi bayan sake farawa.
  *Mashayin mai kunnawa, Jerin gaba*
- **Shafi ga kowane abu.** Albam, mawaƙa da playlist tare da Play da Shuffle,
  duka waƙoƙin mawaƙi, da waƙoƙin da suka shafi abin da ake kunnawa.
  *Bincike, Gida*
- **Tsallake abin da ba ka so.** Ƙi waƙoƙi don radio su wuce su, tsallake
  waƙoƙin da aka yi da AI da sassan bidiyo da SponsorBlock ya yi wa alama, da
  ɓoye abun da bai dace da yara ba ko fifita sigogi masu tsabta. *Mashayin
  mai kunnawa, Settings › Playback*
- **Mai ƙidayar barci da gudu.** Tsayawa bayan wani lokaci, bayan waƙa ɗaya ko
  bayan jerin gaba, da raguwar sauti a hankali, ko kai tsaye da dare, da
  kunna waƙoƙi da podcast da sauri ko a hankali da sautinsu na asali.
  *Mashayin mai kunnawa*
- **Bidiyon waƙa da murfuna masu motsi.** Bidiyon waƙa maimakon murfinta, da
  gajerun bidiyoyin da wasu albam suke maimaitawa. *Ana kunnawa,
  Settings › Playback*
- **Zaɓa, raba da dubawa sosai.** Zaɓi waƙoƙi da yawa don saka a jerin gaba,
  ƙarawa ko zazzagewa a lokaci guda, raba waƙa ko albam a matsayin katin hoto,
  ka kuma ga inda waƙa ta fito da dalilin da ya hana ta kunnawa. *Menu na
  waƙa*

### Gida da Explore

- **Gida** da shiryayyunka, Komawa daga inda ka tsaya da sabbin fitarwa na
  mawaƙan da kake bi, a shirya yadda kake so. *Gida*
- **Explore (Bincika):** sabbin fitarwa, jadawalin waƙoƙi, yanayi da nauʼoʼi.
  *Explore*

### Kalmomin waƙa

- **Kalmomin waƙa masu daidaita da waƙar** waɗanda suke bin waƙar, kalma bayan
  kalma inda tushen yake da hakan, har da cikakken allo, ko rubutu kawai idan
  ka fi so. *Allon kalmomin waƙa, Ana kunnawa*
- **Fassara da haruffan Latin:** fassara a ƙasan kowane layi, romaji don
  Japananci, da haruffan Latin don Koriyanci, Sinanci, Rashanci, Larabci da
  wasu rubutu da yawa. *Allon kalmomin waƙa*
- **Zaɓin tushenka:** Better Lyrics, Unison, LRCLIB, YouTube Music da
  rubutun YouTube, bisa tsarin da ka saita, tare da wani tushe, bincike,
  gyare-gyarenka da daidaita lokaci a danna ɗaya. *Settings › Playback ›
  Lyrics, Allon kalmomin waƙa*
- **Nema a cikin kalmomin waƙa,** raba layukansu a matsayin kati, da karanta
  kalmomin waƙa da fayilolin waƙoƙinka suke ɗauke da su. *Allon kalmomin waƙa*

### Podcast

- **Podcast na YouTube Music da Spotify.** Nemo ka bi shirye-shirye, fara kowane
  kashi daga inda ka tsaya, yi wa kashi alamar an saurare shi, karanta
  babuka da rubutun furucinsu, kuma ka ba su gudu na daban da na waƙoƙi.
  *Bincike, Shafin shiri, Ɗakin waƙoƙi › Podcast*

### Ɗakin waƙoƙinka

- **Mixes:** playlist ɗin da suke ɗaukar waƙoƙi daga kowane aiki, a tsarin da
  ka zaɓa, a bincika a kuma tsara, da murfuna naka na kanka. *Ɗakin waƙoƙi ›
  Mixes, Gefen allo*
- **Waƙoƙin da ka so** daga ayyukan biyu a jeri ɗaya, da playlist, albam da
  mawaƙan YouTube Music da Spotify ɗinka, waɗanda za ka iya adanawa, yi, raba
  da yin aiki tare a kansu. Abin da ka so ko ka adana yana kai wa aikin ma.
  *Ɗakin waƙoƙi*
- **Maƙalli, manyan fayiloli da alamomi** suna kiyaye tsarin babban ɗakin
  waƙoƙi, kuma abin da ba ka so a ciki za a iya ɓoye shi. *Gefen allo, Menu na
  tayal*
- **Bin mawaƙa,** da asusu ko ba shi, da ganin sabbin fitarwarsu. *Shafin
  mawaƙi, Ɗakin waƙoƙi › Mawaƙa*
- **Tarihi, Ƙididdiga da Shekararka:** mintuna a kowace rana, waƙoƙinka da
  mawaƙanka mafi yawa, da kati na su don raba. *Ɗakin waƙoƙi › Tarihi,
  Ƙididdiga*
- **Zazzagewa** don kunnawa ba tare da intanet ba, da maʼajiyar waƙoƙin da ka
  kunna. *Menu na waƙa, Ɗakin waƙoƙi › Zazzagewa*

### Kiɗa a wannan kwamfutar

- **Fayilolin kiɗanka na kanka** a cikin ɗakin waƙoƙi da bincike, daga manyan
  fayilolin da MixMusic yake sa ido a kansu: MP3, FLAC, WAV, M4A, Opus da
  Vorbis, tare da tags da murfunansu. *Settings › Playback › Music on this
  computer*
- **Gyara tags ɗinsu,** tara kwafin waƙa ɗaya, da buɗewa ko jefa fayiloli don
  kunna su. *Menu na waƙa, Jefa a kan taga*

### Asusu da ayyuka

- **YouTube Music.** Shiga a shafin Google na kansa, ta burauzarka ko da zaman
  burauza, a matsayin asusunka ko ɗaya daga cikin tashoshin alamar
  kasuwancinsa, don shafin gida, waƙoƙin da ka so, playlist, albam da
  mawaƙa. Abin da kake kunnawa yana shiga tarihin YouTube Music ɗinka, wanda
  shawarwarinsa suke fitowa daga gare shi. *Settings › Accounts*
- **Last.fm:** kowace waƙa da ka kunna har rabinta, ko na minti huɗu, tana
  shiga bayananka, kuma scrobbles da aka yi ba tare da intanet ba ana aika su
  idan ka dawo kan intanet. *Settings › Accounts*
- **ListenBrainz:** haka ma, tare da Last.fm ko ba tare da shi ba, zuwa
  ListenBrainz ko sabarka ta kanka, kamar Maloja. Waƙoƙin da ka so za su iya
  zuwa kowanne daga cikinsu a matsayin loves. *Settings › Accounts*
- **Libre.fm:** haka ma, zuwa Libre.fm ko wata sabar GNU FM, ba tare da ka
  ƙirƙiri asusun API ba. *Settings › Accounts*
- **Discord:** bayananka suna nuna abin da ake kunnawa, tare da hotonsa da
  lokacin da ya rage, yadda ka zaɓa. *Settings › Desktop*

### Spotify ta hanyoyi uku

- **A cikin MixMusic kansa,** da ingancin Spotify, ba tare da manhajar
  Spotify a buɗe ba (Premium, a Windows da macOS).
- **A Spotify Connect:** manhajar Spotify ɗinka, wayarka ko lasifika, a
  lossless idan manhajar Spotify tana da shi a kunne.
- **Daga YouTube Music:** MixMusic yana kunna rikodin waƙar Spotify a
  YouTube Music, wanda ba ya buƙatar Premium don kunnawa.

Zaɓi a Settings › Playback › Spotify tracks.

### Sauti

- **Equalizer mai madaurai goma** tare da saitunan da aka shirya 23 da naka na
  kanka, yanayi mai sauƙi na Bass, Mid da Treble, da gyaran belun kunne sama da
  8,800 daga AutoEq. *Settings › Playback › Equalizer*
- **Crossfade, albam marasa tazara, daidaita ƙarar waƙoƙi da daidaita ƙarar
  sauti.** *Settings › Playback › Sound*
- **Reverb, echo, crossfeed, daidaita hagu da dama da faɗin stereo.**
  *Settings › Playback*
- **Surround 5.1** inda waƙa take da shi, da naʼurar fitar da sauti da ka zaɓa.
  *Mashayin mai kunnawa, Settings › Playback › Sound*
- **Visualizer,** da aka zana daga injin sautin MixMusic na kansa. *Ana
  kunnawa*

### Kunna a wata naʼura, abokai da sauran kwamfutoci

- **Kunna a wata naʼura** zuwa Chromecast, lasifikan Google Home da Nest da
  rukunoninsu, masu kunnawa na DLNA da UPnP da masu karɓar FCast, har da
  fayilolin kiɗanka na kanka. *Mashayin mai kunnawa, Ƙaramin mai kunnawa*
- **Saurare tare da abokai:** buɗe ɗaki da yake kunnawa tare a kwamfutocin
  abokanka, a hanyar sadarwarka ko ko ina da hanyar gayyata. *Menu na kunna a
  wata naʼura na mashayin mai kunnawa*
- **Sarrafa daga nesa:** sarrafa MixMusic a wata kwamfuta ta hanyar sadarwarka,
  da mayar da abin da ake kunnawa daga ɗaya zuwa ɗaya. *Menu na kunna a wata
  naʼura na mashayin mai kunnawa, Settings › Desktop*
- **Mataimakan AI** a kwamfutarka na iya sanin abin da ake kunnawa, bincike,
  kunnawa da saita mai ƙidayar barci, ta Model Context Protocol. *Settings ›
  Desktop › AI assistants*

### A kan kwamfutarka

- **Ƙaramin mai kunnawa** da yake tsayawa a saman sauran tagoginka, da jerin
  gaba da kalmomin waƙa a cikin allonsa. *Mashayin mai kunnawa, <kbd>Ctrl</kbd>
  <kbd>M</kbd>*
- **Maɓallan mai jarida** da sarrafa mai jarida na tsarin a Windows, macOS da
  Linux. *Built in*
- **Gajerun hanyoyi na duniya** naka na kanka don kunnawa, dakatarwa,
  tsallakewa, sauya ƙara, rufe sauti, so ko ƙin abin da ake kunnawa ko nuna
  MixMusic, ko wace taga ce a gaba. *Settings › Shortcuts*
- **Maɓallan mai kunnawa** a hoton ɗan ƙaramin taskbar a Windows, da mai
  kunnawa a kewayen notch na MacBook. *Built in, Settings › Desktop*
- **Sarrafawa daga tray,** sanarwar sabbin waƙoƙi, kasancewa a sama da
  farawa idan ka shiga kwamfuta. *Settings › Desktop*
- **Hanyoyin `mixmusic://`** da tutoci kamar `--next`, don maɓallin Stream
  Deck ko rubutun umarni. *Layin umarni*
- **Abin da ake kunnawa don overlays:** kati na abin da ake kunnawa don tushen
  burauza na OBS, da bayanansa don shafinka na kanka. *Settings › Desktop*

### Ka mayar da shi naka

- **Tsare-tsare.** Blend, na asali, yana da siffofi masu zagaye da motsi mai
  kuzari a kan layuka masu yawa na manyan murfuna. Expressive, bisa Material
  3 Expressive, yana da annashuwa, da cike-cike masu launuka, maɓallan
  siffar kwaya da mashayin mai kunnawa mai shawagi. Crate ba shi da ƙawa
  mai yawa, don murfuna su yi aikin, kuma Tiles shi ne kamannin MixMusic na
  farko, da tayal-tayal masu lebur da manyan kalmomi na ƙananan haruffa.
  *Settings › Appearance*
- **Jigogi.** Goma sha huɗu a ciki, kowanne a kowane tsari, da tasirin da za
  ka iya daidaitawa, jigo ɗaya don yanayin haske da ɗaya don yanayin duhu, da
  launin ado naka ko wanda aka ɗauko daga hoton waƙar da ake kunnawa. Material
  da Material Dark suna yin kowane launi daga na murfin. *Settings ›
  Appearance*
- **Jigogi da plugin na alʼumma,** a danna sau ɗaya a kasuwa. *Kasuwa*
- **Plugin** suna ƙara shafuka, umarni, ayyukan waƙa da widgets ta hanyar API
  mai nauʼi, ana zana su da jigonka. *Settings › Plugins*
- **Madannai da farko.** Gajerun hanyoyi don duk abin da ake yawan yi, maɓalli
  naka na kowane umarni har da na plugin, maɓallan da ke sarrafa waƙoƙi daga
  kowace manhaja, da command palette (<kbd>Ctrl</kbd> <kbd>K</kbd>) don
  sauran.
- **Harshenka.** Harsuna 66 baya ga Turanci, daga dama zuwa hagu inda harshen
  yake karantuwa haka, da kwanuka da lambobi yadda yankinka yake rubuta su.
  *Settings › Appearance › Language*
- **Don masu karanta allo da madannai.** Sunaye da sanarwa don masu karanta
  allo, kowane sarrafawa a cikin isar madannai, girman rubutu, zuƙowa da
  rage motsi. *Built in, Settings › Appearance*
- **Fayil na saituna.** Saitunanka su ne `settings.json` a babban fayil ɗin
  bayananka (`%AppData%\MixMusic` a Windows, `~/Library/Application Support/MixMusic`
  a macOS, `~/.config/MixMusic` a Linux), wanda za ka iya ajiyewa tare da
  dotfiles ɗinka. Ka ajiye shi yayin da MixMusic yake aiki, sai ya fara aiki
  nan da nan. *Settings › Desktop › Settings file*

### Ajiyar kwafi, shigowa da fitarwa

- **Ajiye kwafi da maido** ɗakin waƙoƙinka, saitunanka da ƙarin abubuwanka, da
  hannu ko kai tsaye, a wannan kwamfutar ko wata. *Settings › Desktop ›
  Backups*
- **Shigo da playlist** daga fayiloli da daga bayananka na Spotify da Google,
  da fitar da jeri da ɗakin waƙoƙinka zuwa fayilolin da sauran masu kunnawa
  suke karantawa. *Ɗakin waƙoƙi › Mixes, Menu na More na shafi*

### Sabuntawa

- MixMusic yana duba sabuwar fitarwa a farawa da kuma bayan kowane ƴan saʼoʼi,
  yana duba sa hannunta, yana kuma shigar da ita idan ka ce, ko da kansa idan
  ka zaɓi haka. *Settings › About*

## Duba kewaye

<p align="center">
  <img src="docs/look-listen.webp" alt="Shafukan MixMusic a tsarin Blend, a tsaye a kan wani fage mai duhu yana fuskantarka: a tsakiya ana kunna アイドル na YOASOBI, an zagaye shi da hoda, kowane layi na kalmomin waƙar da romaji da fassarar Turanci a ƙasansa; albam ɗin THE BOOK 3 a hagu da shafin wani podcast da ake bi a dama" />
</p>

<p align="center"><sub><b>Saurara.</b> A tsakiya, ana kunna アイドル na YOASOBI: kowane layi yana haskakawa yayin da ake rera shi, da romaji da fassara a ƙasansa. A hagu, albam; a dama, podcast da kake bi.</sub></p>

<p align="center">
  <img src="docs/look-yours.webp" alt="Ƙarin MixMusic, a tsaye a kan wani fage mai duhu yana fuskantarka: Settings › Appearance da tsare-tsare huɗun a tsakiya, an zagaye da hoda; jigon Material wanda murfin waƙar da ake kunnawa ya ba shi launi a hagu, da jigogin alʼumma na kasuwa a dama" />
</p>

<p align="center"><sub><b>Ka mayar da shi naka.</b> A tsakiya, tsare-tsare huɗun: Blend, na asali, Expressive, Crate da Tiles, a ƙarƙashin jigogi. A hagu, jigon Material wanda yake ɗaukar launukansa daga murfin da ake kunnawa. A dama, jigogin alʼumma a kasuwa.</sub></p>

## Takardu

Jagororin suna a shafin yanar gizo, a
[mixmusic.functionx.dev/docs](https://mixmusic.functionx.dev/docs/), da kuma a
[fzn0x.github.io/mixmusic/docs](https://fzn0x.github.io/mixmusic/docs/),
inda bincike yake samo kowanne daga cikinsu, da kuma a [`docs/`](docs) a nan.

- **Farawa:** [Farawa](docs/getting-started.md),
  [Siffofi](docs/features.md), [Asusu](docs/accounts.md),
  [Spotify](docs/spotify.md), [Magance matsaloli](docs/troubleshooting.md)
- **Amfani da MixMusic:** [Kunna kiɗa](docs/playing-music.md),
  [Gida](docs/home.md), [Ɗakin waƙoƙinka](docs/library.md),
  [Shigowa da fitar da playlist](docs/import-export.md),
  [Kiɗa a wannan kwamfutar](docs/local-music.md),
  [Kalmomin waƙa](docs/lyrics.md),
  [Podcast](docs/podcasts.md), [Kamanni da jin daɗi](docs/look-and-feel.md),
  [A kwamfuta](docs/desktop.md),
  [Overlays don yawo kai tsaye](docs/stream-overlay.md),
  [Kunna a wata naʼura](docs/casting.md),
  [Sarrafa daga nesa](docs/remote-control.md),
  [Saurare tare da abokai](docs/listen-with-friends.md),
  [Mataimakan AI](docs/ai-assistants.md),
  [Kasuwa](docs/marketplace.md), [Saituna](docs/settings.md)
- **Yadda yake aiki:** [Yadda MixMusic yake aiki](docs/how-it-works.md),
  [Injin sauti](docs/audio-engine.md),
  [Inda rafukan sauti suke fitowa](docs/streams.md),
  [Yadda ake kunna waƙoƙin Spotify](docs/spotify-playback.md),
  [Bayananka da sirri](docs/data-and-privacy.md)
- **Yi naka:** [Jigogi](docs/themes.md), [Plugin](docs/plugins.md)
- **Aikin:** [Yabo](docs/credits.md), [Lasisi](docs/license.md),
  [Gine-gine](docs/architecture/README.md),
  [Gudummawa](CONTRIBUTING.md), [Kaʼidar ɗabiʼa](CODE_OF_CONDUCT.md),
  [Tsaro](SECURITY.md)

## Shigarwa

Zazzage fayil ɗin tsarinka daga
[sabuwar fitarwa](https://github.com/fzn0x/mixmusic/releases/latest):

| Tsari | Zazzagewa |
|---|---|
| macOS 12 zuwa gaba (Apple silicon da Intel) | [`mixmusic-<version>-macos-universal.dmg`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows 10 da 11 (x64, Arm), mai shigarwa | [`mixmusic-<version>-windows-<arch>-setup.exe`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows, mai ɗaukuwa, ba shigarwa | [`mixmusic-<version>-windows-<arch>-portable.zip`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Debian, Ubuntu da makamantansu | [`mixmusic-<version>-linux-<arch>.deb`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Fedora, openSUSE da makamantansu | [`mixmusic-<version>-linux-<arch>.rpm`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Arch Linux da makamantansa (nan ba da jimawa ba) | [`mixmusic-bin`](https://aur.archlinux.org/packages/mixmusic-bin) a AUR: `yay -S mixmusic-bin` |
| Duk Linux, fayil ɗaya | [`mixmusic-<version>-linux-<arch>.AppImage`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Duk Linux, a keɓe (Flatpak) | [`mixmusic-<version>-linux-<arch>.flatpak`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Duk Linux, a babban fayil ɗin gidanka | [`mixmusic-<version>-linux-<arch>.tar.gz`](https://github.com/fzn0x/mixmusic/releases/latest), tare da `install.sh` |

`<arch>` shi ne `amd64` don x64 da `arm64` don Arm. Kowace fitarwa tana da
jerin `SHA256SUMS` don tabbatar da abubuwan da ka zazzage. Ba ka buƙatar
fayilolin da ake kira `update-…`, `….delta`, `mixmusic-<version>-darwin-universal.tar.gz` ko
`-windows-<arch>.tar.gz`: manhajojin da aka shigar suna sabuntawa daga gare
su.

Masu shigarwa, zip mai ɗaukuwa da kundin Linux suna sabunta kansu. `.deb`,
`.rpm`, AppImage da Flatpak ba za su iya maye gurbin fayilolinsu ba, don haka
MixMusic yana gaya maka idan sabon sigar ta fito kuma yana zazzage maka ita.
`mixmusic-bin` na AUR yana sabuntawa tare da sauran tsarinka. Don zip mai
ɗaukuwa, buɗe shi a ko ina sannan ka buɗe `MixMusic.exe`. Don AppImage, mayar
da shi mai aiwatarwa (`chmod +x`) sannan ka buɗe shi. Don Flatpak, buɗe shi da
cibiyar manhajojinka, ko ka gudanar da
`flatpak install ./mixmusic-<version>-linux-<arch>.flatpak`; a karon farko,
Flatpak yana ɗauko GNOME runtime daga Flathub.

A Mac, [Homebrew](https://brew.sh) yana shigar da MixMusic ma, kuma
`brew upgrade` yana sabunta shi:

```sh
brew install --cask fzn0x/tap/mixmusic
```

[Farawa](docs/getting-started.md) yana jagorantarka a shigarwa da farawa na
farko a kowane tsari.

### Farawa na farko

- **macOS:** Apple ba ta tabbatar da manhajar ba tukuna, don haka a karon
  farko da ka buɗe ta, macOS tana cewa ba za ta iya dubata ba. Ka buɗe ta
  ko da haka daga System Settings › Privacy & Security › Open Anyway
  (Saitunan Tsari › Sirri & Tsaro › Buɗe ko da haka), ko ka gudanar da wannan
  sau ɗaya a Terminal:

  ```sh
  xattr -dr com.apple.quarantine /Applications/MixMusic.app
  ```
- **Windows:** ba a sa wa mai shigarwar sa hannu tukuna ba, don haka
  SmartScreen na iya yin gargaɗi game da shi: zaɓi More info, sannan Run
  anyway. Shiga YouTube Music da kunna Spotify a MixMusic suna buƙatar
  WebView2 Runtime, wanda Windows 11 ya ƙunsa.
- **Linux:** MixMusic yana buƙatar GTK 3, da PulseAudio (PipeWire ma yana
  yi masa hidima) ko ALSA. Shiga YouTube Music yana buƙatar WebKitGTK 4.1,
  alamar tray tana buƙatar `libayatana-appindicator3`, kuma adana shigarwarka
  a cikin keyring na tsarin yana buƙatar `libsecret`. `.deb` da `.rpm` suna
  shigar da su, Flatpak kuma yana zuwa da su; don AppImage da kundin:

  ```sh
  # Debian and Ubuntu
  sudo apt install libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
  # Fedora
  sudo dnf install gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
  ```

  MixMusic ba shi da font na haruffan Sinanci, Japananci da Koriyanci, kuma a
  Linux harafin da babu wani font da aka shigar da shi yana bayyana a matsayin
  fanko. `.deb` da `.rpm` suna neman ɗaya; da sauran, shigar da ɗaya da kanka
  sannan ka sake kunna MixMusic:

  ```sh
  # Debian and Ubuntu
  sudo apt install fonts-noto-cjk
  # Fedora
  sudo dnf install google-noto-sans-cjk-vf-fonts
  # Arch
  sudo pacman -S noto-fonts-cjk
  ```
- **Kowane tsari:** MixMusic yana samun sautin yawancin waƙoƙin YouTube Music
  da kansa. Don sauran, kamar waƙoƙin da aka yi wa yara, yana amfani da
  [yt-dlp](https://github.com/yt-dlp/yt-dlp), wanda yake shigar maka daga
  Settings › Playback kuma yake sabuntawa. yt-dlp yana buƙatar JavaScript
  runtime: [Deno](https://deno.com), Node.js, Bun ko QuickJS. Flatpak yana
  da Node.js.

### Flatpak

Flatpak yana gudanar da MixMusic a cikin keɓaɓɓen wuri, wanda yake ganin
hanyar sadarwa, sabar sauti, allo da GPU, kuma ba ya ganin babban fayil ɗin
gidanka sai bayanan martaba na burauzoz da aka shigar daga Flathub, a karantawa
kawai, wanda za a iya shigo da shigar YouTube Music daga gare su. Yana zuwa
don x64 da Arm64, kamar Raspberry Pi 5, Asahi Linux da kwamfutocin
Snapdragon. MixMusic ba ya kan Flathub a yanzu: shigar da `.flatpak` na
fitarwa.

Flatpak yana adana saitunansa da ɗakin waƙoƙinsa a
`~/.var/app/io.github.fzn0x.mixmusic`, a keɓe daga kwafin `.deb`, `.rpm`,
AppImage ko kundi, kuma duka biyun na iya gudana a lokaci guda. Don kawo
saituna da ɗakin waƙoƙin wani kwafi, ajiye shi a Settings › Desktop › Backups
sannan ka maido da fayil ɗin a Flatpak. Don kwafar fayilolin da hannu maimakon
haka, rufe duka biyun, sannan ka kwafi `settings.json`, `library.json`,
`plays.jsonl`, `local-copies.json`, `queue.json`, `positions.json` da manyan
fayilolin `addons` da `mix-covers` daga `~/.config/MixMusic` zuwa
`~/.var/app/io.github.fzn0x.mixmusic/config/MixMusic`. Ka rufe duka biyun da
farko: MixMusic da ke aiki yana riƙe `library.json` da yawancin sauran
fayiloli a cikin ƙwaƙwalwa, kuma idan ya sake adanawa yana rubutawa a kan
fayil ɗin da aka kwafa. Sannan ka sake shiga asusunka: shigarwa suna nan a
keyring na ɗayan kwafin. [Shafin bayanai da sirri](docs/data-and-privacy.md#the-data-folder)
yana bayyana abin da kowane fayil yake ɗauke da shi.

A Flatpak, farawa idan ka shiga kwamfuta yana bi ta kwamfutarka, wadda za ta
iya tambayarka da farko, kuma yt-dlp naka na kanka dole ne ya kasance a
`~/.var/app/io.github.fzn0x.mixmusic`, domin keɓaɓɓen wurin ba ya ganin wani
babban fayil. Manhajar Discord wadda ba Flatpak ba tana nuna abin da ake
kunnawa ne kawai idan tana aiki lokacin da MixMusic ya fara, har sai ta sake
farawa; Discord a matsayin Flatpak yana aiki ta kowace hanya. Flatpak sabo ne:
gaya mana a cikin [issue](https://github.com/fzn0x/mixmusic/issues) yadda
yake aiki a kwamfutarka.

### Asusu (duka na zaɓi ne)

- **YouTube Music:** zaɓi Sign in to YouTube Music a Settings › Accounts
  sannan ka shiga Google a taga da ta buɗe. Abin da ka rubuta
  yana tafiya zuwa Google kaɗai. MixMusic yana adana zaman a ɓoye, da maɓalli
  a keyring na tsarinka idan akwai. Inda wannan taga ba za ta buɗe ba, ko
  Google ta ƙi ta, shiga da burauzarka, wadda MixMusic yake buɗewa da sabuwar
  bayanan martaba nata kuma yake share ta da zarar ka shiga; ka shigo da
  zaman burauzar da ka shiga da ita (MixMusic yana karanta shi da yt-dlp
  ɗinsa, kuma yana riƙe da kukis na YouTube kaɗai); ko ka liƙa Cookie header
  na shafin music.youtube.com da aka shiga.
- **Spotify:** Spotify yana barin manhaja ta yi wa masu amfani har biyar
  hidima, kuma muddin mai ita yana da Premium kawai, don haka kana shiga ne
  da manhajar Spotify ta kanka. Yana ɗaukar minti biyu:
  [docs/spotify.md](docs/spotify.md) yana jagorantarka.
- **Last.fm:** ƙirƙiri asusun API a
  [last.fm/api/account/create](https://www.last.fm/api/account/create)
  (kowane suna, ba callback URL), liƙa API key da shared secret ɗinsa a
  Settings › Accounts, sannan ka amince da MixMusic a burauzarka.
- **ListenBrainz:** liƙa user token na
  [saitunan ListenBrainz ɗinka](https://listenbrainz.org/settings/), ko na
  sabarka ta kanka, a Settings › Accounts.
- **Libre.fm:** haɗa shi a Settings › Accounts, sannan ka amince da MixMusic
  a shafin Libre.fm a burauzarka, ko na wata sabar GNU FM.
- **Discord:** muddin manhajar Discord tana aiki, bayananka suna nuna abin da
  kake saurara. Kashe shi a Settings › Desktop.

[Asusu](docs/accounts.md) yana bayyana kowane matakin shiga mataki da mataki.

### Iyakokin da aka sani

- **Windows a kan Arm** ba a gwada shi sosai kamar x64, kuma zip mai ɗaukuwa
  ba ya rajistar hanyoyin `mixmusic://`.
- **Manhajojin Spotify a yanayin ci gaba,** kamar wadda kake shiga da ita,
  suna ba da sakamakon bincike goma na kowane irin, kuma waƙoƙin playlist ɗin
  da ba naka ba suna ɓoye.
- **A Linux,** Spotify yana kunnawa ta Spotify Connect a asali, domin
  WebKitGTK ba zai iya kunna sautin Spotify da aka kare ba.
- **YouTube** na iya sauya mahaɗansa ya karya bincike, kunnawa ko shiga har
  sai MixMusic ko yt-dlp sun cim masa.

Wani abu ba ya aiki? [Magance matsaloli](docs/troubleshooting.md) yana ƙunshe
da matsalolin da aka fi samu da kuma matsalolin da aka sani.

## Gina daga tushe

Kana buƙatar [Go](https://go.dev/dl/) 1.27.3 ko sama da haka,
[Bun](https://bun.sh) 1.4 ko sama da haka, wanda yake gudanar da layin umarni
na MyGo, da Git. MyGo ba ya buƙatar C compiler, don haka kowane tsari na iya
gina MixMusic ga kowane tsari.

### Windows

```powershell
winget install GoLang.Go
winget install Oven-sh.Bun
winget install Git.Git
```

Buɗe sabon terminal, sannan:

```powershell
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

Shiga da kunna Spotify suna buƙatar WebView2 Runtime, wanda Windows 11 ya
ƙunsa. A Windows 10, shigar da shi daga
[Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/).

### macOS

Da [Homebrew](https://brew.sh):

```sh
brew install go oven-sh/bun/bun
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Linux

Shigar da Go daga [go.dev/dl](https://go.dev/dl/) (fakitin rarrabawa galibi
sun fi 1.27.3 tsufa), sannan Bun da ɗakunan karatu da MixMusic yake
gudana da su:

```sh
curl -fsSL https://bun.sh/install | bash
# Debian and Ubuntu
sudo apt install git libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
# Fedora
sudo dnf install git gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
```

Sannan:

```sh
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Nix

Da [Nix](https://nixos.org), `nix develop` yana ba ka kayan aikin da ake gina
fitarwa da su, a sigogin da `flake.lock` ya kulle: Go 1.27, Bun, GoReleaser,
jq, zip da Zola, kuma a Linux NSIS, osslsigncode da binutils. `nix build`
yana gina MixMusic don Linux daga tushe, zuwa cikin `result/`, kuma
`nix run github:fzn0x/mixmusic` yana gudanar da shi.

### Haɓakawa

`bun run dev` yana gina manhajar, ya fara ta, kuma yana sake gina ta yayin da
kake gyarawa. Bayananta suna keɓe daga MixMusic da aka shigar, a ƙarƙashin
suna `MixMusic Dev`.

```sh
bun run build:plugins          # the plugins' JavaScript, from their TypeScript
go test ./...                  # the app's tests
bun run typecheck              # the plugin SDK and the community's plugins
go run ./tools/registry -check # the community registry
bun run build                  # the packaged app for this system, in build/
bun run build -- -platform windows/amd64,linux/amd64,linux/arm64
```

A cikin Nix shell, gina abubuwan zazzagewa na Linux da
`.github/scripts/build.sh linux/amd64,linux/arm64`, kamar yadda fitarwa
suke yi. `bun run build` kawai a can yana yin shirin Linux wanda yake neman
loader na Nix, kuma yana farawa ne kawai a NixOS.
`go run ./tools/elfloader build/linux-amd64` yana duba gini.

[Shafin yanar gizo](https://mixmusic.functionx.dev/) yana cikin `site/`,
an gina shi da [Zola](https://www.getzola.org), kuma jagororin da ke cikin
`docs/` su ne shafukansa na `/docs/`. `go run ./tools/sitedocs && zola --root site serve`
yana nuna shi kafin a buga. Ana gina fitarwa a cikin Nix shell kuma
[GoReleaser](https://goreleaser.com) ne yake bugawa (`.goreleaser.yaml`).

[CONTRIBUTING.md](CONTRIBUTING.md) yana da gwaje-gwajen da za a gudanar kafin
pull request, kuma [docs/architecture](docs/architecture/README.md) yana
bayyana yadda sassan suke haɗuwa, shafi ɗaya ga kowanne.

### Tsari

Manhajar ita ce babban package, tare da packages na `internal/` a ƙarƙashinta.
Packages ɗin da suke wajen `internal/` na kowa ne ya shigo da su, kuma an
rubuta su a
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic).

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

[docs/architecture](docs/architecture/README.md) yana bayyana sauran packages
na `internal/`, shafi ɗaya ga kowane sashe na manhajar.

Packages ɗin da za ka iya shigowa da su:

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

## Ƙungiya

Kowa na iya shigar da jigogi da plugin na alʼumma daga kasuwa, alamar shago
da ke gefen shafi, kuma kana iya ƙara naka.

### Yi jigo

Jigo fayil ne na `theme.json` wanda yake saita launuka, haruffa da kusurwoyi,
waɗanda ake gani a tsarin da mai amfani ya zaɓa.
[docs/themes.md](docs/themes.md) yana lissafa kowane token kuma yana nuna
cikakken jigo. Don gwada naka, saka babban fayil ɗinsa a babban fayil ɗin
jigogi (Settings › Appearance › open themes folder; Saituna › Kamanni › buɗe
babban fayil ɗin jigogi) sannan ka zaɓe shi a Settings › Appearance.

### Yi plugin

Plugin ƙaramin module ne, da aka rubuta da TypeScript ko JavaScript, wanda
yake ƙara shafuka, umarni, ayyukan waƙa da widgets.
[docs/plugins.md](docs/plugins.md) yana bayyana API ɗin, tare da
[nauʼoʼinsa](sdk/index.d.ts) don editanka. [Sleep timer](community/plugins/sleep-timer)
da [spectrum](community/plugins/spectrum) na alʼumma misalai ne masu aiki.

### Raba shi

1. Yi fork na wannan repository.
2. Ƙara babban fayil guda ɗaya: `community/themes/<id>/` tare da
   `theme.json` ɗinsa don jigo, ko `community/plugins/<id>/` tare da
   `plugin.json` da `index.ts` ɗinsa don plugin.
3. Gudanar da `bun install`, `bun run build:plugins`, `go run ./tools/registry`
   da `go run ./tools/registry -check`.
4. Buɗe pull request, tare da hoton allo.

Da zarar an haɗa shi, CI yana buga shi a
[reshen registry](https://github.com/fzn0x/mixmusic/tree/registry), kuma
yana fitowa a kasuwar kowa. [community/README.md](community/README.md)
yana da cikakkun bayanai.

## Sirri

MixMusic ba shi da asusu, sabar, nazari ko telemetry na kansa. Yana magana
da:

- **YouTube da Google:** kundin YouTube Music, rafukansa da yadda kowace waƙa
  take da ƙara, ana tambaya yayin da ba a shiga ba, da hotunan murfi; wasu
  rafuka ta yt-dlp. Idan an shiga, ɗakin waƙoƙinka a can, kuma, sai dai ka
  kashe a Settings › Accounts, waƙoƙin da kake kunnawa, waɗanda suke shiga
  tarihin YouTube Music ɗinka. YouTube kansa, ba tare da shiga ba kuma ba
  tare da kukis ba, don zaɓin bincike na YouTube, duba waƙoƙin da aka yi da
  AI, rubutun bidiyo a matsayin kalmomin waƙa da rubutun furuci, da kalmomin
  waƙa da aka daidaita da bidiyon waƙa.
- **Spotify,** da zarar ka shiga: ɗakin waƙoƙinka, naʼurorinka na Spotify
  Connect, da mai kunnawa na Spotify, wanda ake lodawa daga `sdk.scdn.co`,
  idan Spotify yana kunnawa a MixMusic.
- **Better Lyrics:** taken waƙar da ake kunnawa, mawaƙinta na farko, albam
  ɗinta da tsawonta, don samo kalmomin waƙarta, da zarar tana da albam da
  tsawo.
- **Unison:** ID ɗin bidiyon waƙar da ake kunnawa, ko, idan babu shi, taken
  waƙar, mawaƙanta, albam ɗinta da tsawonta, don samo kalmomin waƙarta. Sai
  yayin da ka kunna fassara, ko rubuta Larabci da haruffan Latin, a gefen
  kalmomin waƙa, kalmomin waƙar da ake kunnawa, tare da ID ɗin bidiyonta,
  waɗanda Unison yake sa Google ta fassara ko ta rubuta da haruffan Latin.
- **LRCLIB:** taken waƙar da ake kunnawa, mawaƙanta, albam ɗinta da tsawonta,
  don samo kalmomin waƙarta.
- **SponsorBlock,** sai yayin da ka kunna Skip the parts SponsorBlock marks:
  haruffa 4 na farko na hash ɗin ID ɗin kowane bidiyo, ba ID ɗin kansa ba,
  don samo sassan da ya yi wa alama.
- **GitHub:** duba sabuwar sigar a farawa da kuma bayan kowane ƴan saʼoʼi,
  wanda za ka iya kashewa a Settings › About; rajistar alʼumma idan ka buɗe
  kasuwa; abubuwan zazzagewa na yt-dlp; jerin belun kunne na AutoEq da
  bayanin da ka zaɓa; kuma, a karon farko da ka kunna romaji a gefen
  kalmomin waƙar Japananci, ƙamus ɗin karatun kanji (11.7 MB). Ana yin
  romaji, da haruffan Latin na sauran rubutu in ban da Larabci, a
  kwamfutarka: ba a aika kalmomin waƙar.
- **Last.fm,** da zarar ka haɗa shi: abin da kake kunnawa.
- **ListenBrainz,** ko sabarka ta kanka mai API ɗinsa, da zarar ka haɗa
  shi: abin da kake kunnawa.
- **Libre.fm,** ko wata sabar GNU FM, da zarar ka haɗa shi: abin da kake
  kunnawa.
- **Discord,** sai dai ka kashe shi: abin da ake kunnawa, ana aika shi zuwa
  manhajar Discord a kwamfutarka, wadda take nuna shi a bayananka.
- **Naʼurorin da kake kunna wa,** a hanyar sadarwarka: taken waƙa, mawaƙi da
  murfin abin da ake kunnawa. Suna ɗauko sautin daga MixMusic.
- **Sauran kwamfutoci masu MixMusic,** don sarrafa daga nesa da Saurare tare
  da abokai, a ɓoye, a hanyar sadarwarka ko, yayin da kake barin abokai a
  wajenta su shiga, ta intanet. Sai a nemi router ɗinka ya tura tashar jiragen
  ruwa ɗaya, kuma hanyar gayyatarka tana ɗauke da adireshin intanet ɗinka.
- **Mataimakan AI** a kwamfutarka, yayin da ka ba su izini: abin da ake
  kunnawa, jerin gaba, sakamakon bincikensu da sunayen playlist ɗinka, wanda
  mataimaki na iya mikawa ga samfurin AI da yake amfani da shi.
- **YouTube da Spotify,** don sanin ko hanyar sadarwa tana amsawa: buƙata ta
  HTTPS don amsa mara komai.

Plugin na iya aika duk abin da suke so: ana nuna maka izinin plugin, ba a tilasta
shi, shi ya sa MixMusic yake neman ka amince da plugin kafin ya kunna shi.
[Bayananka da sirri](docs/data-and-privacy.md#what-each-feature-sends-where)
yana da tebur na abin da kowace siffa take aikawa da inda.

Abin da yake adanawa a kwamfutarka:

- Saitunanka da ɗakin waƙoƙinka (waƙoƙin da ka so da waɗanda ka ƙi, mixes,
  kowane kunnawa da ƙididdigarka take ƙirgawa, mawaƙa da shirye-shiryen da
  kake bi, maƙalli, manyan fayiloli da alamomi, tarihi, binciken kwanan nan,
  da ƙundin fayilolin kiɗanka), a babban fayil ɗin da Settings › About ya
  ambata.
- Waƙoƙin da ka kunna, a cikin maʼajiya, gwargwadon abin da ka yarda a
  Settings › Playback, da yadda YouTube ya auna ƙararsu. Waƙoƙin da ka
  zazzage, a babban fayil ɗin zazzagewa.
- Murfunan da ka gani, har zuwa 256 MB.
- Fassarar kalmomin waƙar da ka kunna, bisa waƙa da harshe, da ƙamus ɗin da
  romaji yake karanta kanji da shi, da zarar ka kunna shi.
- Shigarwarka, har da zaman YouTube Music, a keyring na tsarin. A Windows,
  idan Credential Manager ba zai iya riƙe ɗaya ba, yana shiga cikin fayil da
  aka ɓoye don asusun Windows ɗinka; a Linux ba tare da keyring ba, cikin
  fayilolin da mai amfaninka kaɗai zai iya karantawa.
- Log, ba tare da shigarwarka ba.

Shafin tarihi na ɗakin waƙoƙi yana goge abin da ka kunna, Settings › Playback
yana goge maʼajiya da murfuna, kuma fita daga wani aiki yana share shigarsa.
[Goge, mayar ko sake farawa](docs/data-and-privacy.md#clear-move-or-reset)
yana bayyana yadda ake mayar da bayananka, ajiye kwafi mai ɗaukuwa da
`MIXMUSIC_DATA_DIR`, ko fara sabo.

A Windows, visualizer yana jin sautin Spotify da ake kunnawa a wannan
kwamfutar, a MixMusic ko a manhajar Spotify, ta process loopback na Windows,
wanda yake jin wannan manhajar kaɗai. A macOS 14.2 zuwa gaba, yana jin
Spotify da ake kunnawa a MixMusic ta process tap, da zarar ka yarda, yadda
macOS take tambaya. Yana nazarin sautin a cikin ƙwaƙwalwa kuma ba ya taɓa
rikodinsa ko aika shi.

## Yabo

An gina MixMusic a kan waɗannan ayyuka:

- **[MyGo](https://github.com/egoist/mygo)**, tsarin manhajar kwamfuta da aka
  gina MixMusic da shi. MixMusic yana amfani da UI na asali na MyGo: MyGo yana
  zana kowace taga da kansa, a Go, ba tare da webview ba, daga shafuka da
  mashayin mai kunnawa har zuwa ƙaramin mai kunnawa. Layin umarninsa yana
  tattara manhajar, tare da masu shigarwarta da sabuntawar da aka sa wa
  hannu waɗanda take sabunta kanta daga gare su.
- **[Go](https://go.dev)**, harshen manhajar, tare da
  [purego](https://github.com/ebitengine/purego), wanda yake kiran
  ɗakunan karatu na tsarin ba tare da cgo ba, da
  [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) da
  [golang.org/x/text](https://pkg.go.dev/golang.org/x/text).
- **[Sobek](https://github.com/grafana/sobek)**, injin JavaScript da kowane
  plugin yake gudana a ciki.
- **[Nix](https://nixos.org)**, wanda yake kulle kayan aikin da ake gina
  fitarwa da su, kuma yake gina MixMusic don Linux daga tushe.
- **[GoReleaser](https://goreleaser.com)**, wanda yake buga fitarwa, da
  [nFPM](https://nfpm.goreleaser.com) ɗinsa, wanda yake yin fakitin RPM.
- **[Bun](https://bun.sh)**, wanda yake gudanar da ginin, da
  **[TypeScript](https://www.typescriptlang.org)**, wanda yake tattara
  plugin kuma yake duba nauʼoʼin SDK ɗin plugin.
- **[Zola](https://www.getzola.org)**, wanda yake gina shafin yanar gizo.
- **[NSIS](https://nsis.sourceforge.io)**, wanda yake yin masu shigarwa na
  Windows, da [appimagetool](https://github.com/AppImage/appimagetool),
  AppImage.
- **[yt-dlp](https://github.com/yt-dlp/yt-dlp)**, wanda yake samo sautin
  waƙoƙin da MixMusic ba zai iya samowa da kansa ba.
- **[SponsorBlock](https://sponsor.ajay.app)**, buɗaɗɗen rumbun bayanai na
  sassan bidiyon YouTube da jamaʼa ke bayarwa, na Ajay Ramachandran da
  masu ba da gudummawarsa, wanda MixMusic yake tsallake sassansa idan ka
  roƙe shi. Uses SponsorBlock data licensed under
  [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/)
  from https://sponsor.ajay.app/.
- **[Better Lyrics](https://betterlyrics.org)**, sabis da ke adana kalmomin waƙa masu daidaita kalma bayan kalma.
- **[Unison](https://unison.betterlyrics.org)**, buɗaɗɗen maʼajiyar kalmomin
  waƙa da jamaʼa ke bayarwa, a ƙarƙashin Unison Community License, da **[LRCLIB](https://lrclib.net)**,
  buɗaɗɗiyar maʼajiyar kalmomin waƙa masu daidaita da waƙar.
- **[kagome](https://github.com/ikawaha/kagome)**, mai nazarin tsarin kalmomin
  Japananci wanda yake karanta kanji na kalmomin waƙa don romaji ɗinsu,
  tare da ƙamus ɗin IPA na
  [kagome-dict](https://github.com/ikawaha/kagome-dict), da aka yi daga
  mecab-ipadic, wanda MixMusic yake zazzagewa a karon farko da aka kunna
  romaji.
- **[libopus](https://opus-codec.org)**, wanda decoder ɗinsa shi ne tushen
  decoder na Opus, da **[SharpJaad](https://github.com/jimm98y/SharpJaad)**,
  fassarar C# ta [JAAD](https://sourceforge.net/projects/jaadec/) wadda
  decoder na AAC ya samo asali daga gare ta.
- **[Chromium](https://www.chromium.org)**, Web Audio nasa, wanda biquad,
  DynamicsCompressor da AnalyserNode ɗinsa su ne tushen matattarun equalizer,
  daidaita ƙarar sauti da visualizer.
- **[AutoEq](https://github.com/jaakkopasanen/AutoEq)**, na Jaakko Pasanen
  da masu ba da gudummawarsa, wanda bayanansa suke gyara sautin belun kunne,
  a tsarin [Equalizer APO](https://sourceforge.net/projects/equalizerapo/).
- **[go-mp3](https://github.com/hajimehoshi/go-mp3)** da
  **[jfreymuth/vorbis](https://github.com/jfreymuth/vorbis)**, waɗanda suke
  decoding na fayilolin MP3 da Vorbis a kwamfutarka, da
  **[Xiph.Org](https://xiph.org)**, wanda `flac` da `ogg` suke karanta FLAC
  da Ogg ɗinsa.
- **[FFmpeg](https://ffmpeg.org)**, wanda gwaje-gwajen decoders suke kwatantawa
  da shi.
- **[Selawik](https://github.com/microsoft/Selawik)**,
  **[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily)**,
  **[Roboto Flex](https://github.com/googlefonts/roboto-flex)** da
  **[Archivo](https://github.com/Omnibus-Type/Archivo)**, haruffan tagogi.
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities)**,
  wanda launukan jigogin Material fassarar kimiyyar launi ta HCT ɗinsa ne.
- **[FCast](https://docs.fcast.org)**, buɗaɗɗiyar yarjejeniyar sadarwa wadda
  MixMusic yake amfani da ita wajen kunna waƙa a kan masu karɓar FCast. MixMusic
  aiwatarwa ce mai zaman kanta ta ita, kuma ba shi da alaƙa da FUTO; FCast
  alamar kasuwanci ce mai rijista ta FUTO.
- **[Kevin MacLeod](https://incompetech.com)**, wanda kiɗansa, a ƙarƙashin
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), yake kunnawa a
  shafin yanar gizo.

[Yabo](docs/credits.md) yana ambaton kowane aiki, takarda da maʼauni da
MixMusic yake ginuwa a kansu.

## Gudummawa

Gyare-gyare, sabbin siffofi, jigogi, plugin da rahotannin kurakurai duka ana
maraba da su.

- **Ba da rahoton matsala** daga Settings › About › report a problem. Yana
  buɗe sabon issue na GitHub da sigarka, tsarinka, jigonka da plugin ɗinka a
  cike. Ko kuma [buɗe issue](https://github.com/fzn0x/mixmusic/issues/new)
  da kanka.
- **Aika canji** a matsayin pull request. [CONTRIBUTING.md](CONTRIBUTING.md)
  yana bayyana gwaje-gwajen da za a gudanar, salon commit, da shigar jerin
  canje-canje da canjin da mutane za su lura da shi yake buƙata.
- **Raba jigo ko plugin:** duba [Ƙungiya](#ƙungiya).
- **Ba da rahoton rauni na tsaro** a asirce, yadda [SECURITY.md](SECURITY.md)
  ya bayyana.

Duk wanda yake shiga cikin aikin yana bin [kaʼidar ɗabiʼa](CODE_OF_CONDUCT.md).
[CONTRIBUTORS.md](CONTRIBUTORS.md) yana gode wa mutanen da lambarsu,
raʼayoyinsu da rahotanninsu suka tsara MixMusic.

## Lasisi

MixMusic yana da lasisi biyu: yi amfani da shi a ƙarƙashin
[lasisin MIT](LICENSE-MIT) ko
[Lasisin Jamaʼa na GNU, sigar 3](LICENSE-GPL), duk wanda ka fi so. Ana karɓar
gudummawa a ƙarƙashin duka biyun. Haruffan da ke `fonts` suna ƙarƙashin SIL
Open Font License: [na Selawik](fonts/OFL.txt),
[na Quicksand](fonts/quicksand-OFL.txt),
[na Roboto Flex](fonts/robotoflex-OFL.txt) da
[na Archivo](fonts/archivo-OFL.txt). Decoder na Opus fassara ne na libopus,
a ƙarƙashin [lasisin BSD ɗinsa](opus/LICENSE.libopus), decoder na AAC ya samo
asali daga SharpJaad, a ƙarƙashin [lasisin MIT ɗinsa](aac/LICENSE.sharpjaad),
kuma kimiyyar launi ta HCT fassara ce ta Material Color Utilities, a ƙarƙashin
[lasisin Apache ɗinsa](internal/hct/LICENSE.material-color-utilities). Sauran
sassa suna riƙe lasisin abin da suka samo asali daga gare shi, kamar matattarun
Web Audio na Chromium, go-mp3, jfreymuth/vorbis, kagome da bayanan Unihan:
[Lasisi](docs/license.md) yana lissafa kowanne.

MixMusic ba shi da alaƙa da Spotify ko Google, kuma ba su amince da shi ko
sun ɗauki nauyinsa ba. Yana kunna abin da waɗannan ayyuka suke bai wa
masu amfani da su, ta hanyoyin ayyukan da kansu; yi amfani da shi yadda
sharuɗansu suka yarda. Spotify da YouTube Music alamun kasuwanci ne na
masu su.
