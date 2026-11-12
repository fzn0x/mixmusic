<p align="center"><a href="README.md">English</a> · <a href="README.hi.md">हिन्दी</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ur.md">اردو</a> · <strong>Hausa</strong> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.bn.md">বাংলা</a> · <a href="README.ru.md">Русский</a> · <a href="README.am.md">አማርኛ</a></p>

<p align="center">
  <img src="brand/icon.png" width="112" height="112" alt="Alamar MixMusic: zobba biyu masu launin hoda da maɓallin kunnawa a kan baƙin fage" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center"><strong>Mai kunnawa guda ɗaya don YouTube Music da Spotify.</strong></p>

<p align="center">Manhajar YouTube Music da Spotify don macOS, Windows da Linux. YouTube Music yana aiki ba tare da asusu ba. Bincike ɗaya, jerin gaba ɗaya, playlist ɗin da aka haɗa, kalmomin waƙa masu daidaita da waƙar, jigogi da plugin.</p>

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
  <a href="https://fzn0x.github.io/mixmusic/">Shafin yanar gizo</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">Zazzage</a> ·
  <a href="https://fzn0x.github.io/mixmusic/docs/">Takardu</a> ·
  <a href="CHANGELOG.md">Jerin canje-canje</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic 0.5.9 a yanayinsa na asali, tsarin Blend tare da jigon Mix, a tsaye a kan wani fage mai duhu yana fuskantarka: a tsakiya ana kunna アイドル na YOASOBI, an zagaye shi da hoda, tare da romaji a ƙasan kowane layi na kalmomin waƙar da ƙaramin mai kunnawa a gabansa; a hagu akwai bincike, a dama kuma mix ɗin da ke cikin ɗakin waƙoƙi" />
</p>

## Dalilin MixMusic

Idan waƙoƙinka sun rabu tsakanin YouTube Music da Spotify, yawanci kana
buƙatar manhajoji biyu, da bincike biyu, jerin gaba biyu da ɗakunan waƙoƙi
biyu. MixMusic yana haɗa ayyukan biyu a cikin mai kunnawa guda ɗaya:

- **Bincike ɗaya.** Yana bincika YouTube Music da Spotify a lokaci guda, kuma
  yana alamta kowane sakamako da aikin da ya fito daga gare shi.
- **Jerin gaba ɗaya da ɗakin waƙoƙi ɗaya.** Waƙar Spotify na iya biyo bayan
  waƙar YouTube Music kai tsaye, kuma playlist ɗaya na iya ɗaukar waƙoƙi daga
  duka biyun.
- **Ba ya buƙatar asusu.** YouTube Music yana kunnawa da zarar ka buɗe
  manhajar. Ka shiga ne kawai idan kana son ɗakin waƙoƙinka na kanka. Spotify
  na zaɓi ne.
- **Ƙaramin shiri.** MixMusic shiri ne guda ɗaya na Go wanda yake zana
  tagoginsa da kansa a kan GPU. Babu burauza a cikinsa da za a zazzage ko a
  riƙe a cikin ƙwaƙwalwa.
- **Naka ne ka sauya shi.** Yana zuwa da tsari huɗu da jigogi goma sha huɗu,
  alʼumma na ƙara jigogi da plugin, kuma kowane umarni na iya samun maɓalli
  naka na kanka.
- **Sirri.** MixMusic ba shi da asusu, sabar, nazari ko telemetry na kansa.
- **Kyauta kuma open source,** a ƙarƙashin lasisin MIT ko GPL 3.0.

## Siffofi

### Bincike da kunnawa

- **Bincike ɗaya don ayyukan biyu.** Sakamakon YouTube Music da Spotify a
  gefe da gefe, kowanne da alamar aikinsa. Sakamakon YouTube Music yana nuna
  sau nawa aka kunna kowace waƙa, don a rarrabe ta asali daga waɗanda aka
  sake rerawa.
- **Jerin gaba ɗaya.** Haɗa waƙoƙin ayyukan biyu, ka gauraya ko ka maimaita a
  tsakaninsu, kuma ka bar radio ya ci gaba idan jerin gaba ya ƙare.
- **Explore (Bincika).** Sabbin fitarwa, waƙoƙin da ake yi, jadawalin waƙoƙi,
  yanayi da nauʼoʼi, da waƙoƙin da suka dace da wadda ake kunnawa.
