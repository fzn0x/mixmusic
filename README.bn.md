<p align="center"><a href="README.md">English</a> · <a href="README.hi.md">हिन्दी</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ur.md">اردو</a> · <a href="README.ha.md">Hausa</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <strong>বাংলা</strong> · <a href="README.ru.md">Русский</a> · <a href="README.am.md">አማርኛ</a></p>

<p align="center">
  <img src="brand/icon.png" width="112" height="112" alt="MixMusic-এর আইকন: কালো পটভূমিতে দুটি গোলাপি বলয় ও একটি প্লে বোতাম" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center"><strong>YouTube Music ও Spotify-এর জন্য একটি প্লেয়ার।</strong></p>

<p align="center">macOS, Windows ও Linux-এর জন্য YouTube Music ও Spotify ক্লায়েন্ট। YouTube Music চলে কোনো অ্যাকাউন্ট ছাড়াই। একটি সার্চ, একটি কিউ, মিশ্র প্লেলিস্ট, সিঙ্ক করা লিরিক্স, থিম ও প্লাগইন।</p>

<p align="center"><sub>এটি <a href="README.md">ইংরেজি README</a>-এর অনুবাদ। দুটির মধ্যে পার্থক্য থাকলে ইংরেজি সংস্করণটিই সর্বশেষ ও সঠিক।</sub></p>

<p align="center">
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml/badge.svg" alt="ওয়েবসাইট" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases/latest"><img src="https://img.shields.io/github/v/release/fzn0x/mixmusic" alt="সর্বশেষ রিলিজ" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases"><img src="https://img.shields.io/github/downloads/fzn0x/mixmusic/total" alt="ডাউনলোড" /></a>
  <a href="https://pkg.go.dev/github.com/fzn0x/mixmusic"><img src="https://pkg.go.dev/badge/github.com/fzn0x/mixmusic.svg" alt="Go Reference" /></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/fzn0x/mixmusic" alt="Go সংস্করণ" /></a>
  <a href="https://awesomemygo.dev/apps/mixmusic"><img src="https://awesomemygo.dev/badge.svg" height="20" alt="Awesome MyGo-তে তালিকাভুক্ত" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey" alt="macOS, Windows ও Linux" />
  <a href="#লাইসেন্স"><img src="https://img.shields.io/badge/license-MIT%20or%20GPL--3.0-blue" alt="MIT বা GPL-3.0" /></a>
</p>

<p align="center">
  <a href="https://fzn0x.github.io/mixmusic/">ওয়েবসাইট</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">ডাউনলোড</a> ·
  <a href="https://fzn0x.github.io/mixmusic/docs/">ডকুমেন্টেশন</a> ·
  <a href="CHANGELOG.md">পরিবর্তনের তালিকা</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic 0.5.9 তার ডিফল্ট চেহারায়, Mix থিমসহ Blend লেআউটে, একটি অন্ধকার মঞ্চে সোজা দাঁড়িয়ে আপনার দিকে মুখ করে আছে: মাঝখানে YOASOBI-র アイドル বাজছে, গোলাপি রেখায় ঘেরা, লিরিক্সের প্রতিটি লাইনের নিচে romaji এবং সামনে মিনি প্লেয়ার; বাঁ দিকে একটি সার্চ আর ডান দিকে লাইব্রেরির মিক্সগুলো" />
</p>

## কেন MixMusic

আপনার গান যদি YouTube Music ও Spotify-তে ভাগ হয়ে থাকে, তাহলে সাধারণত
দুটি অ্যাপ লাগে: দুটি সার্চ, দুটি কিউ আর দুটি লাইব্রেরি। MixMusic দুটি
সার্ভিসকেই একটি প্লেয়ারে আনে:

- **একটি সার্চ।** এটি একসঙ্গে YouTube Music ও Spotify-তে খোঁজে এবং
  প্রতিটি ফলাফলে কোন সার্ভিস থেকে এসেছে তা চিহ্নিত করে।
- **একটি কিউ ও একটি লাইব্রেরি।** YouTube Music-এর গানের ঠিক পরেই
  Spotify-র গান বাজতে পারে, আর একটি প্লেলিস্টে দুই সার্ভিসের গানই
  রাখা যায়।
- **অ্যাকাউন্ট লাগে না।** অ্যাপ খুললেই YouTube Music বাজতে শুরু করে।
  নিজের লাইব্রেরি চাইলে তবেই সাইন ইন করুন। Spotify ঐচ্ছিক।
- **ছোট একটি প্রোগ্রাম।** MixMusic একটিমাত্র Go প্রোগ্রাম, যা GPU-তে
  নিজের উইন্ডো নিজেই আঁকে। এর ভেতরে কোনো ব্রাউজার নেই, যা ডাউনলোড করতে
  বা মেমরিতে ধরে রাখতে হবে।
- **আপনার মতো করে বদলানো যায়।** চারটি লেআউট ও চৌদ্দটি থিম সঙ্গেই
  আসে, কমিউনিটি আরও থিম ও প্লাগইন যোগ করে, আর যেকোনো কমান্ডে নিজের
  পছন্দের কী বসানো যায়।
- **গোপনীয়তা।** MixMusic-এর নিজস্ব কোনো অ্যাকাউন্ট, সার্ভার, অ্যানালিটিক্স
  বা টেলিমেট্রি নেই।
- **বিনামূল্যে ও ওপেন সোর্স,** MIT লাইসেন্স বা GPL 3.0-এর অধীনে।

## ফিচার

### সার্চ ও প্লে

- **দুই সার্ভিসের জন্য একটি সার্চ।** YouTube Music ও Spotify-র ফলাফল
  পাশাপাশি দেখায়, প্রতিটিতে তার সার্ভিসের চিহ্ন থাকে। YouTube Music-এর
  ফলাফলে প্রতিটি গান কতবার বাজানো হয়েছে তা দেখায়, ফলে কভার থেকে মূল
  গানটি আলাদা করে চেনা যায়।
