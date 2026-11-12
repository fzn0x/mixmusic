<p align="center"><a href="README.md">English</a> · <strong>हिन्दी</strong> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ur.md">اردو</a> · <a href="README.ha.md">Hausa</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.bn.md">বাংলা</a> · <a href="README.ru.md">Русский</a> · <a href="README.am.md">አማርኛ</a></p>

<p align="center">
  <img src="brand/icon.png" width="112" height="112" alt="MixMusic का आइकन: काले रंग पर दो गुलाबी छल्ले और एक प्ले बटन" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center"><strong>YouTube Music और Spotify के लिए एक ही प्लेयर।</strong></p>

<p align="center">macOS, Windows और Linux के लिए YouTube Music और Spotify क्लाइंट। YouTube Music बिना अकाउंट के चलता है। एक सर्च, एक क्यू, मिली-जुली playlist, सिंक होने वाले lyrics, थीम और प्लगइन।</p>

<p align="center"><sub>यह <a href="README.md">अंग्रेज़ी README</a> का अनुवाद है। जहाँ दोनों में फ़र्क़ हो, वहाँ अंग्रेज़ी वाला ताज़ा और सही है।</sub></p>

<p align="center">
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml/badge.svg" alt="वेबसाइट" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases/latest"><img src="https://img.shields.io/github/v/release/fzn0x/mixmusic" alt="नवीनतम रिलीज़" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases"><img src="https://img.shields.io/github/downloads/fzn0x/mixmusic/total" alt="डाउनलोड" /></a>
  <a href="https://pkg.go.dev/github.com/fzn0x/mixmusic"><img src="https://pkg.go.dev/badge/github.com/fzn0x/mixmusic.svg" alt="Go Reference" /></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/fzn0x/mixmusic" alt="Go का संस्करण" /></a>
  <a href="https://awesomemygo.dev/apps/mixmusic"><img src="https://awesomemygo.dev/badge.svg" height="20" alt="Awesome MyGo में सूचीबद्ध" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey" alt="macOS, Windows और Linux" />
  <a href="#लाइसेंस"><img src="https://img.shields.io/badge/license-MIT%20or%20GPL--3.0-blue" alt="MIT या GPL-3.0" /></a>
</p>

<p align="center">
  <a href="https://fzn0x.github.io/mixmusic/">वेबसाइट</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">डाउनलोड</a> ·
  <a href="https://fzn0x.github.io/mixmusic/docs/">दस्तावेज़</a> ·
  <a href="CHANGELOG.md">बदलावों की सूची</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic 0.5.9 अपने डिफ़ॉल्ट रूप में, Mix थीम के साथ Blend लेआउट, गहरे मंच पर सीधा खड़ा और आपकी ओर मुँह किए हुए: बीच में YOASOBI का アイドル चल रहा है, गुलाबी किनारी के साथ, उसके हर lyrics की पंक्ति के नीचे romaji और सामने मिनी प्लेयर; बाईं ओर एक सर्च और दाईं ओर लाइब्रेरी के mix" />
</p>

## MixMusic क्यों

अगर आपका संगीत YouTube Music और Spotify में बँटा है, तो आमतौर पर आपको
दो ऐप चाहिए होते हैं, यानी दो सर्च, दो क्यू और दो लाइब्रेरी। MixMusic दोनों
सेवाओं को एक ही प्लेयर में ले आता है:

- **एक सर्च।** यह YouTube Music और Spotify में एक साथ खोजता है, और हर
  नतीजे पर उसकी सेवा का निशान लगाता है।
- **एक क्यू और एक लाइब्रेरी।** Spotify का गाना YouTube Music के गाने के ठीक
  बाद चल सकता है, और एक playlist में दोनों के गाने रह सकते हैं।
- **अकाउंट की ज़रूरत नहीं।** ऐप खोलते ही YouTube Music चलने लगता है। साइन इन
  तभी करें जब आपको अपनी लाइब्रेरी चाहिए। Spotify वैकल्पिक है।
- **छोटा प्रोग्राम।** MixMusic एक ही Go प्रोग्राम है जो अपनी विंडो GPU पर
  खुद बनाता है। इसके अंदर कोई ब्राउज़र नहीं है जिसे डाउनलोड करना पड़े या
  मेमोरी में रखना पड़े।
- **अपने हिसाब से बदलें।** चार लेआउट और चौदह थीम साथ आते हैं, समुदाय और
  थीम तथा प्लगइन जोड़ता है, और हर कमांड को आप अपनी कुंजी दे सकते हैं।
- **निजी।** MixMusic का अपना कोई अकाउंट, सर्वर, analytics या telemetry
  नहीं है।
- **मुफ़्त और ओपन सोर्स,** MIT लाइसेंस या GPL 3.0 के तहत।

## सुविधाएँ

### खोजें और चलाएँ

- **दोनों सेवाओं के लिए एक सर्च।** YouTube Music और Spotify के नतीजे साथ-साथ
  दिखते हैं, हर एक पर उसकी सेवा का निशान होता है। YouTube Music के नतीजों में
  दिखता है कि हर गाना कितनी बार चला, ताकि मूल गाना कवर से अलग पहचाना जाए।
