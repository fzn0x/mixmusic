<p align="center"><a href="README.md">English</a> · <a href="README.hi.md">हिन्दी</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ur.md">اردو</a> · <a href="README.ha.md">Hausa</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <strong>বাংলা</strong> · <a href="README.ru.md">Русский</a> · <a href="README.am.md">አማርኛ</a></p>

<p align="center">
  <img src="brand/icon.png" width="112" height="112" alt="MixMusic-এর আইকন: কালো পটভূমিতে দুটি গোলাপি বলয় ও একটি প্লে বোতাম" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center"><strong>YouTube Music ও Spotify-র জন্য একটি প্লেয়ার।</strong></p>

<p align="center">macOS, Windows ও Linux-এর জন্য YouTube Music ও Spotify ক্লায়েন্ট। YouTube Music চলে কোনো অ্যাকাউন্ট ছাড়াই। একটি সার্চ, একটি কিউ, মিশ্র প্লেলিস্ট, সিঙ্ক করা লিরিক্স, পডকাস্ট, আপনার নিজের গানের ফাইল, কাস্টিং, থিম ও প্লাগইন।</p>

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
  <a href="https://mixmusic.functionx.dev/">ওয়েবসাইট</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">ডাউনলোড</a> ·
  <a href="https://mixmusic.functionx.dev/docs/">ডকুমেন্টেশন</a> ·
  <a href="docs/features.md">সব ফিচার</a> ·
  <a href="CHANGELOG.md">পরিবর্তনের তালিকা</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic তার ডিফল্ট চেহারায়, Mix থিমসহ Blend লেআউটে, একটি অন্ধকার মঞ্চে সোজা দাঁড়িয়ে আপনার দিকে মুখ করে আছে: মাঝখানে YOASOBI-র アイドル বাজছে, গোলাপি রেখায় ঘেরা, লিরিক্সের প্রতিটি লাইনের নিচে romaji এবং সামনে মিনি প্লেয়ার; বাঁ দিকে একটি সার্চ আর ডান দিকে লাইব্রেরির মিক্সগুলো" />
</p>

## কেন MixMusic

আপনার গান যদি YouTube Music ও Spotify-তে ভাগ হয়ে থাকে, তাহলে সাধারণত
দুটি অ্যাপ লাগে: দুটি সার্চ, দুটি কিউ আর দুটি লাইব্রেরি। MixMusic দুটি
সার্ভিসকেই একটি প্লেয়ারে আনে:

- **একটি সার্চ।** এটি একসঙ্গে YouTube Music ও Spotify-তে খোঁজে এবং
  প্রতিটি ফলাফলে কোন সার্ভিস থেকে এসেছে তা চিহ্নিত করে।
- **একটি কিউ ও একটি লাইব্রেরি।** YouTube Music-এর গানের ঠিক পরেই
  Spotify-র গান বাজতে পারে, আর একটি প্লেলিস্টে দুই সার্ভিসের গানই
  রাখা যায়, সঙ্গে আপনার কম্পিউটারের গানের ফাইলও।
- **অ্যাকাউন্ট লাগে না।** অ্যাপ খুললেই YouTube Music বাজতে শুরু করে।
  নিজের লাইব্রেরি চাইলে তবেই সাইন ইন করুন। Spotify ঐচ্ছিক।
- **ছোট একটি প্রোগ্রাম।** MixMusic একটিমাত্র Go প্রোগ্রাম, যা GPU-তে
  নিজের উইন্ডো নিজেই আঁকে। এর ভেতরে কোনো ব্রাউজার নেই, যা ডাউনলোড করতে
  বা মেমরিতে ধরে রাখতে হবে।
- **আপনার মতো করে বদলানো যায়।** চারটি লেআউট ও চৌদ্দটি থিম সঙ্গেই
  আসে, কমিউনিটি আরও থিম ও প্লাগইন যোগ করে, আর যেকোনো কমান্ডে নিজের
  পছন্দের কী বসানো যায়।
- **আপনার ভাষায়।** ইংরেজি ছাড়াও উইন্ডোটি আসে 66টি ভাষায়, আর আরবি,
  হিব্রু, ফারসি ও উর্দুর জন্য ডান থেকে বাঁয়ে সাজানো হয়।
- **গোপনীয়তা।** MixMusic-এর নিজস্ব কোনো অ্যাকাউন্ট, সার্ভার, অ্যানালিটিক্স
  বা টেলিমেট্রি নেই।
- **বিনামূল্যে ও ওপেন সোর্স,** MIT লাইসেন্স বা GPL 3.0-এর অধীনে।

## ফিচার

MixMusic কী কী করে, এলাকা ধরে ধরে, আর অ্যাপে তা কোথায় পাবেন।
**Built in** মানে এটি নিজে থেকেই কাজ করে। [Features](docs/features.md)-এ
0.1.0 থেকে 0.5.12 পর্যন্ত মোট 363টি ফিচার আছে, প্রতিটির জন্য এক লাইন, সঙ্গে
কোন সংস্করণে এসেছে তা।

### সার্চ ও প্লে

- **দুই সার্ভিসের জন্য একটি সার্চ।** YouTube Music ও Spotify-র ফলাফল
  পাশাপাশি দেখায়, প্রতিটিতে তার সার্ভিসের চিহ্ন থাকে, আর সঙ্গে আপনার
  নিজের গানের ফাইলও। YouTube Music-এর ফলাফলে প্রতিটি গান কতবার বাজানো
  হয়েছে তা দেখায়, ফলে কভার থেকে মূল গানটি আলাদা করে চেনা যায়। *উপরের বার*
- **সব ধরনের ফলাফল।** গান, ভিডিও, অ্যালবাম, শিল্পী, প্লেলিস্ট ও পডকাস্ট,
  সেরা মিল, সবচেয়ে বেশি বাজানো বা সবচেয়ে নতুন অনুযায়ী সাজানো, আর
  মিউজিক ক্যাটালগে নেই এমন লাইভ সেট ও আপলোডের জন্য খোদ YouTube। সার্চ
  বক্সে পেস্ট করা YouTube বা Spotify লিংক সঙ্গে সঙ্গে বাজে। *সার্চ*
- **একটি কিউ।** দুই সার্ভিসের গান মিলিয়ে নিন, সবগুলোর মধ্যে শাফল ও
  রিপিট করুন, কোনো গান পরে বাজানোর জন্য সরিয়ে দিন, আর কিউ ফুরিয়ে গেলে
  রেডিও চালিয়ে যাক। রিস্টার্টের পর কিউ যেমন রেখেছিলেন তেমনই ফিরে আসে।
  *প্লেয়ার বার, কিউ*