- **Kalmomin waƙa masu daidaita da waƙar** waɗanda suke bin waƙar, ko rubutu
  kawai idan ka fi so.
- **Ƙaramin mai kunnawa** da yake tsayawa a saman sauran tagoginka.

### Ɗakin waƙoƙinka

- **Mixes:** playlist ɗin da suke ɗaukar waƙoƙi daga kowane aiki. Shigar da
  playlist na wani aiki cikin mix, ka bincika ka kuma tsara a cikin kowanne
  daga cikinsu.
- **Waƙoƙin da ka so** daga ayyukan biyu a jeri ɗaya.
- **Tarihi da ƙididdigar saurare,** rana da rana.
- **Asusunka.** Ka shiga YouTube Music a shafin Google na kansa don samun
  shafin gida, waƙoƙin da ka so, playlist, albam da mawaƙa, a matsayin
  asusunka ko ɗaya daga cikin tashoshin alamar kasuwancinsa. Ka shiga Spotify
  don ɗakin waƙoƙinka a can. Waƙoƙin da ka so, albam da playlist ɗin da ka
  adana, da mawaƙan da kake bi suna kai wa aikin ma. Abin da kake kunnawa
  yana shiga tarihin YouTube Music ɗinka, wanda shawarwarinsa suke fitowa
  daga gare shi.

### Spotify ta hanyoyi uku

- **A cikin MixMusic kansa,** da ingancin Spotify, ba tare da manhajar
  Spotify a buɗe ba (Premium, a Windows da macOS).
- **A Spotify Connect:** manhajar Spotify ɗinka, wayarka ko lasifika.
- **Daga YouTube Music:** MixMusic yana kunna rikodin waƙar Spotify a
  YouTube Music, wanda ba ya buƙatar Premium don kunnawa.

### Sauti

- Crossfade, daidaita ƙarar waƙoƙi da daidaita ƙarar sauti.
- Equalizer mai madaurai goma tare da saitunan da aka shirya.
- Visualizer.
- Maʼajiyar waƙoƙin da ka kunna don sauraro ba tare da intanet ba.

### A kan kwamfutarka

- Maɓallan mai jarida da sarrafa mai jarida na tsarin a Windows, macOS da
  Linux.
- Gajerun hanyoyi na duniya naka na kanka don kunnawa, dakatarwa, tsallakewa,
  sauya ƙara, rufe sauti, so ko ƙin abin da ake kunnawa ko nuna MixMusic, ko
  wace taga ce a gaba.
- Maɓallan mai kunnawa a hoton ɗan ƙaramin taskbar a Windows.
- Sarrafawa daga tray, sanarwar sabbin waƙoƙi, da farawa idan ka shiga
  kwamfuta.
- Hanyoyin `mixmusic://`.
- **Last.fm:** kowace waƙa da ka ji tana shiga bayananka, kuma scrobbles da
  aka yi ba tare da intanet ba ana aika su idan ka dawo kan intanet.
- **Discord:** bayananka suna nuna abin da ake kunnawa, tare da hotonsa da
  lokacin da ya rage.
- **Sabuntawa:** MixMusic yana duba sabuwar fitarwa a farawa da kuma bayan
  kowane ƴan saʼoʼi, yana duba sa hannunta, yana kuma shigar da ita idan ka
  ce, ko da kansa idan ka zaɓi haka.

### Ka mayar da shi naka

- **Tsare-tsare.** Blend, na asali, yana da siffofi masu zagaye da motsi mai
  kuzari a kan layuka masu yawa na manyan murfuna. Expressive, bisa Material
  3 Expressive, yana da annashuwa, da cike-cike masu launuka, maɓallan
  siffar kwaya da mashayin mai kunnawa mai shawagi. Crate ba shi da ƙawa
  mai yawa, don murfuna su yi aikin, kuma Tiles shi ne kamannin MixMusic na
  farko, da tayal-tayal masu lebur da manyan kalmomi na ƙananan haruffa.
- **Jigogi.** Goma sha huɗu a ciki, kowanne a kowane tsari, da tasirin da za
  ka iya daidaitawa, jigo ɗaya don yanayin haske da ɗaya don yanayin duhu, da
  launin ado naka ko wanda aka ɗauko daga hoton waƙar da ake kunnawa. Material
  da Material Dark suna yin kowane launi daga na murfin.