- **एक क्यू।** दोनों सेवाओं के गाने मिलाएँ, उन सबमें शफ़ल और रिपीट करें, और
  क्यू खत्म होने पर रेडियो को आगे चलने दें।
- **Explore (खोजें)।** नई रिलीज़, ट्रेंडिंग गाने, चार्ट, मूड और जॉनर, और जो
  चल रहा है उससे जुड़े गाने।
- **सिंक होने वाले lyrics** जो गाने के साथ चलते हैं, या चाहें तो सादा टेक्स्ट।
- **मिनी प्लेयर** जो आपकी बाकी विंडो के ऊपर बना रहता है।

### आपकी लाइब्रेरी

- **Mix:** ऐसी playlist जिनमें हर सेवा के गाने रह सकते हैं। किसी सेवा की
  playlist को mix में इंपोर्ट करें, और किसी भी mix के अंदर सर्च और सॉर्ट करें।
- **पसंद किए गए गाने** दोनों सेवाओं से, एक ही सूची में।
- **हिस्ट्री और सुनने के आँकड़े,** दिन-दर-दिन।
- **आपके अकाउंट।** अपने होम फ़ीड, पसंद किए गए गाने, playlist, एल्बम और
  कलाकारों के लिए Google के अपने पेज पर YouTube Music में साइन इन करें, अपने
  अकाउंट से या उसके किसी ब्रांड चैनल से। वहाँ की अपनी लाइब्रेरी के लिए Spotify
  में साइन इन करें। लाइक, सेव किए गए एल्बम और playlist, और फ़ॉलो किए गए
  कलाकार सेवा तक भी पहुँचते हैं। आप जो सुनते हैं वह आपकी YouTube Music
  हिस्ट्री में जुड़ता है, जिससे उसके सुझाव बनते हैं।

### Spotify, तीन तरीकों से

- **सीधे MixMusic में,** Spotify की क्वालिटी पर, बिना Spotify ऐप खोले
  (Premium, Windows और macOS पर)।
- **Spotify Connect पर:** आपका Spotify ऐप, आपका फ़ोन या कोई स्पीकर।
- **YouTube Music से:** MixMusic Spotify के गाने की रिकॉर्डिंग YouTube Music
  पर चलाता है, जिसके लिए Premium नहीं चाहिए।

### आवाज़

- क्रॉसफ़ेड, एक जैसी लाउडनेस और वॉल्यूम लेवलिंग।
- प्रीसेट के साथ दस बैंड का इक्वलाइज़र।
- एक विज़ुअलाइज़र।
- आपके सुने गए गानों का ऑफ़लाइन कैश।

### आपके डेस्कटॉप पर

- Windows, macOS और Linux पर मीडिया कुंजियाँ और सिस्टम के मीडिया कंट्रोल।
- प्ले, पॉज़, स्किप, वॉल्यूम बदलने, म्यूट, चल रहे गाने को लाइक या डिसलाइक करने
  या MixMusic दिखाने के लिए आपके अपने ग्लोबल शॉर्टकट, चाहे कोई भी विंडो
  सामने हो।
- Windows में टास्कबार थंबनेल पर प्लेयर के बटन।
- ट्रे कंट्रोल, नए गानों की सूचनाएँ, और लॉगिन पर शुरू होना।
- `mixmusic://` लिंक।
- **Last.fm:** आपका सुना हर गाना आपकी प्रोफ़ाइल में जुड़ता है, और ऑफ़लाइन
  हुए scrobble ऑनलाइन लौटते ही भेज दिए जाते हैं।
- **Discord:** आपकी प्रोफ़ाइल दिखाती है कि क्या चल रहा है, उसकी आर्टवर्क और
  बचे हुए समय के साथ।
- **अपडेट:** MixMusic शुरू होने पर और हर कुछ घंटों में नई रिलीज़ देखता है,
  उसका सिग्नेचर जाँचता है, और जब आप कहें तब उसे इंस्टॉल करता है, या आप
  चुनें तो अपने आप।

### इसे अपना बनाएँ

- **लेआउट।** Blend, यानी डिफ़ॉल्ट, में बड़े कवर की घनी पंक्तियों पर गोल आकार
  और उछलती हुई गति है। Expressive, Material 3 Expressive की तर्ज़ पर, चंचल है,
  टोनल फ़िल, पिल बटन और तैरते प्लेयर बार के साथ। Crate संयमित है, ताकि कवर
  ही काम करें, और Tiles MixMusic का पहला रूप है, जिसमें सपाट टाइल और
  विशाल छोटे अक्षरों के शब्द हैं।
- **थीम।** चौदह बिल्ट-इन, हर एक किसी भी लेआउट में, ट्यून किए जा सकने वाले
  इफ़ेक्ट के साथ, लाइट मोड के लिए एक थीम और डार्क मोड के लिए एक, और आपका
  अपना या चल रही आर्टवर्क से लिया गया एक्सेंट रंग। Material और Material
  Dark हर रंग कवर से बनाते हैं।