- **সবকিছুর জন্য একটি পেজ।** অ্যালবাম, শিল্পী ও প্লেলিস্টে Play ও
  Shuffle, শিল্পীর সব গান, আর যা বাজছে তার সঙ্গে সম্পর্কিত গান।
  *সার্চ, হোম*
- **যা চান না তা বাদ দিন।** গান ডিসলাইক করুন যাতে রেডিও সেগুলো এড়িয়ে
  যায়, AI দিয়ে বানানো গান এবং SponsorBlock-এর চিহ্নিত ভিডিও অংশ বাদ দিন,
  আর এক্সপ্লিসিট কনটেন্ট লুকান বা ক্লিন সংস্করণ বেছে নিন। *প্লেয়ার বার,
  Settings › Playback*
- **স্লিপ টাইমার ও গতি।** নির্দিষ্ট সময়, একটি গান বা কিউ শেষে থামান,
  ধীরে ধীরে আওয়াজ কমিয়ে, বা রাতে নিজে থেকেই, আর গান ও পডকাস্ট দ্রুত
  বা ধীরে বাজান সুরের পিচ না বদলে। *প্লেয়ার বার*
- **মিউজিক ভিডিও ও চলমান কভার।** কভারের বদলে গানের মিউজিক ভিডিও, আর
  কিছু অ্যালবামের লুপ করা ছোট ভিডিও। *এখন বাজছে, Settings › Playback*
- **বেছে নিন, শেয়ার করুন, কাছ থেকে দেখুন।** একসঙ্গে কয়েকটি গান বেছে
  কিউ, যোগ বা ডাউনলোড করুন, কোনো গান বা অ্যালবাম ছবির কার্ড হিসেবে
  শেয়ার করুন, আর দেখুন গানটি কোথা থেকে আসে এবং কেন বাজেনি। *গানের মেনু*

### হোম ও এক্সপ্লোর

- **হোম,** আপনার শেলফ, Jump back in এবং ফলো করা শিল্পীদের নতুন রিলিজ
  নিয়ে, আপনার পছন্দমতো সাজানো। *হোম*
- **Explore (এক্সপ্লোর):** নতুন রিলিজ, চার্ট, মুড ও জনরা। *এক্সপ্লোর*

### লিরিক্স

- **সিঙ্ক করা লিরিক্স,** যা গানের সঙ্গে সঙ্গে চলে, সোর্সে থাকলে শব্দে শব্দে,
  ফুল স্ক্রিনেও, অথবা চাইলে সাধারণ টেক্সট। *লিরিক্স প্যানেল, এখন বাজছে*
- **অনুবাদ ও লাতিন অক্ষর:** প্রতিটি লাইনের নিচে অনুবাদ, জাপানির জন্য
  romaji, আর কোরিয়ান, চীনা, রুশ, আরবি ও আরও অনেক লিপির জন্য লাতিন অক্ষর।
  *লিরিক্স প্যানেল*
- **সোর্স আপনার পছন্দের:** Better Lyrics, Unison, LRCLIB, YouTube Music ও
  YouTube-এর ক্যাপশন, আপনার ঠিক করা ক্রমে, আর এক ক্লিকেই অন্য সোর্স, সার্চ,
  নিজের এডিট ও টাইমিং। *Settings › Playback › Lyrics, লিরিক্স প্যানেল*
- **লিরিক্সে খুঁজুন,** এর লাইন কার্ড হিসেবে শেয়ার করুন, আর আপনার গানের
  ফাইলে থাকা লিরিক্স পড়ুন। *লিরিক্স প্যানেল*

### পডকাস্ট

- **YouTube Music ও Spotify-র পডকাস্ট।** শো খুঁজুন ও ফলো করুন, প্রতিটি
  পর্ব যেখানে ছেড়েছিলেন সেখান থেকে শুরু করুন, পর্ব শোনা হয়েছে বলে চিহ্নিত
  করুন, চ্যাপ্টার ও ট্রান্সক্রিপ্ট পড়ুন, আর গানের থেকে আলাদা গতি ঠিক করুন।
  *সার্চ, শোয়ের পেজ, Library › Podcasts*

### আপনার লাইব্রেরি

- **মিক্স:** যে প্লেলিস্টে সব সার্ভিসের গান থাকে, আপনার নিজের ক্রমে,
  সার্চ ও সর্ট করা যায়, নিজের কভারসহ। *Library › Mixes, সাইডবার*
- **পছন্দের গান** দুই সার্ভিস থেকে একটি তালিকায়, আর আপনার YouTube Music ও
  Spotify প্লেলিস্ট, অ্যালবাম ও শিল্পী, যেগুলো আপনি সংরক্ষণ করতে, বানাতে,
  শেয়ার করতে ও একসঙ্গে সাজাতে পারেন। লাইক ও সংরক্ষণ সার্ভিসেও পৌঁছে যায়।
  *Library*
- **পিন, ফোল্ডার ও ট্যাগ** বড় লাইব্রেরি গুছিয়ে রাখে, আর যা সেখানে চান
  না তা লুকিয়ে রাখা যায়। *সাইডবার, টাইলের মেনু*
- **শিল্পীদের ফলো করুন,** অ্যাকাউন্টসহ বা অ্যাকাউন্ট ছাড়াই, আর তাঁদের
  নতুন রিলিজ দেখুন। *শিল্পীর পেজ, Library › Artists*
- **হিস্ট্রি, স্ট্যাটস ও Your year:** দিনে কত মিনিট, আপনার সেরা গান ও
  শিল্পী, আর শেয়ার করার জন্য সেগুলোর কার্ড। *Library › History, Stats*
- **ডাউনলোড,** নেটওয়ার্ক ছাড়া বাজানোর জন্য, আর আপনার বাজানো গানগুলোর
  অফলাইন ক্যাশ। *গানের মেনু, Library › Downloads*

### এই কম্পিউটারের গান

- **আপনার নিজের গানের ফাইল** লাইব্রেরিতে ও সার্চে, MixMusic যেসব ফোল্ডার
  দেখে সেখান থেকে: MP3, FLAC, WAV, M4A, Opus ও Vorbis, ট্যাগ ও কভারসহ।
  *Settings › Playback › Music on this computer*