- **Jigogi da plugin na alʼumma,** a danna sau ɗaya a kasuwa.
- **Plugin** suna ƙara shafuka, umarni, ayyukan waƙa da widgets ta hanyar API
  mai nauʼi, ana zana su da jigonka.
- **Madannai da farko.** Gajerun hanyoyi don duk abin da ake yawan yi, maɓalli
  naka na kowane umarni har da na plugin, maɓallan da ke sarrafa waƙoƙi daga
  kowace manhaja, da command palette (<kbd>Ctrl</kbd> <kbd>K</kbd>) don
  sauran.
- **Fayil na saituna.** Saitunanka su ne `settings.json` a babban fayil ɗin
  bayananka (`%AppData%\MixMusic` a Windows, `~/Library/Application Support/MixMusic`
  a macOS, `~/.config/MixMusic` a Linux), wanda za ka iya ajiyewa tare da
  dotfiles ɗinka. Ka ajiye shi yayin da MixMusic yake aiki, sai ya fara aiki
  nan da nan; Settings › Desktop › Settings file (Saituna › Desktop › Fayil
  na saituna) yana buɗe shi.

## Duba kewaye

<p align="center">
  <img src="docs/look-listen.webp" alt="Shafukan MixMusic a tsarin Blend, a tsaye a kan wani fage mai duhu yana fuskantarka: a tsakiya ana kunna アイドル na YOASOBI, an zagaye shi da hoda, kowane layi na kalmomin waƙar da romaji da fassarar Turanci a ƙasansa; albam ɗin THE BOOK 3 a hagu da shafin wani podcast da ake bi a dama" />
</p>

<p align="center"><sub><b>Saurara.</b> A tsakiya, ana kunna アイドル na YOASOBI: kowane layi yana haskakawa yayin da ake rera shi, da romaji da fassara a ƙasansa. A hagu, albam; a dama, podcast da kake bi.</sub></p>

<p align="center">
  <img src="docs/look-yours.webp" alt="Ƙarin MixMusic, a tsaye a kan wani fage mai duhu yana fuskantarka: Settings › Appearance (Saituna › Kamanni) da tsare-tsare huɗun a tsakiya, an zagaye da hoda; jigon Material wanda murfin waƙar da ake kunnawa ya ba shi launi a hagu, da jigogin alʼumma na kasuwa a dama" />
</p>

<p align="center"><sub><b>Ka mayar da shi naka.</b> A tsakiya, tsare-tsare huɗun: Blend, na asali, Expressive, Crate da Tiles, a ƙarƙashin jigogi. A hagu, jigon Material wanda yake ɗaukar launukansa daga murfin da ake kunnawa. A dama, jigogin alʼumma a kasuwa.</sub></p>

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
fayilolin da ake kira `update-…`, `….delta`, `mixmusic-<version>-macos-…` ko
`-windows-<arch>.tar.gz`: manhajojin da aka shigar suna sabuntawa daga gare
su.

Masu shigarwa, zip mai ɗaukuwa da kundin Linux suna sabunta kansu. `.deb`,
`.rpm`, AppImage da Flatpak ba za su iya maye gurbin fayilolinsu ba, don haka
MixMusic yana gaya maka idan sabon sigar ta fito kuma yana zazzage maka ita.
`mixmusic-bin` na AUR yana sabuntawa tare da sauran tsarinka. Don zip mai
ɗaukuwa, buɗe shi a ko ina sannan ka buɗe `MixMusic.exe`. Don AppImage, mayar
da shi mai aiwatarwa (`chmod +x`) sannan ka buɗe shi. Don Flatpak, buɗe shi da
cibiyar manhajojin ka, ko ka gudanar da
`flatpak install ./mixmusic-<version>-linux-<arch>.flatpak`; a karon farko,
Flatpak yana ɗauko GNOME runtime daga Flathub.