- **একটি কিউ।** দুই সার্ভিসের গান মিলিয়ে নিন, সবগুলোর মধ্যে শাফল ও
  রিপিট করুন, আর কিউ ফুরিয়ে গেলে রেডিও চালিয়ে যাক।
- **Explore (এক্সপ্লোর)।** নতুন রিলিজ, ট্রেন্ডিং গান, চার্ট, মুড ও জনরা,
  আর যা বাজছে তার সঙ্গে সম্পর্কিত গান।
- **সিঙ্ক করা লিরিক্স,** যা গানের সঙ্গে সঙ্গে চলে, অথবা চাইলে সাধারণ
  টেক্সট।
- **একটি মিনি প্লেয়ার,** যা অন্য উইন্ডোগুলোর ওপরে থাকে।

### আপনার লাইব্রেরি

- **মিক্স:** যে প্লেলিস্টে সব সার্ভিসের গান রাখা যায়। কোনো সার্ভিসের
  প্লেলিস্ট মিক্সে ইমপোর্ট করুন, আর যেকোনোটির ভেতরে সার্চ ও সর্ট করুন।
- **পছন্দের গান** দুই সার্ভিস থেকে একটি তালিকায়।
- **হিস্ট্রি ও শোনার পরিসংখ্যান,** দিনে দিনে।
- **আপনার অ্যাকাউন্ট।** হোম ফিড, পছন্দের গান, প্লেলিস্ট, অ্যালবাম ও
  শিল্পীদের জন্য Google-এর নিজস্ব পেজে YouTube Music-এ সাইন ইন করুন,
  আপনার অ্যাকাউন্ট হিসেবে বা তার কোনো ব্র্যান্ড চ্যানেল হিসেবে। সেখানকার
  লাইব্রেরির জন্য Spotify-তে সাইন ইন করুন। লাইক, সংরক্ষিত অ্যালবাম ও
  প্লেলিস্ট এবং ফলো করা শিল্পীরাও সার্ভিসে পৌঁছে যায়। আপনি যা শোনেন তা
  আপনার YouTube Music হিস্ট্রিতে যোগ হয়, যেখান থেকে তার সুপারিশ আসে।

### Spotify, তিনভাবে

- **MixMusic-এর ভেতরেই,** Spotify-র মানে, কোনো Spotify অ্যাপ না খুলেই
  (Premium লাগে, Windows ও macOS-এ)।
- **Spotify Connect-এ:** আপনার Spotify অ্যাপ, ফোন বা স্পিকারে।
- **YouTube Music থেকে:** MixMusic Spotify-র গানটির রেকর্ডিং YouTube
  Music-এ বাজায়, যা বাজাতে Premium লাগে না।

### সাউন্ড

- ক্রসফেড, সমান লাউডনেস ও ভলিউম লেভেলিং।
- প্রিসেটসহ দশ ব্যান্ডের ইকুয়ালাইজার।
- একটি ভিজ্যুয়ালাইজার।
- আপনার শোনা গানগুলোর অফলাইন ক্যাশ।

### আপনার ডেস্কটপে

- Windows, macOS ও Linux-এ মিডিয়া কী এবং সিস্টেমের মিডিয়া কন্ট্রোল।
- প্লে, পজ, স্কিপ, ভলিউম বদল, মিউট, চলতি গানে লাইক বা ডিসলাইক, অথবা
  MixMusic দেখানোর জন্য নিজের গ্লোবাল শর্টকাট, যে উইন্ডোই সামনে থাকুক।
- Windows-এ টাস্কবারের থাম্বনেইলে প্লেয়ারের বোতাম।
- ট্রে কন্ট্রোল, নতুন গানের নোটিফিকেশন এবং লগইনের সময় চালু হওয়া।
- `mixmusic://` লিংক।
- **Last.fm:** আপনি যে গানই শোনেন তা আপনার প্রোফাইলে যোগ হয়, আর অফলাইনে
  হওয়া স্ক্রোবল অনলাইনে ফিরলে পাঠানো হয়।
- **Discord:** আপনার প্রোফাইলে কী বাজছে তা আর্টওয়ার্ক ও বাকি সময়সহ দেখায়।
- **আপডেট:** MixMusic চালু হওয়ার সময় এবং প্রতি কয়েক ঘণ্টা পরপর নতুন
  রিলিজ আছে কি না দেখে, তার সিগনেচার যাচাই করে, এবং আপনি বললে, বা আপনি
  চাইলে নিজে থেকেই, তা ইনস্টল করে।

### নিজের মতো সাজান

- **লেআউট।** ডিফল্ট Blend-এ আছে গোল আকার আর বড় কভারের ঘন সারিতে
  স্প্রিংয়ের মতো গতি। Expressive, Material 3 Expressive-এর আদলে, খেলাচ্ছলে
  ভরা, আছে টোনাল ফিল, পিল বোতাম ও ভাসমান প্লেয়ার বার। Crate সংযত, যাতে
  কভারগুলোই কাজ করে, আর Tiles হলো MixMusic-এর প্রথম চেহারা, সমতল টাইল
  ও বিশাল ছোট হাতের অক্ষরের শব্দ নিয়ে।
- **থিম।** চৌদ্দটি বিল্ট-ইন, প্রতিটি যেকোনো লেআউটে চলে, বদলানো যায় এমন
  ইফেক্টসহ, লাইট মোডের জন্য একটি ও ডার্ক মোডের জন্য একটি থিম, আর নিজের
  বা বাজতে থাকা আর্টওয়ার্ক থেকে নেওয়া অ্যাক্সেন্ট রঙ। Material ও
  Material Dark প্রতিটি রং কভার থেকে নেয়।