- **ট্যাগ এডিট করুন,** একই গানের কপিগুলো একসঙ্গে করুন, আর ফাইল বাজাতে
  খুলুন বা উইন্ডোতে ছেড়ে দিন। *গানের মেনু, উইন্ডোতে ড্রপ*

### অ্যাকাউন্ট ও সার্ভিস

- **YouTube Music।** Google-এর নিজস্ব পেজে, আপনার ব্রাউজারের মাধ্যমে বা
  ব্রাউজারের সেশন দিয়ে সাইন ইন করুন, আপনার অ্যাকাউন্ট হিসেবে বা তার
  কোনো ব্র্যান্ড চ্যানেল হিসেবে, হোম ফিড, পছন্দের গান, প্লেলিস্ট, অ্যালবাম
  ও শিল্পীদের জন্য। আপনি যা বাজান তা আপনার YouTube Music হিস্ট্রিতে যোগ
  হয়, যেখান থেকে তার সুপারিশ আসে। *Settings › Accounts*
- **Last.fm:** আপনি যে গান অর্ধেক পর্যন্ত বা চার মিনিট শোনেন, তা আপনার
  প্রোফাইলে যোগ হয়, আর অফলাইনে হওয়া স্ক্রোবল অনলাইনে ফিরলে পাঠানো হয়।
  *Settings › Accounts*
- **ListenBrainz:** একই কাজ, Last.fm-এর পাশাপাশি বা তা ছাড়াই, ListenBrainz-এ
  বা আপনার নিজের কোনো সার্ভারে, যেমন Maloja। লাইক করা গান এদের যেকোনোটিতে
  love হিসেবে যেতে পারে। *Settings › Accounts*
- **Libre.fm:** একই কাজ, Libre.fm-এ বা অন্য কোনো GNU FM সার্ভারে, কোনো API
  অ্যাকাউন্ট তৈরি না করেই। *Settings › Accounts*
- **Discord:** আপনার প্রোফাইলে কী বাজছে তা আর্টওয়ার্ক ও বাকি সময়সহ দেখায়,
  আপনার পছন্দমতো। *Settings › Desktop*

### Spotify, তিনভাবে

- **MixMusic-এর ভেতরেই,** Spotify-র মানে, কোনো Spotify অ্যাপ না খুলেই
  (Premium লাগে, Windows ও macOS-এ)।
- **Spotify Connect-এ:** আপনার Spotify অ্যাপ, ফোন বা স্পিকারে, Spotify
  অ্যাপে লসলেস চালু থাকলে লসলেসে।
- **YouTube Music থেকে:** MixMusic Spotify-র গানটির রেকর্ডিং YouTube
  Music-এ বাজায়, যা বাজাতে Premium লাগে না।

বেছে নিন Settings › Playback › Spotify tracks-এ।

### সাউন্ড

- **দশ ব্যান্ডের ইকুয়ালাইজার,** 23টি প্রিসেট ও আপনার নিজের প্রিসেটসহ, Bass,
  Mid ও Treble-এর একটি সহজ মোড, আর AutoEq থেকে 8,800-এরও বেশি হেডফোনের
  সংশোধন। *Settings › Playback › Equalizer*
- **ক্রসফেড, গ্যাপলেস অ্যালবাম, সমান লাউডনেস ও ভলিউম লেভেলিং।**
  *Settings › Playback › Sound*
- **রিভার্ব, ইকো, ক্রসফিড, ব্যালান্স ও স্টেরিও প্রস্থ।** *Settings ›
  Playback*
- **5.1 সারাউন্ড,** যেখানে গানে তা আছে, আর আপনার পছন্দের আউটপুট ডিভাইস।
  *প্লেয়ার বার, Settings › Playback › Sound*
- **একটি ভিজ্যুয়ালাইজার,** MixMusic-এর নিজস্ব অডিও ইঞ্জিন থেকে আঁকা। *এখন বাজছে*

### কাস্টিং, বন্ধু ও অন্য কম্পিউটার

- **কাস্ট করুন** Chromecast, Google Home ও Nest স্পিকার ও তাদের গ্রুপে, DLNA
  ও UPnP রেন্ডারার এবং FCast রিসিভারে, আপনার নিজের গানের ফাইলও।
  *প্লেয়ার বার, মিনি প্লেয়ার*
- **বন্ধুদের সঙ্গে শুনুন:** এমন একটি রুম চালান যা আপনার বন্ধুদের
  কম্পিউটারে তাল মিলিয়ে বাজে, আপনার নেটওয়ার্কে বা ইনভাইট লিংক দিয়ে যেকোনো
  জায়গায়। *প্লেয়ার বারের কাস্ট মেনু*
- **রিমোট কন্ট্রোল:** আপনার নেটওয়ার্কের অন্য কম্পিউটারের MixMusic
  নিয়ন্ত্রণ করুন, আর যা বাজছে তা এক কম্পিউটার থেকে আরেকটিতে সরান।
  *প্লেয়ার বারের কাস্ট মেনু, Settings › Desktop*
- **AI অ্যাসিস্ট্যান্ট** আপনার কম্পিউটারে Model Context Protocol-এর মাধ্যমে
  জানতে পারে কী বাজছে, সার্চ করতে, গান চালাতে ও স্লিপ টাইমার ঠিক করতে পারে।
  *Settings › Desktop › AI assistants*

### আপনার ডেস্কটপে

- **একটি মিনি প্লেয়ার,** যা অন্য উইন্ডোগুলোর ওপরে থাকে, আর তার প্যানেলে
  কিউ ও লিরিক্স। *প্লেয়ার বার, <kbd>Ctrl</kbd> <kbd>M</kbd>*
- **মিডিয়া কী** এবং Windows, macOS ও Linux-এ সিস্টেমের মিডিয়া কন্ট্রোল।
  *Built in*
- **গ্লোবাল শর্টকাট,** আপনার নিজের, প্লে, পজ, স্কিপ, ভলিউম বদল, মিউট, চলতি
  গানে লাইক বা ডিসলাইক, অথবা MixMusic দেখানোর জন্য, যে উইন্ডোই সামনে থাকুক।
  *Settings › Shortcuts*
- **প্লেয়ারের বোতাম** Windows-এ টাস্কবারের থাম্বনেইলে, আর MacBook-এর নচের
  চারপাশে একটি প্লেয়ার। *Built in, Settings › Desktop*
- **ট্রে কন্ট্রোল,** নতুন গানের নোটিফিকেশন, সবসময় ওপরে রাখা এবং লগইনের
  সময় চালু হওয়া। *Settings › Desktop*