A Mac, [Homebrew](https://brew.sh)
yana shigar da MixMusic, kuma `brew upgrade` yana sabunta shi.

```sh
brew install --cask fzn0x/tap/mixmusic
```

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
- **Kowane tsari:** MixMusic yana samun sautin yawancin waƙoƙin YouTube Music
  da kansa. Don sauran, kamar waƙoƙin da aka yi wa yara, yana amfani da
  [yt-dlp](https://github.com/yt-dlp/yt-dlp), wanda yake shigar maka daga
  Settings › Playback (Saituna › Kunnawa) kuma yake sabuntawa. yt-dlp yana
  buƙatar JavaScript runtime: [Deno](https://deno.com), Node.js, Bun ko
  QuickJS. Flatpak yana da Node.js.

### Flatpak

Flatpak yana gudanar da MixMusic a cikin keɓaɓɓen wuri, wanda yake ganin
hanyar sadarwa, sabar sauti, allo da GPU, amma ba ya ganin babban fayil ɗin
gidanka ko kaɗan. MixMusic yana adana saitunansa da ɗakin waƙoƙinsa a
`~/.var/app/io.github.fzn0x.mixmusic`, a keɓe daga kwafin `.deb`, `.rpm`,
AppImage ko kundi, kuma duka biyun na iya gudana a lokaci guda. Don kawo
saituna da ɗakin waƙoƙin wani kwafi, rufe duka biyun, kwafi `settings.json`,
`library.json`, `queue.json` da babban fayil ɗin `addons` daga
`~/.config/MixMusic` zuwa `~/.var/app/io.github.fzn0x.mixmusic/config/MixMusic`,
sannan ka sake shiga asusunka: shigarwa suna nan a keyring na ɗayan kwafin.

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
  (Saituna › Asusu) sannan ka shiga Google a taga da ta buɗe. Abin da ka rubuta
  yana tafiya zuwa Google kaɗai. MixMusic yana adana zaman a ɓoye, da maɓalli
  a keyring na tsarinka idan akwai. Inda wannan taga ba za ta buɗe ba, ko
  Google ta ƙi ta, shiga da burauzarka, wadda MixMusic yake buɗewa da sabuwar
  bayanan martaba nata kuma yake share ta da zarar ka shiga; ko ka shigo da
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
- **Discord:** muddin manhajar Discord tana aiki, bayananka suna nuna abin da
  kake saurara. Kashe shi a Settings › Desktop.

Wani abu ba ya aiki? [docs/troubleshooting.md](docs/troubleshooting.md)
yana ƙunshe da matsalolin da aka fi samu da kuma matsalolin da aka sani.

## Gina daga tushe

Kana buƙatar [Go](https://go.dev/dl/) 1.27.3 ko sama da haka,
[Bun](https://bun.sh) 1.2 ko sama da haka, wanda yake gudanar da layin umarni
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
fitarwa da su, a sigogin da `flake.lock` ya kulle: Go, Bun, NSIS, GoReleaser
da Zola. `nix build` yana gina MixMusic don Linux daga tushe, zuwa cikin
`result/`.

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

[Shafin yanar gizo](https://fzn0x.github.io/mixmusic/) yana cikin `site/`,
an gina shi da [Zola](https://www.getzola.org), kuma jagororin da ke cikin
`docs/` su ne shafukansa na `/docs/`. `go run ./tools/sitedocs && zola --root site serve`
yana nuna shi kafin a buga. Ana gina fitarwa a cikin Nix shell kuma
[GoReleaser](https://goreleaser.com) ne yake bugawa (`.goreleaser.yaml`).

[docs/architecture](docs/architecture/README.md) yana bayyana yadda sassan
suke haɗuwa.

### Tsari

Manhajar ita ce babban package, tare da packages na `internal/` a ƙarƙashinta.
Packages ɗin da suke wajen `internal/` na kowa ne ya shigo da su, kuma an
rubuta su a
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic).

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

Packages ɗin da za ka iya shigowa da su:

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
2. Ƙara babban fayil guda ɗaya: `community/themes/<id>/` don jigo, ko
   `community/plugins/<id>/` don plugin, tare da `plugin.json` da
   `index.ts` ɗinsa.
3. Gudanar da `bun run build:plugins`, `go run ./tools/registry` da
   `go run ./tools/registry -check`.
4. Buɗe pull request.

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
  kashe a Settings › Accounts (Saituna › Asusu), waƙoƙin da kake kunnawa, waɗanda
  suke shiga tarihin YouTube Music ɗinka.
- **Spotify,** da zarar ka shiga: ɗakin waƙoƙinka, naʼurorinka na Spotify
  Connect, da mai kunnawa na Spotify, wanda ake lodawa daga `sdk.scdn.co`,
  idan Spotify yana kunnawa a MixMusic.
- **Unison:** ID ɗin bidiyon waƙar da ake kunnawa, ko, idan babu shi, taken
  waƙar, mawaƙanta, albam ɗinta da tsawonta, don samo kalmomin waƙarta. Sai
  yayin da ka kunna fassara, ko rubuta da haruffan Latin, a gefen kalmomin
  waƙa, kalmomin waƙar da ake kunnawa, tare da ID ɗin bidiyonta, waɗanda
  Unison yake sa Google ta fassara.
- **LRCLIB:** taken waƙar da ake kunnawa, mawaƙanta, albam ɗinta da tsawonta,
  don samo kalmomin waƙarta.
- **GitHub:** duba sabuwar sigar a farawa da kuma bayan kowane ƴan saʼoʼi,
  wanda za ka iya kashewa a Settings › About (Saituna › Game da); rajistar
  alʼumma idan ka buɗe kasuwa; abubuwan zazzagewa na yt-dlp; kuma, a karon
  farko da ka kunna romaji a gefen kalmomin waƙar Japananci, ƙamus ɗin
  karatun kanji (11.7 MB). Ana yin romaji a kwamfutarka: ba a aika kalmomin
  waƙar.
- **Last.fm,** da zarar ka haɗa shi: abin da kake kunnawa.
- **Discord,** sai dai ka kashe shi: abin da ake kunnawa, ana aika shi zuwa
  manhajar Discord a kwamfutarka, wadda take nuna shi a bayananka.

Abin da yake adanawa a kwamfutarka:

- Saitunanka da ɗakin waƙoƙinka (waƙoƙin da ka so da waɗanda ka ƙi, mixes,
  tarihi da binciken kwanan nan), a babban fayil ɗin da Settings › About ya
  ambata.
- Waƙoƙin da ka kunna, a cikin maʼajiya, gwargwadon abin da ka yarda a
  Settings › Playback, da yadda YouTube ya auna ƙararsu.
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
- **[Unison](https://unison.betterlyrics.org)**, buɗaɗɗen maʼajiyar kalmomin
  waƙa da jamaʼa ke bayarwa, a ƙarƙashin ODbL, da **[LRCLIB](https://lrclib.net)**,
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
- **[FFmpeg](https://ffmpeg.org)**, wanda gwaje-gwajen decoders suke kwatantawa
  da shi.
- **[Selawik](https://github.com/microsoft/Selawik)**,
  **[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily)**,
  **[Roboto Flex](https://github.com/googlefonts/roboto-flex)** da
  **[Archivo](https://github.com/Omnibus-Type/Archivo)**, haruffan tagogi.
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities)**,
  wanda launukan jigogin Material fassarar kimiyyar launi ta HCT ɗinsa ne.
- **[Kevin MacLeod](https://incompetech.com)**, wanda kiɗansa, a ƙarƙashin
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), yake kunnawa a
  shafin yanar gizo.

## Gudummawa

Gyare-gyare, sabbin siffofi, jigogi, plugin da rahotannin kurakurai duka ana
maraba da su.

- **Ba da rahoton matsala** daga Settings › About › report a problem
  (Saituna › Game da › ba da rahoton matsala). Yana buɗe sabon issue na GitHub
  da sigarka, tsarinka, jigonka da plugin ɗinka a cike. Ko kuma
  [buɗe issue](https://github.com/fzn0x/mixmusic/issues/new) da kanka.
- **Aika canji** a matsayin pull request. [CONTRIBUTING.md](CONTRIBUTING.md)
  yana bayyana gwaje-gwajen da za a gudanar, salon commit, da shigar jerin
  canje-canje da canjin da mutane za su lura da shi yake buƙata.
- **Raba jigo ko plugin:** duba [Ƙungiya](#ƙungiya).

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
[lasisin Apache ɗinsa](internal/hct/LICENSE.material-color-utilities).

MixMusic ba shi da alaƙa da Spotify ko Google, kuma ba su amince da shi ko
sun ɗauki nauyinsa ba. Yana kunna abin da waɗannan ayyuka suke bai wa
masu amfani da su, ta hanyoyin ayyukan da kansu; yi amfani da shi yadda
sharuɗansu suka yarda. Spotify da YouTube Music alamun kasuwanci ne na
masu su.