- **কমিউনিটি থিম ও প্লাগইন,** মার্কেটপ্লেসে এক ক্লিকেই পাওয়া যায়।
- **প্লাগইন** টাইপড API-র মাধ্যমে পেজ, কমান্ড, ট্র্যাক অ্যাকশন ও উইজেট
  যোগ করে, যা আপনার থিমেই আঁকা হয়।
- **কীবোর্ড আগে।** সাধারণ সবকিছুর শর্টকাট, যেকোনো কমান্ডের জন্য নিজের কী,
  প্লাগইনের কমান্ডসহ, যেকোনো অ্যাপ থেকে গান নিয়ন্ত্রণের কী, আর বাকিটার
  জন্য একটি কমান্ড প্যালেট (<kbd>Ctrl</kbd> <kbd>K</kbd>)।
- **একটি সেটিংস ফাইল।** আপনার সেটিংস থাকে ডেটা ফোল্ডারের `settings.json`-এ
  (Windows-এ `%AppData%\MixMusic`, macOS-এ `~/Library/Application Support/MixMusic`,
  Linux-এ `~/.config/MixMusic`), যা আপনি নিজের ডটফাইলের সঙ্গে রাখতে পারেন।
  MixMusic চলার সময় সেভ করলে তা সঙ্গে সঙ্গে প্রযোজ্য হয়; Settings › Desktop
  › Settings file (সেটিংস › ডেস্কটপ › সেটিংস ফাইল) এটি খোলে।

## এক নজরে

<p align="center">
  <img src="docs/look-listen.webp" alt="Blend লেআউটে MixMusic-এর পেজ, একটি অন্ধকার মঞ্চে সোজা দাঁড়িয়ে আপনার দিকে মুখ করে আছে: মাঝখানে YOASOBI-র アイドル বাজছে, গোলাপি রেখায় ঘেরা, লিরিক্সের প্রতিটি লাইনের নিচে romaji ও ইংরেজি অনুবাদ; বাঁ দিকে THE BOOK 3 অ্যালবাম আর ডান দিকে ফলো করা একটি পডকাস্টের পেজ" />
</p>

<p align="center"><sub><b>শুনুন।</b> মাঝখানে YOASOBI-র アイドル বাজছে: গাওয়ার সঙ্গে সঙ্গে প্রতিটি লাইন জ্বলে ওঠে, নিচে romaji ও অনুবাদ থাকে। বাঁ দিকে একটি অ্যালবাম; ডান দিকে আপনার ফলো করা একটি পডকাস্ট।</sub></p>

<p align="center">
  <img src="docs/look-yours.webp" alt="MixMusic-এর আরও কিছু, একটি অন্ধকার মঞ্চে সোজা দাঁড়িয়ে আপনার দিকে মুখ করে আছে: মাঝখানে চারটি লেআউটসহ Settings › Appearance, গোলাপি রেখায় ঘেরা; বাঁ দিকে বাজতে থাকা কভারের রঙে রাঙানো একটি Material থিম, আর ডান দিকে মার্কেটপ্লেসের কমিউনিটি থিম" />
</p>

<p align="center"><sub><b>নিজের মতো সাজান।</b> মাঝখানে চারটি লেআউট: ডিফল্ট Blend, Expressive, Crate ও Tiles, থিমগুলোর নিচে। বাঁ দিকে একটি Material থিম, যা বাজতে থাকা কভার থেকে রং নেয়। ডান দিকে মার্কেটপ্লেসের কমিউনিটি থিম।</sub></p>

## ইনস্টল