- **समुदाय की थीम और प्लगइन,** मार्केटप्लेस में बस एक क्लिक दूर।
- **प्लगइन** एक टाइप्ड API के ज़रिए पेज, कमांड, गाने की क्रियाएँ और विजेट
  जोड़ते हैं, जो आपकी थीम में बने दिखते हैं।
- **कीबोर्ड पहले।** हर आम काम के लिए शॉर्टकट, किसी भी कमांड के लिए आपकी अपनी
  कुंजी, प्लगइन की कमांड समेत, किसी भी ऐप से संगीत चलाने वाली कुंजियाँ, और
  बाकी सब के लिए कमांड पैलेट (<kbd>Ctrl</kbd> <kbd>K</kbd>)।
- **एक सेटिंग फ़ाइल।** आपकी सेटिंग आपके डेटा फ़ोल्डर में `settings.json` है
  (Windows पर `%AppData%\MixMusic`, macOS पर
  `~/Library/Application Support/MixMusic`, Linux पर `~/.config/MixMusic`),
  जिसे आप अपनी dotfiles के साथ रख सकते हैं। MixMusic चलते समय इसे सेव करें,
  और यह तुरंत लागू हो जाती है; Settings › Desktop › Settings file
  (सेटिंग › डेस्कटॉप › सेटिंग फ़ाइल) इसे खोलता है।

## एक नज़र

<p align="center">
  <img src="docs/look-listen.webp" alt="Blend लेआउट में MixMusic के पेज, गहरे मंच पर सीधे खड़े और आपकी ओर मुँह किए हुए: बीच में YOASOBI का アイドル चल रहा है, गुलाबी किनारी के साथ, उसके lyrics की हर पंक्ति के नीचे romaji और अंग्रेज़ी अनुवाद; बाईं ओर एल्बम THE BOOK 3 और दाईं ओर फ़ॉलो किए गए एक पॉडकास्ट का पेज" />
</p>

<p align="center"><sub><b>सुनें।</b> बीच में YOASOBI का アイドル चल रहा है: हर पंक्ति गाते समय रोशन होती है, उसके नीचे romaji और अनुवाद के साथ। बाईं ओर एक एल्बम; दाईं ओर आपका फ़ॉलो किया हुआ एक पॉडकास्ट।</sub></p>

<p align="center">
  <img src="docs/look-yours.webp" alt="MixMusic के और हिस्से, गहरे मंच पर सीधे खड़े और आपकी ओर मुँह किए हुए: बीच में Settings › Appearance (सेटिंग › दिखावट) चारों लेआउट के साथ, गुलाबी किनारी में; बाईं ओर चल रहे कवर से रंगी Material थीम और दाईं ओर मार्केटप्लेस की समुदाय की थीम" />
</p>

<p align="center"><sub><b>इसे अपना बनाएँ।</b> बीच में चारों लेआउट: Blend, यानी डिफ़ॉल्ट, Expressive, Crate और Tiles, थीम के नीचे। बाईं ओर एक Material थीम जो चल रहे कवर से अपने रंग लेती है। दाईं ओर मार्केटप्लेस में समुदाय की थीम।</sub></p>

## इंस्टॉल करें