- **`mixmusic://` লিংক** এবং `--next`-এর মতো ফ্ল্যাগ, Stream Deck-এর কী বা
  স্ক্রিপ্টের জন্য। *কমান্ড লাইন*
- **ওভারলের জন্য এখন বাজছে:** OBS ব্রাউজার সোর্সের জন্য কী বাজছে তার একটি
  কার্ড, আর আপনার নিজের পেজের জন্য তার ডেটা। *Settings › Desktop*

### নিজের মতো সাজান

- **লেআউট।** ডিফল্ট Blend-এ আছে গোল আকার আর বড় কভারের ঘন সারিতে
  স্প্রিংয়ের মতো গতি। Expressive, Material 3 Expressive-এর আদলে, খেলাচ্ছলে
  ভরা, আছে টোনাল ফিল, পিল বোতাম ও ভাসমান প্লেয়ার বার। Crate সংযত, যাতে
  কভারগুলোই কাজ করে, আর Tiles হলো MixMusic-এর প্রথম চেহারা, সমতল টাইল
  ও বিশাল ছোট হাতের অক্ষরের শব্দ নিয়ে। *Settings › Appearance*
- **থিম।** চৌদ্দটি বিল্ট-ইন, প্রতিটি যেকোনো লেআউটে চলে, বদলানো যায় এমন
  ইফেক্টসহ, লাইট মোডের জন্য একটি ও ডার্ক মোডের জন্য একটি থিম, আর নিজের
  বা বাজতে থাকা আর্টওয়ার্ক থেকে নেওয়া অ্যাক্সেন্ট রঙ। Material ও
  Material Dark প্রতিটি রং কভার থেকে নেয়। *Settings › Appearance*
- **কমিউনিটি থিম ও প্লাগইন,** মার্কেটপ্লেসে এক ক্লিকেই পাওয়া যায়।
  *মার্কেটপ্লেস*
- **প্লাগইন** টাইপড API-র মাধ্যমে পেজ, কমান্ড, ট্র্যাক অ্যাকশন ও উইজেট
  যোগ করে, যা আপনার থিমেই আঁকা হয়। *Settings › Plugins*
- **কীবোর্ড আগে।** সাধারণ সবকিছুর শর্টকাট, যেকোনো কমান্ডের জন্য নিজের কী,
  প্লাগইনের কমান্ডসহ, যেকোনো অ্যাপ থেকে গান নিয়ন্ত্রণের কী, আর বাকিটার
  জন্য একটি কমান্ড প্যালেট (<kbd>Ctrl</kbd> <kbd>K</kbd>)।
- **আপনার ভাষা।** ইংরেজি ছাড়াও 66টি ভাষা, যে ভাষা ডান থেকে বাঁয়ে পড়া হয়
  সেখানে ডান থেকে বাঁয়ে, আর তারিখ ও সংখ্যা আপনার অঞ্চলের নিয়মে।
  *Settings › Appearance › Language*
- **স্ক্রিন রিডার ও কীবোর্ডের জন্য।** স্ক্রিন রিডারের জন্য নাম ও ঘোষণা,
  কীবোর্ডে নাগালের মধ্যে প্রতিটি কন্ট্রোল, টেক্সটের আকার, জুম ও কম মোশন।
  *Built in, Settings › Appearance*
- **একটি সেটিংস ফাইল।** আপনার সেটিংস থাকে ডেটা ফোল্ডারের `settings.json`-এ
  (Windows-এ `%AppData%\MixMusic`, macOS-এ `~/Library/Application Support/MixMusic`,
  Linux-এ `~/.config/MixMusic`), যা আপনি নিজের ডটফাইলের সঙ্গে রাখতে পারেন।
  MixMusic চলার সময় সেভ করলে তা সঙ্গে সঙ্গে প্রযোজ্য হয়। *Settings › Desktop
  › Settings file*

### ব্যাকআপ, ইমপোর্ট ও এক্সপোর্ট

- **ব্যাকআপ ও রিস্টোর** আপনার লাইব্রেরি, সেটিংস ও অ্যাড-অন, হাতে বা নিজে
  থেকে, এই কম্পিউটারে বা অন্যটিতে। *Settings › Desktop › Backups*
- **প্লেলিস্ট ইমপোর্ট করুন** ফাইল থেকে এবং আপনার Spotify ও Google ডেটা
  থেকে, আর তালিকা ও লাইব্রেরি এক্সপোর্ট করুন অন্য প্লেয়ারের পড়তে পারা ফাইলে।
  *Library › Mixes, পেজের More মেনু*

### আপডেট

- MixMusic চালু হওয়ার সময় এবং প্রতি কয়েক ঘণ্টা পরপর নতুন রিলিজ আছে কি না
  দেখে, তার সিগনেচার যাচাই করে, এবং আপনি বললে, বা আপনি চাইলে নিজে থেকেই,
  তা ইনস্টল করে। *Settings › About*

## এক নজরে

<p align="center">
  <img src="docs/look-listen.webp" alt="Blend লেআউটে MixMusic-এর পেজ, একটি অন্ধকার মঞ্চে সোজা দাঁড়িয়ে আপনার দিকে মুখ করে আছে: মাঝখানে YOASOBI-র アイドル বাজছে, গোলাপি রেখায় ঘেরা, লিরিক্সের প্রতিটি লাইনের নিচে romaji ও ইংরেজি অনুবাদ; বাঁ দিকে THE BOOK 3 অ্যালবাম আর ডান দিকে ফলো করা একটি পডকাস্টের পেজ" />
</p>

<p align="center"><sub><b>শুনুন।</b> মাঝখানে YOASOBI-র アイドル বাজছে: গাওয়ার সঙ্গে সঙ্গে প্রতিটি লাইন জ্বলে ওঠে, নিচে romaji ও অনুবাদ থাকে। বাঁ দিকে একটি অ্যালবাম; ডান দিকে আপনার ফলো করা একটি পডকাস্ট।</sub></p>

<p align="center">
  <img src="docs/look-yours.webp" alt="MixMusic-এর আরও কিছু, একটি অন্ধকার মঞ্চে সোজা দাঁড়িয়ে আপনার দিকে মুখ করে আছে: মাঝখানে চারটি লেআউটসহ Settings › Appearance, গোলাপি রেখায় ঘেরা; বাঁ দিকে বাজতে থাকা কভারের রঙে রাঙানো একটি Material থিম, আর ডান দিকে মার্কেটপ্লেসের কমিউনিটি থিম" />