[সর্বশেষ রিলিজ](https://github.com/fzn0x/mixmusic/releases/latest) থেকে
আপনার সিস্টেমের ফাইলটি ডাউনলোড করুন:

| সিস্টেম | ডাউনলোড |
|---|---|
| macOS 12 ও তার পরের (Apple silicon ও Intel) | [`mixmusic-<version>-macos-universal.dmg`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows 10 ও 11 (x64, Arm), ইনস্টলার | [`mixmusic-<version>-windows-<arch>-setup.exe`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows, পোর্টেবল, ইনস্টল ছাড়াই | [`mixmusic-<version>-windows-<arch>-portable.zip`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Debian, Ubuntu ও তাদের ডেরিভেটিভ | [`mixmusic-<version>-linux-<arch>.deb`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Fedora, openSUSE ও তাদের ডেরিভেটিভ | [`mixmusic-<version>-linux-<arch>.rpm`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Arch Linux ও তার ডেরিভেটিভ (শীঘ্রই আসছে) | AUR-এ [`mixmusic-bin`](https://aur.archlinux.org/packages/mixmusic-bin): `yay -S mixmusic-bin` |
| যেকোনো Linux, একটি ফাইল | [`mixmusic-<version>-linux-<arch>.AppImage`](https://github.com/fzn0x/mixmusic/releases/latest) |
| যেকোনো Linux, স্যান্ডবক্সড (Flatpak) | [`mixmusic-<version>-linux-<arch>.flatpak`](https://github.com/fzn0x/mixmusic/releases/latest) |
| যেকোনো Linux, আপনার হোম ফোল্ডারে | [`mixmusic-<version>-linux-<arch>.tar.gz`](https://github.com/fzn0x/mixmusic/releases/latest), সঙ্গে `install.sh` |

`<arch>` হলো x64-এর জন্য `amd64` আর Arm-এর জন্য `arm64`। প্রতিটি রিলিজে
ডাউনলোড যাচাইয়ের জন্য `SHA256SUMS` দেওয়া থাকে। `update-…`, `….delta`,
`mixmusic-<version>-macos-…` বা `-windows-<arch>.tar.gz` নামের ফাইলগুলো
আপনার লাগবে না: ইনস্টল করা অ্যাপ ওগুলো থেকেই আপডেট হয়।

ইনস্টলার, পোর্টেবল zip ও Linux আর্কাইভ নিজে নিজেই আপডেট হয়। `.deb`,
`.rpm`, AppImage ও Flatpak নিজেদের ফাইল বদলাতে পারে না, তাই নতুন
সংস্করণ এলে MixMusic আপনাকে জানায় এবং আপনার জন্য সেটি ডাউনলোড করে।
AUR-এর `mixmusic-bin` আপনার বাকি সিস্টেমের সঙ্গে আপডেট হয়। পোর্টেবল
zip-এর জন্য, যেকোনো জায়গায় আনজিপ করে `MixMusic.exe` খুলুন। AppImage-এর
জন্য, এটি এক্সিকিউটেবল করে (`chmod +x`) খুলুন। Flatpak-এর জন্য, এটি আপনার
সফটওয়্যার সেন্টারে খুলুন, অথবা চালান
`flatpak install ./mixmusic-<version>-linux-<arch>.flatpak`; প্রথমবার
Flatpak Flathub থেকে GNOME রানটাইম নামিয়ে আনে।

শীঘ্রই আসছে, এর ট্যাপ চালু হলে: Mac-এ [Homebrew](https://brew.sh)
MixMusic ইনস্টল করে, আর `brew upgrade` তা আপডেট করে।

```sh
brew install --cask fzn0x/tap/mixmusic
```

### প্রথমবার চালু করা

- **macOS:** অ্যাপটি এখনও Apple দ্বারা নোটারাইজড নয়, তাই প্রথমবার খুললে
  macOS বলে যে এটি যাচাই করতে পারছে না। তবু খুলুন System Settings ›
  Privacy & Security › Open Anyway (সিস্টেম সেটিংস › প্রাইভেসি ও সিকিউরিটি ›
  তবুও খুলুন) থেকে, অথবা Terminal-এ একবার এটি চালান:

  ```sh
  xattr -dr com.apple.quarantine /Applications/MixMusic.app
  ```
- **Windows:** ইনস্টলার এখনও সাইন করা নয়, তাই SmartScreen সতর্ক করতে পারে:
  More info (আরও তথ্য) বেছে নিন, তারপর Run anyway (তবুও চালান)। YouTube
  Music-এ সাইন ইন এবং MixMusic-এ Spotify বাজাতে WebView2 Runtime লাগে, যা
  Windows 11-এ আগে থেকেই থাকে।
- **Linux:** MixMusic-এর জন্য GTK 3 এবং PulseAudio (PipeWire-ও তা
  দেয়) বা ALSA লাগে। YouTube Music-এ সাইন ইনের জন্য WebKitGTK 4.1, ট্রে
  আইকনের জন্য `libayatana-appindicator3`, আর আপনার সাইন-ইন সিস্টেমের
  কীরিংয়ে রাখার জন্য `libsecret` লাগে। `.deb` ও `.rpm` এগুলো ইনস্টল করে,
  আর Flatpak এগুলো সঙ্গে নিয়েই আসে; AppImage ও আর্কাইভের জন্য:

  ```sh
  # Debian and Ubuntu
  sudo apt install libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
  # Fedora
  sudo dnf install gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
  ```
- **সব সিস্টেমে:** YouTube Music-এর বেশিরভাগ গানের অডিও MixMusic নিজেই
  খুঁজে নেয়। বাকিগুলোর জন্য, যেমন শিশুদের জন্য বানানো গান, এটি
  [yt-dlp](https://github.com/yt-dlp/yt-dlp) ব্যবহার করে, যা Settings ›
  Playback (সেটিংস › প্লেব্যাক) থেকে এটি আপনার জন্য ইনস্টল করে এবং আপ টু ডেট
  রাখে। yt-dlp-র জন্য একটি JavaScript রানটাইম লাগে:
  [Deno](https://deno.com), Node.js, Bun বা QuickJS। Flatpak-এ Node.js আছে।

### Flatpak

Flatpak MixMusic-কে একটি স্যান্ডবক্সে চালায়, যা নেটওয়ার্ক, সাউন্ড সার্ভার,
ডিসপ্লে ও GPU দেখতে পায়, কিন্তু আপনার হোম ফোল্ডারের কিছুই নয়। MixMusic
তার সেটিংস ও লাইব্রেরি রাখে `~/.var/app/io.github.fzn0x.mixmusic`-এ,
`.deb`, `.rpm`, AppImage বা আর্কাইভ কপি থেকে আলাদাভাবে, আর দুটিই একসঙ্গে
চলতে পারে। কোনো কপির সেটিংস ও লাইব্রেরি নিয়ে আসতে, দুটিই বন্ধ করুন,
`~/.config/MixMusic` থেকে `settings.json`, `library.json`, `queue.json` ও
`addons` ফোল্ডার কপি করে
`~/.var/app/io.github.fzn0x.mixmusic/config/MixMusic`-এ রাখুন, এবং আপনার
অ্যাকাউন্টগুলোতে আবার সাইন ইন করুন: সাইন-ইন থাকে অন্য কপির কীরিংয়ে।

Flatpak-এ লগইনের সময় চালু হওয়া আপনার ডেস্কটপের মাধ্যমে যায়, যা আগে
আপনাকে জিজ্ঞেস করতে পারে, আর নিজের yt-dlp থাকলে সেটি
`~/.var/app/io.github.fzn0x.mixmusic`-এর ভেতরে রাখতে হবে, কারণ স্যান্ডবক্স
অন্য কোনো ফোল্ডার দেখে না। Flatpak নয় এমন Discord অ্যাপ কী বাজছে তা দেখায়
কেবল যদি MixMusic চালু হওয়ার সময় সেটি চলমান থাকে, নয়তো রিস্টার্ট
না হওয়া পর্যন্ত দেখায় না; Flatpak হিসেবে Discord দুভাবেই কাজ করে। Flatpak
নতুন: আপনার ডেস্কটপে এটি কেমন চলছে তা একটি
[issue](https://github.com/fzn0x/mixmusic/issues)-তে জানান।

### অ্যাকাউন্ট (সবই ঐচ্ছিক)

- **YouTube Music:** Settings › Accounts (সেটিংস › অ্যাকাউন্ট)-এ Sign in to
  YouTube Music (YouTube Music-এ সাইন ইন) বেছে নিন এবং যে উইন্ডো খোলে তাতে
  Google-এ সাইন ইন করুন। আপনি যা টাইপ করেন তা কেবল Google-এ যায়। MixMusic
  সেশনটি এনক্রিপ্ট করে রাখে, যেখানে কীরিং আছে সেখানে সিস্টেমের কীরিংয়ে
  রাখা কী দিয়ে। যেখানে ওই উইন্ডো খোলা যায় না, বা Google তা প্রত্যাখ্যান
  করে, সেখানে নিজের ব্রাউজার দিয়ে সাইন ইন করুন, যা MixMusic নতুন একটি
  নিজস্ব প্রোফাইল দিয়ে খোলে এবং আপনি ঢুকে গেলে মুছে দেয়; অথবা যে ব্রাউজারে
  সাইন ইন করেছেন তার সেশন ইমপোর্ট করুন (MixMusic সেটি তার yt-dlp দিয়ে
  পড়ে, এবং কেবল YouTube-এর কুকি রাখে); অথবা সাইন ইন করা music.youtube.com
  ট্যাবের Cookie হেডার পেস্ট করুন।
- **Spotify:** Spotify একটি অ্যাপকে সর্বোচ্চ পাঁচজন ব্যবহারকারীকে সেবা দিতে
  দেয়, তা-ও কেবল যতক্ষণ এর মালিকের Premium আছে, তাই আপনি নিজের একটি
  Spotify অ্যাপ দিয়ে সাইন ইন করেন। এতে দুই মিনিট লাগে:
  [docs/spotify.md](docs/spotify.md) আপনাকে ধাপে ধাপে দেখিয়ে দেয়।
- **Last.fm:** [last.fm/api/account/create](https://www.last.fm/api/account/create)-এ
  একটি API অ্যাকাউন্ট তৈরি করুন (যেকোনো নাম, কোনো callback URL নয়), এর API
  key ও shared secret Settings › Accounts (সেটিংস › অ্যাকাউন্ট)-এ পেস্ট করুন,
  এবং আপনার ব্রাউজারে MixMusic-কে অনুমোদন দিন।
- **Discord:** Discord অ্যাপ চলার সময় আপনার প্রোফাইলে আপনি কী শুনছেন তা
  দেখায়। Settings › Desktop (সেটিংস › ডেস্কটপ)-এ এটি বন্ধ করুন।

কিছু কাজ করছে না? [docs/troubleshooting.md](docs/troubleshooting.md)-এ
সাধারণ সমস্যা ও জানা সমস্যাগুলো আছে।

## সোর্স থেকে বিল্ড

আপনার লাগবে [Go](https://go.dev/dl/) 1.27.3 বা তার পরের, [Bun](https://bun.sh)
1.2 বা তার পরের, যা MyGo-র কমান্ড লাইন চালায়, এবং Git। MyGo-র কোনো C
কম্পাইলার লাগে না, তাই যেকোনো সিস্টেমে সব সিস্টেমের জন্য MixMusic বিল্ড
করা যায়।

### Windows

```powershell
winget install GoLang.Go
winget install Oven-sh.Bun
winget install Git.Git
```

একটি নতুন টার্মিনাল খুলুন, তারপর:

```powershell
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

সাইন ইন ও Spotify প্লেব্যাকের জন্য WebView2 Runtime লাগে, যা Windows 11-এ
আগে থেকেই থাকে। Windows 10-এ এটি
[Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/) থেকে
ইনস্টল করুন।

### macOS

[Homebrew](https://brew.sh) দিয়ে:

```sh
brew install go oven-sh/bun/bun
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Linux

[go.dev/dl](https://go.dev/dl/) থেকে Go ইনস্টল করুন (ডিস্ট্রিবিউশনের
প্যাকেজ প্রায়ই 1.27.3-এর চেয়ে পুরোনো হয়), তারপর Bun এবং MixMusic যেসব
লাইব্রেরি নিয়ে চলে সেগুলো:

```sh
curl -fsSL https://bun.sh/install | bash
# Debian and Ubuntu
sudo apt install git libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
# Fedora
sudo dnf install git gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
```

তারপর:

```sh
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Nix

[Nix](https://nixos.org) থাকলে `nix develop` আপনাকে সেই টুলগুলো দেয় যা
দিয়ে রিলিজ বিল্ড হয়, `flake.lock`-এ পিন করা সংস্করণে: Go, Bun, NSIS,
GoReleaser ও Zola। `nix build` সোর্স থেকে Linux-এর জন্য MixMusic বিল্ড
করে, `result/`-এ।

### ডেভেলপ

`bun run dev` অ্যাপ বিল্ড করে, চালু করে, এবং আপনি এডিট করলে আবার বিল্ড করে।
এর ডেটা ইনস্টল করা MixMusic থেকে আলাদা থাকে, `MixMusic Dev` নামে।

```sh
bun run build:plugins          # the plugins' JavaScript, from their TypeScript
go test ./...                  # the app's tests
bun run typecheck              # the plugin SDK and the community's plugins
go run ./tools/registry -check # the community registry
bun run build                  # the packaged app for this system, in build/
bun run build -- -platform windows/amd64,linux/amd64,linux/arm64
```

Nix শেলে Linux ডাউনলোড বিল্ড করুন
`.github/scripts/build.sh linux/amd64,linux/arm64` দিয়ে, যেমন রিলিজে হয়।
সেখানে সাধারণ `bun run build` এমন একটি Linux প্রোগ্রাম বানায় যা Nix-এর
নিজস্ব লোডার চায়, আর সেটি শুধু NixOS-এ চালু হয়।
`go run ./tools/elfloader build/linux-amd64` একটি বিল্ড যাচাই করে।

[ওয়েবসাইট](https://fzn0x.github.io/mixmusic/) থাকে `site/`-এ, যা
[Zola](https://www.getzola.org) দিয়ে বিল্ড করা, আর `docs/`-এর গাইডগুলো এর
`/docs/` পেজ। `go run ./tools/sitedocs && zola --root site serve` এর প্রিভিউ
দেখায়। রিলিজ বিল্ড হয় Nix শেলে এবং প্রকাশ করে
[GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`)।

[docs/architecture](docs/architecture/README.md) ব্যাখ্যা করে অংশগুলো কীভাবে
একসঙ্গে মেলে।

### লেআউট

অ্যাপটি হলো রুট প্যাকেজ, আর তার নিচে `internal/`-এর প্যাকেজগুলো। `internal/`-এর
বাইরের প্যাকেজগুলো যে কেউ ইমপোর্ট করতে পারে, এবং
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic)-এ ডকুমেন্ট করা।

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

যেসব প্যাকেজ ইমপোর্ট করা যায়:

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

## কমিউনিটি

যে কেউ মার্কেটপ্লেস থেকে, অর্থাৎ সাইডবারের স্টোর আইকন থেকে, কমিউনিটি থিম ও
প্লাগইন ইনস্টল করতে পারে, আর আপনিও নিজেরটি যোগ করতে পারেন।

### একটি থিম বানান

থিম হলো একটি `theme.json` ফাইল, যা রং, ফন্ট ও কোণ ঠিক করে, আর ব্যবহারকারীর
বেছে নেওয়া লেআউটে সেগুলো দেখা যায়।
[docs/themes.md](docs/themes.md) প্রতিটি টোকেন তালিকাভুক্ত করে এবং একটি
সম্পূর্ণ থিম দেখায়। নিজেরটি পরখ করতে এর ফোল্ডারটি থিম ফোল্ডারে রাখুন
(Settings › Appearance › open themes folder (সেটিংস › অ্যাপিয়ারেন্স › থিম
ফোল্ডার খুলুন)) এবং Settings › Appearance-এ এটি বেছে নিন।

### একটি প্লাগইন বানান

প্লাগইন হলো TypeScript বা JavaScript-এ লেখা ছোট একটি মডিউল, যা পেজ, কমান্ড,
ট্র্যাক অ্যাকশন ও উইজেট যোগ করে। [docs/plugins.md](docs/plugins.md) API
ডকুমেন্ট করে, আপনার এডিটরের জন্য এর [টাইপ](sdk/index.d.ts)সহ। কমিউনিটির
[sleep timer](community/plugins/sleep-timer) ও
[spectrum](community/plugins/spectrum) কাজ করা উদাহরণ।

### শেয়ার করুন

1. এই রিপোজিটরি ফর্ক করুন।
2. একটি ফোল্ডার যোগ করুন: থিমের জন্য `community/themes/<id>/`, অথবা
   প্লাগইনের জন্য `community/plugins/<id>/`, এর `plugin.json` ও `index.ts`
   সহ।
3. চালান `bun run build:plugins`, `go run ./tools/registry` এবং
   `go run ./tools/registry -check`।
4. একটি পুল রিকোয়েস্ট খুলুন।

মার্জ হয়ে গেলে CI এটি
[registry ব্রাঞ্চে](https://github.com/fzn0x/mixmusic/tree/registry) প্রকাশ
করে, আর তা সবার মার্কেটপ্লেসে দেখা যায়।
[community/README.md](community/README.md)-এ বিস্তারিত আছে।

## গোপনীয়তা

MixMusic-এর নিজস্ব কোনো অ্যাকাউন্ট, সার্ভার, অ্যানালিটিক্স বা টেলিমেট্রি
নেই। এটি যাদের সঙ্গে কথা বলে:

- **YouTube ও Google:** YouTube Music-এর ক্যাটালগ, এর স্ট্রিম এবং প্রতিটি
  গান কতটা জোরে, সাইন আউট অবস্থায় চাওয়া, আর আর্টওয়ার্ক; কিছু স্ট্রিম
  yt-dlp-র মাধ্যমে। সাইন ইন করা অবস্থায়, সেখানকার আপনার লাইব্রেরি, এবং,
  Settings › Accounts (সেটিংস › অ্যাকাউন্ট)-এ বন্ধ না করলে, আপনি যে গান
  বাজান, যা আপনার YouTube Music হিস্ট্রিতে যোগ হয়।
- **Spotify,** সাইন ইন করার পর: আপনার লাইব্রেরি, আপনার Spotify Connect
  ডিভাইস, এবং Spotify-র প্লেয়ার, `sdk.scdn.co` থেকে লোড হয়, যখন
  MixMusic-এ Spotify বাজে।
- **Unison:** যে গান বাজছে তার ভিডিও আইডি, অথবা সেটি না থাকলে তার শিরোনাম,
  শিল্পীরা, অ্যালবাম ও দৈর্ঘ্য, লিরিক্স খুঁজতে। শুধু তখনই, যখন আপনি লিরিক্সের
  পাশে translate বা romanize চালু করেন, বাজতে থাকা গানের লিরিক্স, তার ভিডিও
  আইডিসহ, যা Unison Google দিয়ে অনুবাদ করায়।
- **LRCLIB:** যে গান বাজছে তার শিরোনাম, শিল্পীরা, অ্যালবাম ও দৈর্ঘ্য,
  এর লিরিক্স খুঁজতে।
- **GitHub:** চালু হওয়ার সময় এবং প্রতি কয়েক ঘণ্টা পরপর নতুন সংস্করণের
  খোঁজ, যা আপনি Settings › About (সেটিংস › সম্পর্কে)-এ বন্ধ করতে পারেন;
  আপনি মার্কেটপ্লেস খুললে কমিউনিটি রেজিস্ট্রি; yt-dlp-র ডাউনলোড; এবং
  জাপানি লিরিক্সের পাশে প্রথমবার romaji চালু করলে, কানজির পাঠের অভিধান
  (11.7 MB)। Romaji তৈরি হয় আপনার কম্পিউটারেই: লিরিক্স পাঠানো হয় না।
- **Last.fm,** আপনি যুক্ত করলে: আপনি কী বাজান।
- **Discord,** বন্ধ না করলে: কী বাজছে, আপনার কম্পিউটারের Discord অ্যাপে
  পাঠানো হয়, যা তা আপনার প্রোফাইলে দেখায়।

আপনার কম্পিউটারে যা রাখে:

- আপনার সেটিংস ও লাইব্রেরি (পছন্দের ও অপছন্দের গান, মিক্স, হিস্ট্রি এবং
  সাম্প্রতিক সার্চ), Settings › About-এ যে ফোল্ডারের নাম আছে সেখানে।
- আপনার বাজানো গানগুলো, ক্যাশে, Settings › Playback (সেটিংস › প্লেব্যাক)-এ
  যতটা অনুমতি দেন ততটা, এবং YouTube সেগুলো কতটা জোরে মেপেছে।
- আপনার দেখা কভারগুলো, 256 MB পর্যন্ত।
- আপনার চালু করা লিরিক্স অনুবাদগুলো, গান ও ভাষা অনুযায়ী, এবং romaji যে
  অভিধান দিয়ে কানজি পড়ে সেটি, আপনি তা চালু করার পর।
- আপনার সাইন-ইনগুলো, YouTube Music সেশনসহ, সিস্টেমের কীরিংয়ে। Windows-এ
  Credential Manager কোনোটি রাখতে না পারলে সেটি আপনার Windows অ্যাকাউন্টের
  জন্য এনক্রিপ্ট করা একটি ফাইলে যায়; কীরিং ছাড়া Linux-এ, কেবল আপনার
  ব্যবহারকারী পড়তে পারে এমন ফাইলে।
- লগ, আপনার সাইন-ইন ছাড়া।

লাইব্রেরির হিস্ট্রি পেজ আপনার বাজানো গানের তালিকা মুছে দেয়, Settings ›
Playback ক্যাশ ও কভার মুছে দেয়, আর কোনো সার্ভিস থেকে সাইন আউট করলে তার
সাইন-ইন মুছে যায়।

Windows-এ ভিজ্যুয়ালাইজার এই কম্পিউটারে বাজতে থাকা Spotify-র অডিও শোনে,
MixMusic-এ হোক বা Spotify অ্যাপে, Windows-এর process loopback-এর মাধ্যমে,
যা কেবল সেই অ্যাপটিকেই শোনে। macOS 14.2 ও তার পরের সংস্করণে এটি MixMusic-এ
বাজতে থাকা Spotify শোনে একটি process tap-এর মাধ্যমে, macOS যেভাবে চায় সেভাবে
আপনি অনুমতি দেওয়ার পর। এটি অডিও মেমরিতেই বিশ্লেষণ করে, কখনো রেকর্ড করে না
বা পাঠায় না।

## কৃতজ্ঞতা

MixMusic এই প্রজেক্টগুলোর ওপর দাঁড়িয়ে আছে:

- **[MyGo](https://github.com/egoist/mygo),** যে ডেস্কটপ ফ্রেমওয়ার্ক দিয়ে
  MixMusic বানানো। MixMusic MyGo-র নেটিভ UI ব্যবহার করে: MyGo প্রতিটি উইন্ডো
  নিজেই Go-তে আঁকে, ওয়েবভিউ ছাড়া, পেজ ও প্লেয়ার বার থেকে মিনি প্লেয়ার পর্যন্ত।
  এর কমান্ড লাইন অ্যাপটি প্যাকেজ করে, ইনস্টলার সহ এবং সাইন করা আপডেটসহ, যা
  থেকে অ্যাপ নিজে আপডেট হয়।
- **[Go](https://go.dev),** অ্যাপের ভাষা, সঙ্গে
  [purego](https://github.com/ebitengine/purego), যা cgo ছাড়াই সিস্টেমের
  লাইব্রেরি কল করে, এবং [golang.org/x/image](https://pkg.go.dev/golang.org/x/image)
  ও [golang.org/x/text](https://pkg.go.dev/golang.org/x/text)।
- **[Sobek](https://github.com/grafana/sobek),** যে JavaScript ইঞ্জিনে
  প্রতিটি প্লাগইন চলে।
- **[Nix](https://nixos.org),** যা রিলিজ বিল্ডের টুলগুলো পিন করে এবং
  সোর্স থেকে Linux-এর জন্য MixMusic বিল্ড করে।
- **[GoReleaser](https://goreleaser.com),** যা রিলিজ প্রকাশ করে, এবং এর
  [nFPM](https://nfpm.goreleaser.com), যা RPM প্যাকেজ বানায়।
- **[Bun](https://bun.sh),** যা বিল্ড চালায়, এবং
  **[TypeScript](https://www.typescriptlang.org),** যা প্লাগইন কম্পাইল করে ও
  প্লাগইন SDK-র টাইপ যাচাই করে।
- **[Zola](https://www.getzola.org),** যা ওয়েবসাইট বিল্ড করে।
- **[NSIS](https://nsis.sourceforge.io),** যা Windows ইনস্টলার বানায়, এবং
  [appimagetool](https://github.com/AppImage/appimagetool), AppImage-গুলো।
- **[yt-dlp](https://github.com/yt-dlp/yt-dlp),** যা সেসব গানের অডিও খুঁজে
  দেয় যা MixMusic নিজে খুঁজে পায় না।
- **[Unison](https://unison.betterlyrics.org),** লিরিক্সের ওপেন, ক্রাউডসোর্সড
  ডেটাবেস, ODbL-এর অধীনে, এবং **[LRCLIB](https://lrclib.net),** সিঙ্ক করা
  লিরিক্সের ওপেন ডেটাবেস।
- **[kagome](https://github.com/ikawaha/kagome),** জাপানি মর্ফোলজিক্যাল
  অ্যানালাইজার, যা লিরিক্সের কানজি পড়ে তার romaji-র জন্য, সঙ্গে
  [kagome-dict](https://github.com/ikawaha/kagome-dict)-এর IPA অভিধান,
  mecab-ipadic থেকে বানানো, যা romaji প্রথমবার চালু করলে MixMusic ডাউনলোড
  করে।
- **[libopus](https://opus-codec.org),** যার ডিকোডারের পোর্ট হলো Opus
  ডিকোডার, এবং **[SharpJaad](https://github.com/jimm98y/SharpJaad),**
  [JAAD](https://sourceforge.net/projects/jaadec/)-এর C# পোর্ট, যা থেকে AAC
  ডিকোডার উদ্ভূত।
- **[FFmpeg](https://ffmpeg.org),** যার সঙ্গে ডিকোডারগুলোর টেস্ট তুলনা করে।
- **[Selawik](https://github.com/microsoft/Selawik),**
  **[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily),**
  **[Roboto Flex](https://github.com/googlefonts/roboto-flex)** ও
  **[Archivo](https://github.com/Omnibus-Type/Archivo),** উইন্ডোগুলোর
  টাইপফেস।
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities),**
  যার HCT কালার সায়েন্সের পোর্ট হলো Material থিমগুলোর রং।
- **[Kevin MacLeod](https://incompetech.com),** যার সংগীত,
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)-এর অধীনে,
  ওয়েবসাইটে বাজে।

## অবদান রাখা

ফিক্স, ফিচার, থিম, প্লাগইন ও বাগ রিপোর্ট সবই স্বাগত।

- **সমস্যা জানান** Settings › About › report a problem (সেটিংস › সম্পর্কে ›
  সমস্যা জানান) থেকে। এটি আপনার সংস্করণ, সিস্টেম, থিম ও প্লাগইন ভরা একটি
  নতুন GitHub issue খোলে। অথবা নিজে [একটি issue খুলুন](https://github.com/fzn0x/mixmusic/issues/new)।
- **পরিবর্তন পাঠান** পুল রিকোয়েস্ট হিসেবে। [CONTRIBUTING.md](CONTRIBUTING.md)
  ব্যাখ্যা করে কোন চেক চালাতে হবে, কমিটের ধরন, এবং মানুষের চোখে পড়ার মতো
  পরিবর্তনে যে চেঞ্জলগ এন্ট্রি লাগে তা।
- **থিম বা প্লাগইন শেয়ার করুন:** দেখুন [কমিউনিটি](#কমিউনিটি)।

[CONTRIBUTORS.md](CONTRIBUTORS.md) সেই মানুষদের ধন্যবাদ জানায় যাদের কোড, ধারণা
ও রিপোর্ট MixMusic-কে গড়েছে।

## লাইসেন্স

MixMusic দ্বৈত-লাইসেন্সযুক্ত: এটি ব্যবহার করুন [MIT লাইসেন্স](LICENSE-MIT)
অথবা [GNU General Public License, সংস্করণ 3](LICENSE-GPL)-এর অধীনে, যেটি
আপনার পছন্দ। অবদান দুটির অধীনেই গৃহীত হয়। `fonts`-এর টাইপফেসগুলো SIL Open
Font License-এর অধীনে: [Selawik-এরটি](fonts/OFL.txt),
[Quicksand-এরটি](fonts/quicksand-OFL.txt),
[Roboto Flex-এরটি](fonts/robotoflex-OFL.txt) এবং
[Archivo-এরটি](fonts/archivo-OFL.txt)। Opus ডিকোডার libopus-এর পোর্ট, [এর BSD
লাইসেন্সের](opus/LICENSE.libopus) অধীনে, AAC ডিকোডার SharpJaad থেকে উদ্ভূত,
[এর MIT লাইসেন্সের](aac/LICENSE.sharpjaad) অধীনে, আর HCT কালার সায়েন্স
Material Color Utilities-এর পোর্ট, [এর Apache লাইসেন্সের](internal/hct/LICENSE.material-color-utilities)
অধীনে।

MixMusic Spotify বা Google-এর সঙ্গে সম্পর্কিত নয়, তাদের দ্বারা সমর্থিত বা
পৃষ্ঠপোষকতাপ্রাপ্তও নয়। এই সার্ভিসগুলো তাদের ব্যবহারকারীদের যা দেয়, এটি
সেই সার্ভিসগুলোর নিজস্ব ইন্টারফেসের মাধ্যমে তা-ই বাজায়; তাদের শর্ত যতটা
অনুমতি দেয় সেভাবেই ব্যবহার করুন। Spotify ও YouTube Music তাদের মালিকদের
ট্রেডমার্ক।