[नवीनतम रिलीज़](https://github.com/fzn0x/mixmusic/releases/latest) से अपने सिस्टम
की फ़ाइल डाउनलोड करें:

| सिस्टम | डाउनलोड |
|---|---|
| macOS 12 और उसके बाद (Apple silicon और Intel) | [`mixmusic-<version>-macos-universal.dmg`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows 10 और 11 (x64, Arm), इंस्टॉलर | [`mixmusic-<version>-windows-<arch>-setup.exe`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows, पोर्टेबल, बिना इंस्टॉल किए | [`mixmusic-<version>-windows-<arch>-portable.zip`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Debian, Ubuntu और उन पर आधारित | [`mixmusic-<version>-linux-<arch>.deb`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Fedora, openSUSE और उन पर आधारित | [`mixmusic-<version>-linux-<arch>.rpm`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Arch Linux और उस पर आधारित (जल्द आ रहा है) | AUR पर [`mixmusic-bin`](https://aur.archlinux.org/packages/mixmusic-bin): `yay -S mixmusic-bin` |
| कोई भी Linux, एक फ़ाइल | [`mixmusic-<version>-linux-<arch>.AppImage`](https://github.com/fzn0x/mixmusic/releases/latest) |
| कोई भी Linux, सैंडबॉक्स में (Flatpak) | [`mixmusic-<version>-linux-<arch>.flatpak`](https://github.com/fzn0x/mixmusic/releases/latest) |
| कोई भी Linux, आपके होम फ़ोल्डर में | [`mixmusic-<version>-linux-<arch>.tar.gz`](https://github.com/fzn0x/mixmusic/releases/latest), `install.sh` के साथ |

`<arch>` x64 के लिए `amd64` और Arm के लिए `arm64` है। हर रिलीज़ में
`SHA256SUMS` दिया गया है, जिससे आप डाउनलोड जाँच सकते हैं। `update-…`,
`….delta`, `mixmusic-<version>-macos-…` या `-windows-<arch>.tar.gz` नाम की
फ़ाइलें आपको नहीं चाहिए: इंस्टॉल किए गए ऐप इन्हीं से अपडेट होते हैं।

इंस्टॉलर, पोर्टेबल zip और Linux आर्काइव खुद को अपडेट करते हैं। `.deb`, `.rpm`,
AppImage और Flatpak अपनी फ़ाइलें खुद नहीं बदल सकते, इसलिए नया संस्करण आने पर
MixMusic आपको बताता है और उसे आपके लिए डाउनलोड कर देता है। AUR का
`mixmusic-bin` आपके बाकी सिस्टम के साथ अपडेट होता है। पोर्टेबल zip के लिए, उसे
कहीं भी खोलें और `MixMusic.exe` चलाएँ। AppImage के लिए, उसे executable बनाएँ
(`chmod +x`) और चलाएँ। Flatpak के लिए, उसे अपने सॉफ़्टवेयर सेंटर से खोलें, या
`flatpak install ./mixmusic-<version>-linux-<arch>.flatpak` चलाएँ; पहली बार
Flatpak, Flathub से GNOME runtime लाता है।

जल्द आ रहा है, जैसे ही इसका tap तैयार होगा: Mac पर [Homebrew](https://brew.sh)
MixMusic इंस्टॉल करता है, और `brew upgrade` उसे अपडेट करता है।

```sh
brew install --cask fzn0x/tap/mixmusic
```

### पहली बार चलाना

- **macOS:** ऐप को अभी Apple से notarize नहीं कराया गया है, इसलिए पहली बार
  खोलने पर macOS कहता है कि वह इसे जाँच नहीं सकता। इसे फिर भी खोलें: System
  Settings › Privacy & Security › Open Anyway (सिस्टम सेटिंग › गोपनीयता और
  सुरक्षा › फिर भी खोलें) से, या Terminal में यह एक बार चलाएँ:

  ```sh
  xattr -dr com.apple.quarantine /Applications/MixMusic.app
  ```
- **Windows:** इंस्टॉलर पर अभी साइन नहीं है, इसलिए SmartScreen चेतावनी दे सकता
  है: More info चुनें, फिर Run anyway। YouTube Music में साइन इन करने और
  MixMusic में Spotify चलाने के लिए WebView2 Runtime चाहिए, जो Windows 11 में
  पहले से है।
- **Linux:** MixMusic को GTK 3 चाहिए, और PulseAudio (PipeWire भी उसे चलाता है)
  या ALSA। YouTube Music में साइन इन करने के लिए WebKitGTK 4.1 चाहिए, ट्रे आइकन
  के लिए `libayatana-appindicator3`, और आपके साइन-इन को सिस्टम की keyring में रखने
  के लिए `libsecret`। `.deb` और `.rpm` इन्हें इंस्टॉल कर देते हैं, और Flatpak
  इन्हें साथ लाता है; AppImage और आर्काइव के लिए:

  ```sh
  # Debian and Ubuntu
  sudo apt install libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
  # Fedora
  sudo dnf install gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
  ```
- **हर सिस्टम:** MixMusic YouTube Music के ज़्यादातर गानों का ऑडियो खुद ढूँढ लेता
  है। बाकी के लिए, जैसे बच्चों के लिए बने गाने, यह
  [yt-dlp](https://github.com/yt-dlp/yt-dlp) का इस्तेमाल करता है, जिसे यह
  Settings › Playback (सेटिंग › प्लेबैक) से आपके लिए इंस्टॉल करता है और अपडेट
  रखता है। yt-dlp को एक JavaScript runtime चाहिए: [Deno](https://deno.com),
  Node.js, Bun या QuickJS। Flatpak में Node.js है।

### Flatpak

Flatpak, MixMusic को एक सैंडबॉक्स में चलाता है, जिसे नेटवर्क, साउंड सर्वर,
डिस्प्ले और GPU दिखते हैं, और आपका होम फ़ोल्डर बिल्कुल नहीं। MixMusic अपनी
सेटिंग और लाइब्रेरी `~/.var/app/io.github.fzn0x.mixmusic` में रखता है, जो
`.deb`, `.rpm`, AppImage या आर्काइव वाली कॉपी से अलग है, और दोनों एक साथ चल सकती
हैं। किसी कॉपी की सेटिंग और लाइब्रेरी लाने के लिए, दोनों बंद करें,
`~/.config/MixMusic` से `settings.json`, `library.json`, `queue.json` और
`addons` फ़ोल्डर को
`~/.var/app/io.github.fzn0x.mixmusic/config/MixMusic` में कॉपी करें, और अपने
अकाउंट में दोबारा साइन इन करें: साइन-इन दूसरी कॉपी की keyring में ही रहते हैं।

Flatpak में, लॉगिन पर शुरू होना आपके डेस्कटॉप से होकर जाता है, जो पहले आपसे
पूछ सकता है, और आपका अपना yt-dlp `~/.var/app/io.github.fzn0x.mixmusic` में
होना चाहिए, क्योंकि सैंडबॉक्स को कोई और फ़ोल्डर नहीं दिखता। जो Discord ऐप
Flatpak नहीं है, वह चल रहा गाना तभी दिखाता है जब MixMusic के शुरू होते समय वह
चल रहा हो, वरना उसे दोबारा शुरू करने तक; Flatpak वाला Discord दोनों तरह से
काम करता है। Flatpak नया है: किसी [issue](https://github.com/fzn0x/mixmusic/issues)
में हमें बताएँ कि यह आपके डेस्कटॉप पर कैसा चलता है।

### अकाउंट (सभी वैकल्पिक)

- **YouTube Music:** Settings › Accounts (सेटिंग › अकाउंट) में Sign in to
  YouTube Music चुनें और खुलने वाली विंडो में Google में साइन इन करें। आप जो
  टाइप करते हैं वह सिर्फ़ Google को जाता है। MixMusic सेशन को एन्क्रिप्ट करके
  रखता है, और जहाँ keyring है वहाँ उसकी कुंजी आपके सिस्टम की keyring में रखता
  है। जहाँ वह विंडो खुल नहीं सकती, या Google उसे मना कर देता है, वहाँ अपने
  ब्राउज़र से साइन इन करें, जिसे MixMusic अपनी एक नई प्रोफ़ाइल के साथ खोलता है
  और आपके अंदर आते ही मिटा देता है; या उस ब्राउज़र का सेशन इंपोर्ट करें जिसमें
  आपने साइन इन किया है (MixMusic उसे अपने yt-dlp से पढ़ता है, और सिर्फ़ YouTube
  की कुकी रखता है); या साइन इन किए हुए music.youtube.com टैब का Cookie हेडर
  पेस्ट करें।
- **Spotify:** Spotify एक ऐप को पाँच उपयोगकर्ताओं तक ही चलाने देता है, और सिर्फ़
  तब तक जब तक उसके मालिक के पास Premium है, इसलिए आप अपने ही बनाए Spotify ऐप
  से साइन इन करते हैं। इसमें दो मिनट लगते हैं: [docs/spotify.md](docs/spotify.md)
  आपको पूरा तरीका बताता है।
- **Last.fm:** [last.fm/api/account/create](https://www.last.fm/api/account/create)
  पर एक API अकाउंट बनाएँ (कोई भी नाम, कोई callback URL नहीं), उसकी API key और
  shared secret को Settings › Accounts में पेस्ट करें, और अपने ब्राउज़र में
  MixMusic को मंज़ूरी दें।
- **Discord:** जब तक Discord ऐप चल रहा है, आपकी प्रोफ़ाइल दिखाती है कि आप क्या
  सुन रहे हैं। इसे Settings › Desktop में बंद करें।

कुछ काम नहीं कर रहा? [docs/troubleshooting.md](docs/troubleshooting.md) में आम
समस्याएँ और ज्ञात दिक्कतें हैं।

## सोर्स से बनाएँ

आपको [Go](https://go.dev/dl/) 1.27.3 या उसके बाद का, [Bun](https://bun.sh) 1.2
या उसके बाद का, जो MyGo की कमांड लाइन चलाता है, और Git चाहिए। MyGo को C
कंपाइलर नहीं चाहिए, इसलिए कोई भी सिस्टम हर सिस्टम के लिए MixMusic बना सकता है।

### Windows

```powershell
winget install GoLang.Go
winget install Oven-sh.Bun
winget install Git.Git
```

नया टर्मिनल खोलें, फिर:

```powershell
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

साइन इन करने और Spotify चलाने के लिए WebView2 Runtime चाहिए, जो Windows 11 में
पहले से है। Windows 10 पर उसे
[Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/) से
इंस्टॉल करें।

### macOS

[Homebrew](https://brew.sh) के साथ:

```sh
brew install go oven-sh/bun/bun
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Linux

Go को [go.dev/dl](https://go.dev/dl/) से इंस्टॉल करें (डिस्ट्रीब्यूशन के पैकेज
अक्सर 1.27.3 से पुराने होते हैं), फिर Bun और वे लाइब्रेरी जिनके साथ MixMusic
चलता है:

```sh
curl -fsSL https://bun.sh/install | bash
# Debian and Ubuntu
sudo apt install git libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
# Fedora
sudo dnf install git gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
```

फिर:

```sh
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Nix

[Nix](https://nixos.org) के साथ, `nix develop` आपको वे टूल देता है जिनसे
रिलीज़ बनती हैं, उन संस्करणों में जिन्हें `flake.lock` पिन करता है: Go, Bun,
NSIS, GoReleaser और Zola। `nix build` MixMusic को Linux के लिए सोर्स से
`result/` में बनाता है।

### विकास करें

`bun run dev` ऐप बनाता है, उसे शुरू करता है, और आपके बदलाव करते ही उसे दोबारा
बनाता है। इसका डेटा इंस्टॉल किए गए MixMusic से अलग रहता है, `MixMusic Dev`
नाम से।

```sh
bun run build:plugins          # the plugins' JavaScript, from their TypeScript
go test ./...                  # the app's tests
bun run typecheck              # the plugin SDK and the community's plugins
go run ./tools/registry -check # the community registry
bun run build                  # the packaged app for this system, in build/
bun run build -- -platform windows/amd64,linux/amd64,linux/arm64
```

Nix शेल में, Linux के डाउनलोड वैसे ही बनाएँ जैसे रिलीज़ बनती हैं, यानी
`.github/scripts/build.sh linux/amd64,linux/arm64` से। वहाँ सादा `bun run build`
ऐसा Linux प्रोग्राम बनाता है जो Nix का अपना loader माँगता है, और वह सिर्फ़ NixOS
पर शुरू होता है। `go run ./tools/elfloader build/linux-amd64` किसी बिल्ड की
जाँच करता है।

[वेबसाइट](https://fzn0x.github.io/mixmusic/) `site/` में है, जो
[Zola](https://www.getzola.org) से बनी है, और `docs/` की गाइड उसके `/docs/`
पेज हैं। `go run ./tools/sitedocs && zola --root site serve` उसका प्रीव्यू
दिखाता है। रिलीज़ Nix शेल में बनती हैं और [GoReleaser](https://goreleaser.com)
(`.goreleaser.yaml`) से प्रकाशित होती हैं।

[docs/architecture](docs/architecture/README.md) बताता है कि हिस्से आपस में कैसे
जुड़ते हैं।

### संरचना

ऐप रूट पैकेज है, और उसके नीचे `internal/` के पैकेज हैं। `internal/` के बाहर के
पैकेज किसी के भी import करने के लिए हैं, और
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic) पर documented हैं।

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

जो पैकेज आप import कर सकते हैं:

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

## समुदाय

कोई भी मार्केटप्लेस से समुदाय की थीम और प्लगइन इंस्टॉल कर सकता है, जो साइडबार में
स्टोर का आइकन है, और आप अपनी भी जोड़ सकते हैं।

### थीम बनाएँ

थीम एक `theme.json` फ़ाइल है जो रंग, फ़ॉन्ट और कोने तय करती है, जो उपयोगकर्ता के
चुने लेआउट में दिखते हैं। [docs/themes.md](docs/themes.md) हर टोकन गिनाता है और
एक पूरी थीम दिखाता है। अपनी थीम आज़माने के लिए, उसका फ़ोल्डर थीम फ़ोल्डर में रखें
(Settings › Appearance › open themes folder (सेटिंग › दिखावट › थीम फ़ोल्डर खोलें))
और उसे Settings › Appearance में चुनें।

### प्लगइन बनाएँ

प्लगइन एक छोटा मॉड्यूल है, जो TypeScript या JavaScript में लिखा जाता है और पेज,
कमांड, गाने की क्रियाएँ और विजेट जोड़ता है। [docs/plugins.md](docs/plugins.md)
API को documented करता है, आपके एडिटर के लिए उसके [types](sdk/index.d.ts) के साथ।
समुदाय के [sleep timer](community/plugins/sleep-timer) और
[spectrum](community/plugins/spectrum) चलते-फिरते उदाहरण हैं।

### इसे साझा करें

1. इस रिपॉज़िटरी को fork करें।
2. एक फ़ोल्डर जोड़ें: थीम के लिए `community/themes/<id>/`, या प्लगइन के लिए
   `community/plugins/<id>/`, उसकी `plugin.json` और उसकी `index.ts` के साथ।
3. `bun run build:plugins`, `go run ./tools/registry` और
   `go run ./tools/registry -check` चलाएँ।
4. एक pull request खोलें।

मर्ज होने के बाद, CI उसे
[registry branch](https://github.com/fzn0x/mixmusic/tree/registry) पर प्रकाशित
करता है, और वह सबके मार्केटप्लेस में दिखने लगती है।
[community/README.md](community/README.md) में ब्योरा है।

## गोपनीयता

MixMusic का अपना कोई अकाउंट, सर्वर, analytics या telemetry नहीं है। यह इनसे बात
करता है:

- **YouTube और Google:** YouTube Music का कैटलॉग, उसकी स्ट्रीम और हर गाना कितना
  तेज़ है, साइन आउट रहते हुए माँगा गया, और आर्टवर्क; कुछ स्ट्रीम yt-dlp के
  ज़रिए। साइन इन रहने पर, वहाँ की आपकी लाइब्रेरी, और, जब तक आप Settings ›
  Accounts (सेटिंग › अकाउंट) में इसे बंद न करें, आपके सुने गाने, जो आपकी
  YouTube Music हिस्ट्री में जुड़ते हैं।
- **Spotify,** साइन इन करने के बाद: आपकी लाइब्रेरी, आपके Spotify Connect
  डिवाइस, और Spotify का प्लेयर, जो `sdk.scdn.co` से लोड होता है, जब MixMusic
  में Spotify चलता है।
- **Unison:** चल रहे गाने की video ID, या उसके बिना, उसका शीर्षक, कलाकार, एल्बम
  और अवधि, उसके lyrics खोजने के लिए। सिर्फ़ तब जब आप lyrics के पास translate
  या romanize चालू करें, चल रहे गाने के lyrics, उसकी video ID के साथ, जिन्हें
  Unison, Google से अनुवाद कराता है।
- **LRCLIB:** चल रहे गाने का शीर्षक, कलाकार, एल्बम और अवधि, उसके lyrics खोजने
  के लिए।
- **GitHub:** शुरू होने पर और हर कुछ घंटों में नए संस्करण की जाँच, जिसे आप
  Settings › About (सेटिंग › परिचय) में बंद कर सकते हैं; जब आप मार्केटप्लेस
  खोलते हैं तब समुदाय की registry; yt-dlp के डाउनलोड; और, जब आप पहली बार जापानी
  lyrics के पास romaji चालू करते हैं, तब kanji के उच्चारण की डिक्शनरी (11.7 MB)।
  Romaji आपके कंप्यूटर पर बनता है: lyrics भेजे नहीं जाते।
- **Last.fm,** जोड़ने के बाद: आप क्या सुनते हैं।
- **Discord,** जब तक आप इसे बंद न करें: जो चल रहा है, आपके कंप्यूटर के Discord
  ऐप को भेजा जाता है, जो उसे आपकी प्रोफ़ाइल पर दिखाता है।

यह आपके कंप्यूटर पर क्या रखता है:

- आपकी सेटिंग और लाइब्रेरी (पसंद और नापसंद किए गाने, mix, हिस्ट्री और हाल की
  सर्च), उस फ़ोल्डर में जिसका नाम Settings › About बताता है।
- आपके सुने गाने, कैश में, उतने जितना आप Settings › Playback (सेटिंग ›
  प्लेबैक) में allow करें, और YouTube ने उन्हें कितना तेज़ नापा।
- आपके देखे कवर, 256 MB तक।
- आपके चालू किए lyrics के अनुवाद, गाने और भाषा के हिसाब से, और वह डिक्शनरी जिससे
  romaji, kanji पढ़ता है, एक बार चालू करने पर।
- आपके साइन-इन, YouTube Music का सेशन भी, सिस्टम की keyring में। Windows पर,
  जब Credential Manager किसी को नहीं रख पाता, तो वह आपके Windows अकाउंट के लिए
  एन्क्रिप्ट की गई फ़ाइल में जाता है; बिना keyring वाले Linux पर, ऐसी फ़ाइलों में
  जिन्हें सिर्फ़ आपका उपयोगकर्ता पढ़ सकता है।
- लॉग, आपके साइन-इन के बिना।

लाइब्रेरी का हिस्ट्री पेज आपके सुने गाने मिटाता है, Settings › Playback कैश और
कवर मिटाता है, और किसी सेवा से साइन आउट करने पर उसका साइन-इन मिट जाता है।

Windows पर, विज़ुअलाइज़र इस कंप्यूटर पर चल रहे Spotify का ऑडियो सुनता है, चाहे
MixMusic में हो या Spotify ऐप में, Windows के process loopback से, जो सिर्फ़ उसी
ऐप को सुनता है। macOS 14.2 और बाद के संस्करणों पर, यह MixMusic में चल रहे Spotify
को process tap से सुनता है, आपके allow करने के बाद, जैसा macOS पूछता है। यह ऑडियो
का विश्लेषण मेमोरी में करता है और उसे कभी रिकॉर्ड या भेजता नहीं।

## श्रेय

MixMusic इन प्रोजेक्ट पर बना है:

- **[MyGo](https://github.com/egoist/mygo)**, वह डेस्कटॉप फ़्रेमवर्क जिससे MixMusic
  बना है। MixMusic, MyGo का नेटिव UI इस्तेमाल करता है: MyGo हर विंडो खुद बनाता
  है, Go में, बिना webview के, पेज और प्लेयर बार से लेकर मिनी प्लेयर तक। इसकी
  कमांड लाइन ऐप को इंस्टॉलर और साइन किए हुए अपडेट के साथ पैकेज करती है, जिनसे
  वह खुद को अपडेट करता है।
- **[Go](https://go.dev)**, ऐप की भाषा, [purego](https://github.com/ebitengine/purego)
  के साथ, जो cgo के बिना सिस्टम की लाइब्रेरी को कॉल करता है, और
  [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) और
  [golang.org/x/text](https://pkg.go.dev/golang.org/x/text)।
- **[Sobek](https://github.com/grafana/sobek)**, वह JavaScript इंजन जिसमें हर
  प्लगइन चलता है।
- **[Nix](https://nixos.org)**, जो रिलीज़ बनाने वाले टूल को पिन करता है, और
  MixMusic को Linux के लिए सोर्स से बनाता है।
- **[GoReleaser](https://goreleaser.com)**, जो रिलीज़ प्रकाशित करता है, और उसका
  [nFPM](https://nfpm.goreleaser.com), जो RPM पैकेज बनाता है।
- **[Bun](https://bun.sh)**, जो बिल्ड चलाता है, और
  **[TypeScript](https://www.typescriptlang.org)**, जो प्लगइन को कंपाइल करता है
  और प्लगइन SDK के types जाँचता है।
- **[Zola](https://www.getzola.org)**, जो वेबसाइट बनाता है।
- **[NSIS](https://nsis.sourceforge.io)**, जो Windows के इंस्टॉलर बनाता है, और
  [appimagetool](https://github.com/AppImage/appimagetool), AppImage बनाने वाला।
- **[yt-dlp](https://github.com/yt-dlp/yt-dlp)**, जो उन गानों का ऑडियो ढूँढता है
  जिन्हें MixMusic खुद नहीं ढूँढ पाता।
- **[Unison](https://unison.betterlyrics.org)**, lyrics का खुला, crowdsourced
  डेटाबेस, ODbL के तहत, और **[LRCLIB](https://lrclib.net)**, सिंक होने वाले
  lyrics का खुला डेटाबेस।
- **[kagome](https://github.com/ikawaha/kagome)**, जापानी morphological analyser
  जो romaji के लिए lyrics के kanji पढ़ता है, [kagome-dict](https://github.com/ikawaha/kagome-dict)
  की IPA डिक्शनरी के साथ, जो mecab-ipadic से बनी है, और जिसे MixMusic romaji
  पहली बार चालू होने पर डाउनलोड करता है।
- **[libopus](https://opus-codec.org)**, जिसके डिकोडर का Opus डिकोडर एक पोर्ट है,
  और **[SharpJaad](https://github.com/jimm98y/SharpJaad)**, [JAAD](https://sourceforge.net/projects/jaadec/)
  का C# पोर्ट, जिससे AAC डिकोडर निकला है।
- **[FFmpeg](https://ffmpeg.org)**, जिससे डिकोडरों के टेस्ट तुलना करते हैं।
- **[Selawik](https://github.com/microsoft/Selawik)**,
  **[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily)**,
  **[Roboto Flex](https://github.com/googlefonts/roboto-flex)** और
  **[Archivo](https://github.com/Omnibus-Type/Archivo)**, विंडो के फ़ॉन्ट।
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities)**,
  जिसके HCT कलर साइंस का पोर्ट Material थीम के रंग हैं।
- **[Kevin MacLeod](https://incompetech.com)**, जिनका संगीत,
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) के तहत, वेबसाइट पर
  बजता है।

## योगदान करें

सुधार, नई सुविधाएँ, थीम, प्लगइन और बग रिपोर्ट, सभी का स्वागत है।

- **समस्या बताएँ** Settings › About › report a problem (सेटिंग › परिचय ›
  समस्या बताएँ) से। यह आपके संस्करण, सिस्टम, थीम और प्लगइन भरे हुए एक नया GitHub
  issue खोलता है। या खुद [एक issue खोलें](https://github.com/fzn0x/mixmusic/issues/new)।
- **बदलाव भेजें** pull request के रूप में। [CONTRIBUTING.md](CONTRIBUTING.md)
  बताता है कि कौन-सी जाँचें चलानी हैं, commit की शैली क्या है, और ऐसे बदलाव के
  लिए changelog की कौन-सी प्रविष्टि चाहिए जिसे लोग नोटिस करेंगे।
- **थीम या प्लगइन साझा करें:** देखें [समुदाय](#समुदाय)।

[CONTRIBUTORS.md](CONTRIBUTORS.md) उन लोगों का धन्यवाद करता है जिनके कोड, विचारों
और रिपोर्ट ने MixMusic को आकार दिया।

## लाइसेंस

MixMusic दोहरे लाइसेंस के तहत है: इसे [MIT लाइसेंस](LICENSE-MIT) या
[GNU जनरल पब्लिक लाइसेंस, संस्करण 3](LICENSE-GPL) के तहत इस्तेमाल करें, जो भी
आपको पसंद हो। योगदान दोनों के तहत स्वीकार किए जाते हैं। `fonts` में मौजूद
फ़ॉन्ट SIL Open Font License के तहत हैं: [Selawik के](fonts/OFL.txt),
[Quicksand के](fonts/quicksand-OFL.txt),
[Roboto Flex के](fonts/robotoflex-OFL.txt) और
[Archivo के](fonts/archivo-OFL.txt)। Opus डिकोडर libopus का पोर्ट है,
[उसके BSD लाइसेंस](opus/LICENSE.libopus) के तहत, AAC डिकोडर SharpJaad से निकला
है, [उसके MIT लाइसेंस](aac/LICENSE.sharpjaad) के तहत, और HCT कलर साइंस Material
Color Utilities का पोर्ट है, [उसके Apache लाइसेंस](internal/hct/LICENSE.material-color-utilities)
के तहत।

MixMusic का Spotify या Google से कोई संबंध नहीं है, और न ही इन्होंने इसे
समर्थन या प्रायोजित किया है। यह वही चलाता है जो ये सेवाएँ अपने उपयोगकर्ताओं को
देती हैं, सेवाओं के अपने इंटरफ़ेस के ज़रिए; इसे वैसे ही इस्तेमाल करें जैसा उनकी
शर्तें अनुमति देती हैं। Spotify और YouTube Music अपने-अपने मालिकों के ट्रेडमार्क
हैं।