</p>

<p align="center"><sub><b>নিজের মতো সাজান।</b> মাঝখানে চারটি লেআউট: ডিফল্ট Blend, Expressive, Crate ও Tiles, থিমগুলোর নিচে। বাঁ দিকে একটি Material থিম, যা বাজতে থাকা কভার থেকে রং নেয়। ডান দিকে মার্কেটপ্লেসের কমিউনিটি থিম।</sub></p>

## ডকুমেন্টেশন

গাইডগুলো ওয়েবসাইটে আছে,
[mixmusic.functionx.dev/docs](https://mixmusic.functionx.dev/docs/)-এ, এবং
[fzn0x.github.io/mixmusic/docs](https://fzn0x.github.io/mixmusic/docs/)-এও,
যেখানে সার্চ করে যেকোনোটি পাওয়া যায়, আর এখানে [`docs/`](docs)-এ।

- **শুরু করা:** [Getting started](docs/getting-started.md),
  [Features](docs/features.md), [Accounts](docs/accounts.md),
  [Spotify](docs/spotify.md), [Troubleshooting](docs/troubleshooting.md)
- **MixMusic ব্যবহার:** [Playing music](docs/playing-music.md),
  [Home](docs/home.md), [Your library](docs/library.md),
  [Import and export playlists](docs/import-export.md),
  [Music on this computer](docs/local-music.md), [Lyrics](docs/lyrics.md),
  [Podcasts](docs/podcasts.md), [Look and feel](docs/look-and-feel.md),
  [On the desktop](docs/desktop.md),
  [Overlays for streams](docs/stream-overlay.md),
  [Casting](docs/casting.md), [Remote control](docs/remote-control.md),
  [Listen with friends](docs/listen-with-friends.md),
  [AI assistants](docs/ai-assistants.md),
  [Marketplace](docs/marketplace.md), [Settings](docs/settings.md)
- **কীভাবে কাজ করে:** [How MixMusic works](docs/how-it-works.md),
  [The audio engine](docs/audio-engine.md),
  [Where the audio streams come from](docs/streams.md),
  [How Spotify tracks play](docs/spotify-playback.md),
  [Your data and privacy](docs/data-and-privacy.md)
- **নিজে বানান:** [Themes](docs/themes.md), [Plugins](docs/plugins.md)
- **প্রজেক্ট:** [Credits](docs/credits.md), [License](docs/license.md),
  [Architecture](docs/architecture/README.md),
  [Contributing](CONTRIBUTING.md), [Code of conduct](CODE_OF_CONDUCT.md),
  [Security](SECURITY.md)

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
`mixmusic-<version>-darwin-universal.tar.gz` বা `-windows-<arch>.tar.gz`
নামের ফাইলগুলো আপনার লাগবে না: ইনস্টল করা অ্যাপ ওগুলো থেকেই আপডেট হয়।

ইনস্টলার, পোর্টেবল zip ও Linux আর্কাইভ নিজে নিজেই আপডেট হয়। `.deb`,
`.rpm`, AppImage ও Flatpak নিজেদের ফাইল বদলাতে পারে না, তাই নতুন
সংস্করণ এলে MixMusic আপনাকে জানায় এবং আপনার জন্য সেটি ডাউনলোড করে।
AUR-এর `mixmusic-bin` আপনার বাকি সিস্টেমের সঙ্গে আপডেট হয়। পোর্টেবল
zip-এর জন্য, যেকোনো জায়গায় আনজিপ করে `MixMusic.exe` খুলুন। AppImage-এর
জন্য, এটি এক্সিকিউটেবল করে (`chmod +x`) খুলুন। Flatpak-এর জন্য, এটি আপনার
সফটওয়্যার সেন্টারে খুলুন, অথবা চালান
`flatpak install ./mixmusic-<version>-linux-<arch>.flatpak`; প্রথমবার
Flatpak Flathub থেকে GNOME রানটাইম নামিয়ে আনে।

Mac-এ [Homebrew](https://brew.sh)-ও MixMusic ইনস্টল করে, আর `brew upgrade`
তা আপডেট করে:

```sh
brew install --cask fzn0x/tap/mixmusic
```

[Getting started](docs/getting-started.md) প্রতিটি সিস্টেমে ইনস্টল ও প্রথমবার
চালু করার ধাপ দেখিয়ে দেয়।

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

  MixMusic-এর সঙ্গে চীনা, জাপানি ও কোরিয়ান অক্ষরের কোনো ফন্ট আসে না, আর
  Linux-এ কোনো ইনস্টল করা ফন্টে যে অক্ষর নেই তা ফাঁকা জায়গা হয়ে দেখা দেয়।
  `.deb` ও `.rpm` একটি ফন্ট চেয়ে নেয়; অন্যগুলোতে নিজে একটি ইনস্টল করে
  MixMusic রিস্টার্ট করুন:

  ```sh
  # Debian and Ubuntu
  sudo apt install fonts-noto-cjk
  # Fedora
  sudo dnf install google-noto-sans-cjk-vf-fonts
  # Arch
  sudo pacman -S noto-fonts-cjk
  ```
- **সব সিস্টেমে:** YouTube Music-এর বেশিরভাগ গানের অডিও MixMusic নিজেই
  খুঁজে নেয়। বাকিগুলোর জন্য, যেমন শিশুদের জন্য বানানো গান, এটি
  [yt-dlp](https://github.com/yt-dlp/yt-dlp) ব্যবহার করে, যা Settings ›
  Playback (সেটিংস › প্লেব্যাক) থেকে এটি আপনার জন্য ইনস্টল করে এবং আপ টু ডেট
  রাখে। yt-dlp-র জন্য একটি JavaScript রানটাইম লাগে:
  [Deno](https://deno.com), Node.js, Bun বা QuickJS। Flatpak-এ Node.js আছে।

### Flatpak

Flatpak MixMusic-কে একটি স্যান্ডবক্সে চালায়, যা নেটওয়ার্ক, সাউন্ড সার্ভার,
ডিসপ্লে ও GPU দেখতে পায়, কিন্তু আপনার হোম ফোল্ডারের কিছুই নয়, শুধু Flathub
থেকে ইনস্টল করা ব্রাউজারগুলোর প্রোফাইল ছাড়া, যা কেবল পড়ার জন্য, আর সেখান
থেকে YouTube Music সাইন-ইন ইমপোর্ট করা যায়। এটি x64 ও Arm64-এর জন্য
আসে, যেমন Raspberry Pi 5, Asahi Linux ও Snapdragon ল্যাপটপ। MixMusic আপাতত
Flathub-এ নেই: রিলিজের `.flatpak` ইনস্টল করুন।

Flatpak তার সেটিংস ও লাইব্রেরি রাখে `~/.var/app/io.github.fzn0x.mixmusic`-এ,
`.deb`, `.rpm`, AppImage বা আর্কাইভ কপি থেকে আলাদাভাবে, আর দুটিই একসঙ্গে
চলতে পারে। কোনো কপির সেটিংস ও লাইব্রেরি নিয়ে আসতে, Settings › Desktop ›
Backups-এ তার ব্যাকআপ নিন এবং Flatpak-এ সেই ফাইল রিস্টোর করুন। হাতে ফাইল
কপি করতে চাইলে, দুটিই বন্ধ করুন, তারপর `~/.config/MixMusic` থেকে
`settings.json`, `library.json`, `plays.jsonl`, `local-copies.json`,
`queue.json`, `positions.json` এবং `addons` ও `mix-covers` ফোল্ডার কপি করে
`~/.var/app/io.github.fzn0x.mixmusic/config/MixMusic`-এ রাখুন। আগে দুটিই বন্ধ
করুন: চলমান MixMusic `library.json` ও বেশিরভাগ ফাইল মেমরিতে রাখে এবং পরের সেভে
কপি করা ফাইল ওভাররাইট করে দেয়। তারপর আপনার অ্যাকাউন্টগুলোতে আবার সাইন ইন
করুন: সাইন-ইন থাকে অন্য কপির কীরিংয়ে। প্রতিটি ফাইলে কী আছে তা
[ডেটা ও গোপনীয়তার পাতা](docs/data-and-privacy.md#the-data-folder)-য় বলা আছে।

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
- **ListenBrainz:** আপনার [ListenBrainz সেটিংস](https://listenbrainz.org/settings/)
  বা নিজের সার্ভারের user token Settings › Accounts (সেটিংস › অ্যাকাউন্ট)-এ
  পেস্ট করুন।
- **Libre.fm:** Settings › Accounts (সেটিংস › অ্যাকাউন্ট)-এ যুক্ত করুন, এবং
  আপনার ব্রাউজারে Libre.fm-এর পাতায়, বা অন্য GNU FM সার্ভারের পাতায়,
  MixMusic-কে অনুমোদন দিন।
- **Discord:** Discord অ্যাপ চলার সময় আপনার প্রোফাইলে আপনি কী শুনছেন তা
  দেখায়। Settings › Desktop (সেটিংস › ডেস্কটপ)-এ এটি বন্ধ করুন।

[Accounts](docs/accounts.md) প্রতিটি সাইন-ইন ধাপে ধাপে বলে।

### জানা সীমাবদ্ধতা

- **Arm-এ Windows** x64-এর চেয়ে কম পরীক্ষিত, আর পোর্টেবল zip `mixmusic://`
  লিংক নিবন্ধন করে না।
- **ডেভেলপমেন্ট মোডের Spotify অ্যাপ,** যেমন আপনি যেটি দিয়ে সাইন ইন করেন,
  প্রতিটি ধরনের দশটি করে সার্চ ফলাফল পায়, আর আপনার নয় এমন প্লেলিস্টের
  গানগুলো লুকানো থাকে।
- **Linux-এ** Spotify ডিফল্টভাবে Spotify Connect-এর মাধ্যমে বাজে, কারণ
  WebKitGTK Spotify-র সুরক্ষিত অডিও বাজাতে পারে না।
- **YouTube** তার ইন্টারফেস বদলে দিতে পারে, ফলে MixMusic বা yt-dlp তাল
  মেলানো পর্যন্ত সার্চ, প্লেব্যাক বা সাইন-ইন ভেঙে যেতে পারে।

কিছু কাজ করছে না? [Troubleshooting](docs/troubleshooting.md)-এ সাধারণ
সমস্যা ও জানা সমস্যাগুলো আছে।

## সোর্স থেকে বিল্ড

আপনার লাগবে [Go](https://go.dev/dl/) 1.27.3 বা তার পরের, [Bun](https://bun.sh)
1.4 বা তার পরের, যা MyGo-র কমান্ড লাইন চালায়, এবং Git। MyGo-র কোনো C
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
দিয়ে রিলিজ বিল্ড হয়, `flake.lock`-এ পিন করা সংস্করণে: Go 1.27, Bun,
GoReleaser, jq, zip ও Zola, আর Linux-এ NSIS, osslsigncode ও binutils।
`nix build` সোর্স থেকে Linux-এর জন্য MixMusic বিল্ড করে, `result/`-এ, আর
`nix run github:fzn0x/mixmusic` সেটি চালায়।

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

[ওয়েবসাইট](https://mixmusic.functionx.dev/) থাকে `site/`-এ, যা
[Zola](https://www.getzola.org) দিয়ে বিল্ড করা, আর `docs/`-এর গাইডগুলো এর
`/docs/` পেজ। `go run ./tools/sitedocs && zola --root site serve` এর প্রিভিউ
দেখায়। রিলিজ বিল্ড হয় Nix শেলে এবং প্রকাশ করে
[GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`)।

[CONTRIBUTING.md](CONTRIBUTING.md)-এ আছে পুল রিকোয়েস্টের আগে কোন চেক চালাতে
হবে, আর [docs/architecture](docs/architecture/README.md) ব্যাখ্যা করে অংশগুলো
কীভাবে একসঙ্গে মেলে, প্রতিটির জন্য একটি পেজ।

### লেআউট

অ্যাপটি হলো রুট প্যাকেজ, আর তার নিচে `internal/`-এর প্যাকেজগুলো। `internal/`-এর
বাইরের প্যাকেজগুলো যে কেউ ইমপোর্ট করতে পারে, এবং
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic)-এ ডকুমেন্ট করা।

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

[docs/architecture](docs/architecture/README.md) `internal/`-এর বাকি
প্যাকেজগুলোর কথা বলে, অ্যাপের প্রতিটি অংশের জন্য একটি পেজ।

যেসব প্যাকেজ ইমপোর্ট করা যায়:

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
2. একটি ফোল্ডার যোগ করুন: থিমের জন্য `community/themes/<id>/`, এর
   `theme.json` সহ, অথবা প্লাগইনের জন্য `community/plugins/<id>/`, এর
   `plugin.json` ও `index.ts` সহ।
3. চালান `bun install`, `bun run build:plugins`, `go run ./tools/registry`
   এবং `go run ./tools/registry -check`।
4. একটি স্ক্রিনশটসহ পুল রিকোয়েস্ট খুলুন।

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
  বাজান, যা আপনার YouTube Music হিস্ট্রিতে যোগ হয়। খোদ YouTube, সাইন আউট
  অবস্থায় ও কুকি ছাড়া, সার্চের YouTube বিকল্পের জন্য, AI দিয়ে বানানো
  গানের যাচাইয়ের জন্য, লিরিক্স ও ট্রান্সক্রিপ্ট হিসেবে ক্যাপশনের জন্য, আর
  মিউজিক ভিডিওর সঙ্গে সময় মেলানো লিরিক্সের জন্য।
- **Spotify,** সাইন ইন করার পর: আপনার লাইব্রেরি, আপনার Spotify Connect
  ডিভাইস, এবং Spotify-র প্লেয়ার, `sdk.scdn.co` থেকে লোড হয়, যখন
  MixMusic-এ Spotify বাজে।
- **Better Lyrics:** যে গান বাজছে তার শিরোনাম, প্রথম শিল্পী, অ্যালবাম ও
  দৈর্ঘ্য, এর লিরিক্স খুঁজতে, গানটিতে অ্যালবাম ও দৈর্ঘ্য থাকলে।
- **Unison:** যে গান বাজছে তার ভিডিও আইডি, অথবা সেটি না থাকলে তার শিরোনাম,
  শিল্পীরা, অ্যালবাম ও দৈর্ঘ্য, লিরিক্স খুঁজতে। শুধু তখনই, যখন আপনি লিরিক্সের
  পাশে translate চালু করেন, বা আরবি romanize করেন, বাজতে থাকা গানের লিরিক্স,
  তার ভিডিও আইডিসহ, যা Unison Google দিয়ে অনুবাদ বা romanize করায়।
- **LRCLIB:** যে গান বাজছে তার শিরোনাম, শিল্পীরা, অ্যালবাম ও দৈর্ঘ্য,
  এর লিরিক্স খুঁজতে।
- **SponsorBlock,** যখন আপনি Skip the parts SponsorBlock marks
  (SponsorBlock-এর চিহ্নিত অংশ বাদ দিন) চালু করেন: প্রতিটি ভিডিও আইডির
  হ্যাশের প্রথম 4টি অক্ষর, আইডি কখনো নয়, এর চিহ্নিত অংশগুলো খুঁজতে।
- **GitHub:** চালু হওয়ার সময় এবং প্রতি কয়েক ঘণ্টা পরপর নতুন সংস্করণের
  খোঁজ, যা আপনি Settings › About (সেটিংস › সম্পর্কে)-এ বন্ধ করতে পারেন;
  আপনি মার্কেটপ্লেস খুললে কমিউনিটি রেজিস্ট্রি; yt-dlp-র ডাউনলোড; AutoEq-এর
  হেডফোনের তালিকা এবং আপনার বেছে নেওয়া প্রোফাইল; এবং জাপানি লিরিক্সের
  পাশে প্রথমবার romaji চালু করলে, কানজির পাঠের অভিধান (11.7 MB)। Romaji এবং
  আরবি ছাড়া অন্য লিপির লাতিন অক্ষর তৈরি হয় আপনার কম্পিউটারেই: লিরিক্স
  পাঠানো হয় না।
- **Last.fm,** আপনি যুক্ত করলে: আপনি কী বাজান।
- **ListenBrainz,** বা এর API চালানো আপনার নিজের সার্ভার, আপনি যুক্ত করলে:
  আপনি কী বাজান।
- **Libre.fm,** বা অন্য কোনো GNU FM সার্ভার, আপনি যুক্ত করলে: আপনি কী বাজান।
- **Discord,** বন্ধ না করলে: কী বাজছে, আপনার কম্পিউটারের Discord অ্যাপে
  পাঠানো হয়, যা তা আপনার প্রোফাইলে দেখায়।
- **আপনি যেসব ডিভাইসে কাস্ট করেন,** আপনার নেটওয়ার্কে: যা বাজছে তার শিরোনাম,
  শিল্পী ও কভার। অডিও তারা MixMusic থেকে নেয়।
- **MixMusic চলা অন্য কম্পিউটার,** রিমোট কন্ট্রোল ও বন্ধুদের সঙ্গে শোনার জন্য,
  এনক্রিপ্ট করে, আপনার নেটওয়ার্কে, বা যতক্ষণ আপনি বাইরের বন্ধুদের যোগ দিতে
  দেন, ইন্টারনেটে। তখন আপনার রাউটারকে একটি পোর্ট ফরোয়ার্ড করতে বলা হয়, আর
  আপনার ইনভাইট লিংকে থাকে আপনার ইন্টারনেট ঠিকানা।
- **AI অ্যাসিস্ট্যান্ট** আপনার কম্পিউটারে, যতক্ষণ আপনি তাদের অনুমতি দেন: কী
  বাজছে, কিউ, তাদের সার্চের ফলাফল ও আপনার প্লেলিস্টের নাম, যা অ্যাসিস্ট্যান্ট
  তার ব্যবহৃত AI মডেলকে পাঠিয়ে দিতে পারে।
- **YouTube ও Spotify,** নেটওয়ার্ক সাড়া দিচ্ছে কি না জানতে: HTTPS-এর মাধ্যমে
  একটি ফাঁকা উত্তরের অনুরোধ।

প্লাগইন যা খুশি পাঠাতে পারে: প্লাগইনের পারমিশন আপনাকে দেখানো হয়, জোর করে
প্রয়োগ করা হয় না, আর সেজন্যই MixMusic চালু করার আগে প্লাগইনকে বিশ্বাস করতে
বলে। [Your data and privacy](docs/data-and-privacy.md#what-each-feature-sends-where)-তে
একটি টেবিল আছে, প্রতিটি ফিচার কী কোথায় পাঠায় তার।

আপনার কম্পিউটারে যা রাখে:

- আপনার সেটিংস ও লাইব্রেরি (পছন্দের ও অপছন্দের গান, মিক্স, আপনার স্ট্যাটসে
  গোনা প্রতিটি বাজানো, ফলো করা শিল্পী ও শো, পিন, ফোল্ডার ও ট্যাগ, হিস্ট্রি,
  সাম্প্রতিক সার্চ এবং আপনার গানের ফাইলের ইনডেক্স), Settings › About-এ যে
  ফোল্ডারের নাম আছে সেখানে।
- আপনার বাজানো গানগুলো, ক্যাশে, Settings › Playback (সেটিংস › প্লেব্যাক)-এ
  যতটা অনুমতি দেন ততটা, এবং YouTube সেগুলো কতটা জোরে মেপেছে। আপনার
  ডাউনলোড করা গানগুলো, ডাউনলোড ফোল্ডারে।
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
সাইন-ইন মুছে যায়। [Clear, move or reset](docs/data-and-privacy.md#clear-move-or-reset)
বলে কীভাবে আপনার ডেটা সরাতে হয়, `MIXMUSIC_DATA_DIR` দিয়ে পোর্টেবল কপি রাখতে
হয়, বা নতুন করে শুরু করতে হয়।

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
- **[SponsorBlock](https://sponsor.ajay.app),** YouTube ভিডিওর অংশগুলোর
  ওপেন, ক্রাউডসোর্সড ডেটাবেস, Ajay Ramachandran ও এর অবদানকারীদের তৈরি,
  যার চিহ্নিত অংশ আপনি চাইলে MixMusic বাদ দেয়। Uses SponsorBlock data
  licensed under
  [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/)
  from https://sponsor.ajay.app/.
- **[Better Lyrics](https://betterlyrics.org),** যার সেবা শব্দে শব্দে সিঙ্ক
  করা লিরিক্স রাখে।
- **[Unison](https://unison.betterlyrics.org),** লিরিক্সের ওপেন, ক্রাউডসোর্সড
  ডেটাবেস, Unison Community License-এর অধীনে, এবং **[LRCLIB](https://lrclib.net),** সিঙ্ক করা
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
- **[Chromium](https://www.chromium.org)-এর** Web Audio, যার biquad,
  DynamicsCompressor ও AnalyserNode-এর পোর্ট হলো ইকুয়ালাইজারের ফিল্টার,
  ভলিউম লেভেলিং ও ভিজ্যুয়ালাইজার।
- **[AutoEq](https://github.com/jaakkopasanen/AutoEq),** Jaakko Pasanen ও এর
  অবদানকারীদের তৈরি, যার প্রোফাইল হেডফোনের সাউন্ড সংশোধন করে,
  [Equalizer APO](https://sourceforge.net/projects/equalizerapo/)-র ফরম্যাটে।
- **[go-mp3](https://github.com/hajimehoshi/go-mp3)** ও
  **[jfreymuth/vorbis](https://github.com/jfreymuth/vorbis),** যা আপনার
  কম্পিউটারের MP3 ও Vorbis ফাইল ডিকোড করে, এবং
  **[Xiph.Org](https://xiph.org),** যার FLAC ও Ogg `flac` ও `ogg` পড়ে।
- **[FFmpeg](https://ffmpeg.org),** যার সঙ্গে ডিকোডারগুলোর টেস্ট তুলনা করে।
- **[Selawik](https://github.com/microsoft/Selawik),**
  **[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily),**
  **[Roboto Flex](https://github.com/googlefonts/roboto-flex)** ও
  **[Archivo](https://github.com/Omnibus-Type/Archivo),** উইন্ডোগুলোর
  টাইপফেস।
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities),**
  যার HCT কালার সায়েন্সের পোর্ট হলো Material থিমগুলোর রং।
- **[FCast](https://docs.fcast.org),** যে উন্মুক্ত প্রোটোকল দিয়ে MixMusic FCast
  রিসিভারে বাজায়। MixMusic তার একটি স্বাধীন বাস্তবায়ন, এবং FUTO-র সঙ্গে যুক্ত
  নয়; FCast FUTO-র নিবন্ধিত ট্রেডমার্ক।
- **[Kevin MacLeod](https://incompetech.com),** যার সংগীত,
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)-এর অধীনে,
  ওয়েবসাইটে বাজে।

[Credits](docs/credits.md) প্রতিটি প্রজেক্ট, গবেষণাপত্র ও স্ট্যান্ডার্ডের নাম
বলে, যার ওপর MixMusic দাঁড়িয়ে আছে।

## অবদান রাখা

ফিক্স, ফিচার, থিম, প্লাগইন ও বাগ রিপোর্ট সবই স্বাগত।

- **সমস্যা জানান** Settings › About › report a problem (সেটিংস › সম্পর্কে ›
  সমস্যা জানান) থেকে। এটি আপনার সংস্করণ, সিস্টেম, থিম ও প্লাগইন ভরা একটি
  নতুন GitHub issue খোলে। অথবা নিজে [একটি issue খুলুন](https://github.com/fzn0x/mixmusic/issues/new)।
- **পরিবর্তন পাঠান** পুল রিকোয়েস্ট হিসেবে। [CONTRIBUTING.md](CONTRIBUTING.md)
  ব্যাখ্যা করে কোন চেক চালাতে হবে, কমিটের ধরন, এবং মানুষের চোখে পড়ার মতো
  পরিবর্তনে যে চেঞ্জলগ এন্ট্রি লাগে তা।
- **থিম বা প্লাগইন শেয়ার করুন:** দেখুন [কমিউনিটি](#কমিউনিটি)।
- **নিরাপত্তা দুর্বলতা জানান** গোপনে, যেভাবে [SECURITY.md](SECURITY.md)
  বলে।

অংশ নেওয়া সবাই [আচরণবিধি](CODE_OF_CONDUCT.md) মেনে চলেন।
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
অধীনে। অন্য অংশগুলো তাদের উৎসের লাইসেন্স ধরে রাখে, যেমন Chromium-এর Web
Audio-র ফিল্টার, go-mp3, jfreymuth/vorbis, kagome ও Unihan ডেটা:
[License](docs/license.md) প্রতিটির তালিকা দেয়।

MixMusic Spotify বা Google-এর সঙ্গে সম্পর্কিত নয়, তাদের দ্বারা সমর্থিত বা
পৃষ্ঠপোষকতাপ্রাপ্তও নয়। এই সার্ভিসগুলো তাদের ব্যবহারকারীদের যা দেয়, এটি
সেই সার্ভিসগুলোর নিজস্ব ইন্টারফেসের মাধ্যমে তা-ই বাজায়; তাদের শর্ত যতটা
অনুমতি দেয় সেভাবেই ব্যবহার করুন। Spotify ও YouTube Music তাদের মালিকদের
ট্রেডমার্ক।
