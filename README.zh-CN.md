<p align="center"><a href="README.md">English</a> · <a href="README.hi.md">हिन्दी</a> · <strong>简体中文</strong> · <a href="README.id.md">Bahasa Indonesia</a> · <a href="README.ur.md">اردو</a> · <a href="README.ha.md">Hausa</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.bn.md">বাংলা</a> · <a href="README.ru.md">Русский</a> · <a href="README.am.md">አማርኛ</a></p>

<p align="center">
  <img src="brand/icon.png" width="112" height="112" alt="MixMusic 的图标：黑色背景上的两个粉色圆环和一个播放按钮" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center"><strong>一个播放器，同时播放 YouTube Music 和 Spotify。</strong></p>

<p align="center">适用于 macOS、Windows 和 Linux 的 YouTube Music 与 Spotify 客户端。YouTube Music 无需账号即可使用。统一的搜索和播放队列，还有混合歌单、同步歌词、播客、你自己的音乐文件、投放、主题和插件。</p>

<p align="center"><sub>本文译自<a href="README.md">英文 README</a>，如有出入，以最新的英文版为准。</sub></p>

<p align="center">
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml/badge.svg" alt="网站" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases/latest"><img src="https://img.shields.io/github/v/release/fzn0x/mixmusic" alt="最新版本" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases"><img src="https://img.shields.io/github/downloads/fzn0x/mixmusic/total" alt="下载量" /></a>
  <a href="https://pkg.go.dev/github.com/fzn0x/mixmusic"><img src="https://pkg.go.dev/badge/github.com/fzn0x/mixmusic.svg" alt="Go 参考文档" /></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/fzn0x/mixmusic" alt="Go 版本" /></a>
  <a href="https://awesomemygo.dev/apps/mixmusic"><img src="https://awesomemygo.dev/badge.svg" height="20" alt="已被 Awesome MyGo 收录" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey" alt="macOS、Windows 和 Linux" />
  <a href="#许可证"><img src="https://img.shields.io/badge/license-MIT%20or%20GPL--3.0-blue" alt="MIT 或 GPL-3.0" /></a>
</p>

<p align="center">
  <a href="https://mixmusic.functionx.dev/">网站</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">下载</a> ·
  <a href="https://mixmusic.functionx.dev/docs/">文档</a> ·
  <a href="docs/features.md">全部功能</a> ·
  <a href="CHANGELOG.md">更新日志</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic 的默认外观（Blend 布局、Mix 主题），竖立在深色舞台上、正对着你：中间是正在播放 YOASOBI《アイドル》的页面，带粉色描边，每行歌词下方显示罗马音，前面是迷你播放器；左边是搜索，右边是音乐库里的混合歌单" />
</p>

## 为什么选择 MixMusic

如果你的音乐分散在 YouTube Music 和 Spotify 两边，通常就得用两个应用，也就有两处搜索、两个播放队列和两个音乐库。MixMusic 把这两个服务放进同一个播放器里：

- **统一搜索**：同时搜索 YouTube Music 和 Spotify，并在每条结果上标明它来自哪个服务。
- **统一的播放队列和音乐库**：Spotify 的歌曲可以紧接着 YouTube Music 的歌曲播放，一个歌单里也可以同时放两边的歌，还可以放你电脑上的音乐文件。
- **无需账号**：打开应用就能播放 YouTube Music。只有想用自己的音乐库时才需要登录。Spotify 是可选的。
- **程序小巧**：MixMusic 是一个 Go 程序，用 GPU 自己绘制窗口。它内部不带浏览器，既不用额外下载，也不会让浏览器常驻内存。
- **随你改造**：自带四种布局和十四个主题，社区还提供更多主题和插件，任何命令都可以设置你自己的快捷键。
- **你的语言**：除英语外，窗口还有 66 种语言可选，阿拉伯语、希伯来语、波斯语和乌尔都语按从右到左排版。
- **注重隐私**：MixMusic 没有自己的账号体系、服务器、统计分析或遥测。
- **免费开源**：可按 MIT 许可证或 GPL 3.0 使用。

## 功能

MixMusic 能做什么，按领域分类，并说明在应用的哪里可以找到。**Built in**（内置）表示它自动生效。[功能](docs/features.md)逐条列出了从 0.1.0 到 0.5.12 的全部 363 项功能，每项一行，并标明它是哪个版本加入的。

### 搜索与播放

- **一次搜索两个服务**：YouTube Music 和 Spotify 的结果并排显示，每条都标明来自哪个服务，你自己的音乐文件也在其中。YouTube Music 的结果会显示每首歌的播放次数，原版和翻唱一眼就能分清。*顶部栏*
- **各种类型的结果**：歌曲、视频、专辑、艺人、歌单和播客，可按最佳匹配、播放最多或最新排序；还可以搜索 YouTube 本身，找到音乐曲库里没有的现场演出和上传内容。把 YouTube 或 Spotify 链接粘贴到搜索框里，会立即播放。*搜索*
- **统一播放队列**：两个服务的歌曲可以混在一起，随机播放和循环播放都跨服务生效，可以把某首歌移到下一首播放；队列播完后，还可以让电台接着播。重启后，队列会恢复成你离开时的样子。*播放栏、队列*
- **每样东西都有自己的页面**：专辑、艺人和歌单都有 Play 和 Shuffle，可以看艺人的全部歌曲，还有与正在播放的歌曲相关的音乐。*搜索、主页*
- **跳过你不想听的**：对歌曲点不喜欢，电台就会跳过它们；可以跳过 AI 制作的歌曲和 SponsorBlock 标记的视频片段，还可以隐藏含露骨内容的歌曲，或优先选择净化版。*播放栏、设置 › 播放*
- **睡眠定时器和播放速度**：可以在一段时间、一首歌或整个队列之后停止播放，渐弱退出，也可以夜里自动开启；歌曲和播客可以加快或放慢播放，音调不变。*播放栏*
- **音乐视频和动态封面**：可以用歌曲的音乐视频取代封面，也可以显示一些专辑循环播放的短视频。*正在播放、设置 › 播放*
- **选择、分享和细看**：选中多首歌一次性加入队列、添加或下载，把歌曲或专辑分享成图片卡片，还能看到一首歌来自哪里、为什么无法播放。*歌曲的菜单*

### 主页与探索

- **主页**：有你的各个栏目、Jump back in（接着听）和你关注的艺人的新发行，排列方式由你决定。*主页*
- **探索**：新发行、排行榜、心情与曲风。*探索*

### 歌词

- **同步歌词**：歌词跟着歌曲走，来源支持时逐字显示，也可以全屏显示；如果你更喜欢，也可以显示纯文本。*歌词面板、正在播放*
- **翻译和拉丁字母**：每行歌词下方可以显示翻译，日文显示罗马音，韩文、中文、俄文、阿拉伯文和许多其他文字显示拉丁字母。*歌词面板*
- **自选来源**：Better Lyrics、Unison、LRCLIB、YouTube Music 和 YouTube 的字幕，按你设定的顺序使用；换一个来源、搜索、修改你自己的版本和调整时间轴，点一下就行。*设置 › 播放 › 歌词、歌词面板*
- **在歌词中查找**：可以把几行歌词分享成卡片，也能读取你自己的歌曲文件里带的歌词。*歌词面板*

### 播客

- **YouTube Music 和 Spotify 的播客**：查找并关注节目，每一集从你上次停下的地方开始，把节目标为已播放，阅读章节和文字稿，还可以给播客设置和歌曲各自独立的速度。*搜索、节目页面、音乐库 › 播客*

### 你的音乐库

- **混合歌单（Mixes）**：能同时收录各个服务歌曲的歌单，顺序由你决定，可以搜索和排序，也能设置你自己的封面。*音乐库 › 混合歌单、侧边栏*
- **喜欢的歌曲**：两个服务中你喜欢的歌汇总在同一个列表里，还有你在 YouTube Music 和 Spotify 上的歌单、专辑和艺人，你可以收藏、创建、分享并一起编辑。点喜欢和收藏，也会同步到对应的服务。*音乐库*
- **置顶、文件夹和标签**：让庞大的音乐库井然有序，不想看到的内容可以隐藏。*侧边栏、方块的菜单*
- **关注艺人**：登录与否都可以，并查看他们的新发行。*艺人页面、音乐库 › 艺人*
- **播放历史、收听统计和年度回顾**：每天听了多少分钟、你最常听的歌曲和艺人，还有可以分享的卡片。*音乐库 › 历史、统计*
- **下载**：可以离线播放；播放过的歌曲也有离线缓存。*歌曲的菜单、音乐库 › 下载*

### 这台电脑上的音乐

- **你自己的音乐文件**：来自 MixMusic 监视的文件夹，出现在音乐库和搜索中，支持 MP3、FLAC、WAV、M4A、Opus 和 Vorbis，带标签和封面。*设置 › 播放 › 这台电脑上的音乐*
- **编辑标签**：可以把同一首歌的多个副本归到一起，也能打开文件或拖进窗口来播放。*歌曲的菜单、拖到窗口上*

### 账号与服务

- **YouTube Music**：在 Google 自己的页面上登录，可以通过你的浏览器，也可以用某个浏览器的会话；可以用你的账号本身登录，也可以用它名下的某个品牌频道，就能使用你的首页推荐、喜欢的歌曲、歌单、专辑和艺人。你播放的歌曲会进入你的 YouTube Music 播放记录，YouTube Music 的推荐就是根据这些记录来的。*设置 › 账号*
- **Last.fm**：你播放的每首歌，听到一半或满四分钟，就会记录到你的个人资料中，离线时产生的 scrobble 会在恢复联网后补发。*设置 › 账号*
- **ListenBrainz**：同样如此，可以和 Last.fm 一起用，也可以单独用，记录到 ListenBrainz 或你自己的服务器，例如 Maloja。喜欢的歌曲也可以作为 love 发送到其中任意一个。*设置 › 账号*
- **Libre.fm**：同样如此，记录到 Libre.fm 或其他 GNU FM 服务器，不用创建 API 账号。*设置 › 账号*
- **Discord**：你的个人资料上会显示正在播放的歌曲，以及它的封面和剩余时间，显示什么由你决定。*设置 › 桌面*

### Spotify 的三种播放方式

- **直接在 MixMusic 里播放**：音质与 Spotify 相同，无需打开 Spotify 应用（需要 Premium，仅限 Windows 和 macOS）。
- **通过 Spotify Connect 播放**：在你的 Spotify 应用、手机或音箱上播放；Spotify 应用开启无损时，就是无损音质。
- **通过 YouTube Music 播放**：MixMusic 播放这首 Spotify 歌曲在 YouTube Music 上的同一录音，不需要 Premium 也能播放。

在 Settings › Playback › Spotify tracks（设置 › 播放 › Spotify 歌曲）中选择。

### 音频

- **十段均衡器**：有 23 个预设，也可以用你自己的；还有由低音、中音和高音组成的简单模式，以及来自 AutoEq、适用于 8800 多款耳机的音色校正。*设置 › 播放 › 均衡器*
- **交叉淡入淡出、无缝专辑、统一响度和音量均衡**。*设置 › 播放 › 声音*
- **混响、回声、交叉馈送、平衡和立体声宽度**。*设置 › 播放*
- **5.1 环绕声**（歌曲有的话），以及你选择的输出设备。*播放栏、设置 › 播放 › 声音*
- **音乐可视化效果**：由 MixMusic 自己的音频引擎绘制。*正在播放*

### 投放、朋友和其他电脑

- **投放**：可以投放到 Chromecast、Google Home 和 Nest 音箱及其音箱组、DLNA 和 UPnP 渲染器以及 FCast 接收器，你自己的音乐文件也可以。*播放栏、迷你播放器*
- **和朋友一起听**：创建一个房间，在朋友的电脑上同步播放，可以在你的网络内，也可以通过邀请链接在任何地方。*播放栏的投放菜单*
- **远程控制**：控制你网络里另一台电脑上的 MixMusic，并在它们之间转移正在播放的内容。*播放栏的投放菜单、设置 › 桌面*
- **AI 助手**：你电脑上的 AI 助手可以通过 Model Context Protocol 了解正在播放的内容、搜索、播放，还能设置睡眠定时器。*设置 › 桌面 › AI 助手*

### 桌面集成

- **迷你播放器**：始终显示在其他窗口之上，面板里有播放队列和歌词。*播放栏、<kbd>Ctrl</kbd> <kbd>M</kbd>*
- **媒体键**和系统的媒体控制，支持 Windows、macOS 和 Linux。*Built in（内置）*
- **全局快捷键**：你自己设置的快捷键，可以播放、暂停、切歌、调节音量、静音、喜欢或不喜欢正在播放的歌曲，或者显示 MixMusic，不论哪个窗口在最前面。*设置 › 快捷键*
- **播放器按钮**：在 Windows 上位于任务栏缩略图中，在 MacBook 上则是围绕刘海的播放器。*Built in（内置）、设置 › 桌面*
- **托盘控制**、切换到新歌曲时的通知、保持在最前面和开机自启。*设置 › 桌面*
- **`mixmusic://` 链接**和 `--next` 这样的参数，可用于 Stream Deck 按键或脚本。*命令行*
- **供叠加层使用的正在播放信息**：一张显示正在播放内容的卡片，可用作 OBS 浏览器源，也提供数据供你自己的页面使用。*设置 › 桌面*

### 个性化

- **布局**：默认的 Blend 采用圆润的造型和富有弹性的动效，在密集排列的大封面上呈现。Expressive 参照 Material 3 Expressive，风格活泼，有色调填充、胶囊按钮和悬浮的播放栏。Crate 比较克制，让封面来说话；Tiles 是 MixMusic 最初的样子，有扁平的方块和巨大的小写文字。*设置 › 外观*
- **主题**：内置十四个主题，每个都能用在任何一种布局中，效果可以调节；浅色模式和深色模式可以各用一个主题；强调色可以自己选，也可以取自正在播放的歌曲封面。Material 和 Material Dark 会让每一种颜色都取自封面。*设置 › 外观*
- **社区主题和插件**：在市场里一键安装。*市场*
- **插件**：通过带类型的 API 添加页面、命令、歌曲操作和小组件，并按你的主题绘制。*设置 › 插件*
- **键盘优先**：常用操作都有快捷键；任何命令（包括插件的命令）都可以设置你自己的快捷键；还有能在任何应用中控制音乐的按键；其余操作可以用命令面板（<kbd>Ctrl</kbd> <kbd>K</kbd>）完成。
- **你的语言**：除英语外还有 66 种语言，需要从右到左阅读的语言按从右到左排版，日期和数字按你所在地区的写法显示。*设置 › 外观 › 语言*
- **为读屏软件和键盘而设计**：为读屏软件提供名称和播报，每个控件都能用键盘操作，还有文字大小、缩放和减少动态效果。*Built in（内置）、设置 › 外观*
- **设置文件**：你的设置保存在数据文件夹中的 `settings.json` 里（Windows 上是 `%AppData%\MixMusic`，macOS 上是 `~/Library/Application Support/MixMusic`，Linux 上是 `~/.config/MixMusic`），可以和你的 dotfiles 放在一起管理。在 MixMusic 运行时保存这个文件，改动会立即生效。*设置 › 桌面 › 设置文件*

### 备份、导入和导出

- **备份和恢复**你的音乐库、设置和附加内容，可以手动，也可以自动，可以在这台电脑上，也可以在另一台电脑上。*设置 › 桌面 › 备份*
- **导入歌单**：可以从文件、你的 Spotify 数据和 Google 数据导入；也可以把列表和你的音乐库导出为其他播放器能读取的文件。*音乐库 › 混合歌单、页面的“更多”菜单*

### 更新

- MixMusic 在启动时以及之后每隔几小时检查一次新版本，验证它的签名，在你确认后安装；你也可以选择让它自动安装。*设置 › 关于*

## 界面一览

<p align="center">
  <img src="docs/look-listen.webp" alt="Blend 布局下的 MixMusic 各个页面，竖立在深色舞台上、正对着你：中间是正在播放 YOASOBI《アイドル》的页面，带粉色描边，每行歌词下方显示罗马音和英文翻译；左边是专辑《THE BOOK 3》，右边是一个已关注播客的页面" />
</p>

<p align="center"><sub><b>听歌。</b>中间是正在播放的 YOASOBI《アイドル》：每行歌词随演唱逐字亮起，下方显示罗马音和翻译。左边是一张专辑，右边是你关注的播客。</sub></p>

<p align="center">
  <img src="docs/look-yours.webp" alt="MixMusic 的更多页面，竖立在深色舞台上、正对着你：中间是 Settings › Appearance（设置 › 外观）和四种布局，带粉色描边；左边是随正在播放的封面变色的 Material 主题，右边是市场里的社区主题" />
</p>

<p align="center"><sub><b>个性化。</b>中间是四种布局：默认的 Blend，以及 Expressive、Crate 和 Tiles，下方是主题。左边是从正在播放的封面取色的 Material 主题。右边是市场里的社区主题。</sub></p>

## 文档

指南放在网站上，地址是 [mixmusic.functionx.dev/docs](https://mixmusic.functionx.dev/docs/)，也可以在 [fzn0x.github.io/mixmusic/docs](https://fzn0x.github.io/mixmusic/docs/) 看到，那里可以搜索任何一篇；这里的 [`docs/`](docs) 中也有。

- **入门**：[入门](docs/getting-started.md)、[功能](docs/features.md)、[账号](docs/accounts.md)、[Spotify](docs/spotify.md)、[疑难解答](docs/troubleshooting.md)
- **使用 MixMusic**：[播放音乐](docs/playing-music.md)、[主页](docs/home.md)、[你的音乐库](docs/library.md)、[导入和导出歌单](docs/import-export.md)、[这台电脑上的音乐](docs/local-music.md)、[歌词](docs/lyrics.md)、[播客](docs/podcasts.md)、[外观与体验](docs/look-and-feel.md)、[在桌面上](docs/desktop.md)、[直播叠加层](docs/stream-overlay.md)、[投放](docs/casting.md)、[远程控制](docs/remote-control.md)、[和朋友一起听](docs/listen-with-friends.md)、[AI 助手](docs/ai-assistants.md)、[市场](docs/marketplace.md)、[设置](docs/settings.md)
- **工作原理**：[MixMusic 如何工作](docs/how-it-works.md)、[音频引擎](docs/audio-engine.md)、[音频流从哪里来](docs/streams.md)、[Spotify 歌曲如何播放](docs/spotify-playback.md)、[你的数据与隐私](docs/data-and-privacy.md)
- **做你自己的**：[主题](docs/themes.md)、[插件](docs/plugins.md)
- **项目**：[致谢](docs/credits.md)、[许可证](docs/license.md)、[架构](docs/architecture/README.md)、[参与贡献](CONTRIBUTING.md)、[行为准则](CODE_OF_CONDUCT.md)、[安全](SECURITY.md)

## 安装

从[最新版本](https://github.com/fzn0x/mixmusic/releases/latest)中下载适合你系统的文件：

| 系统 | 下载 |
|---|---|
| macOS 12 及更高版本（Apple silicon 和 Intel） | [`mixmusic-<version>-macos-universal.dmg`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows 10 和 11（x64、Arm），安装程序 | [`mixmusic-<version>-windows-<arch>-setup.exe`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows 便携版，无需安装 | [`mixmusic-<version>-windows-<arch>-portable.zip`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Debian、Ubuntu 及其衍生发行版 | [`mixmusic-<version>-linux-<arch>.deb`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Fedora、openSUSE 及其衍生发行版 | [`mixmusic-<version>-linux-<arch>.rpm`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Arch Linux 及其衍生发行版（即将推出） | AUR 上的 [`mixmusic-bin`](https://aur.archlinux.org/packages/mixmusic-bin)：`yay -S mixmusic-bin` |
| 任意 Linux，单个文件 | [`mixmusic-<version>-linux-<arch>.AppImage`](https://github.com/fzn0x/mixmusic/releases/latest) |
| 任意 Linux，沙盒运行（Flatpak） | [`mixmusic-<version>-linux-<arch>.flatpak`](https://github.com/fzn0x/mixmusic/releases/latest) |
| 任意 Linux，装在你的主文件夹中 | [`mixmusic-<version>-linux-<arch>.tar.gz`](https://github.com/fzn0x/mixmusic/releases/latest)，附带 `install.sh` |

`<arch>` 在 x64 上是 `amd64`，在 Arm 上是 `arm64`。每个版本都附有 `SHA256SUMS`，可以用来校验下载的文件。名为 `update-…`、`….delta`、`mixmusic-<version>-darwin-universal.tar.gz` 或 `-windows-<arch>.tar.gz` 的文件不需要下载：它们是已安装的应用自动更新时用的。

安装程序、便携版 zip 和 Linux 压缩包会自行更新。`.deb`、`.rpm`、AppImage 和 Flatpak 无法替换自身的文件，所以有新版本时，MixMusic 会告诉你，并替你下载。AUR 的 `mixmusic-bin` 随你系统的其余软件一起更新。便携版 zip 解压到任意位置，打开 `MixMusic.exe` 即可。AppImage 先加上可执行权限（`chmod +x`），再打开。Flatpak 可以用你的软件中心打开，或者运行 `flatpak install ./mixmusic-<version>-linux-<arch>.flatpak`；第一次安装时，Flatpak 会从 Flathub 获取 GNOME 运行时。

在 Mac 上，[Homebrew](https://brew.sh) 也能安装 MixMusic，用 `brew upgrade` 更新它：

```sh
brew install --cask fzn0x/tap/mixmusic
```

[入门](docs/getting-started.md)会带你走完每个系统上的安装和首次启动。

### 首次启动

- **macOS**：这个应用还没有经过 Apple 公证，所以第一次打开时，macOS 会提示无法检查它。你仍然可以打开它：前往 System Settings › Privacy & Security › Open Anyway（系统设置 › 隐私与安全性 › 仍要打开），或者在 Terminal（终端）中运行一次：

  ```sh
  xattr -dr com.apple.quarantine /Applications/MixMusic.app
  ```
- **Windows**：安装程序还没有签名，所以 SmartScreen 可能会发出警告：先选择 More info（更多信息），再选择 Run anyway（仍要运行）。在 MixMusic 中登录 YouTube Music 和播放 Spotify 需要 WebView2 Runtime，Windows 11 已自带。
- **Linux**：MixMusic 需要 GTK 3，以及 PulseAudio（PipeWire 也能提供 PulseAudio 服务）或 ALSA。登录 YouTube Music 需要 WebKitGTK 4.1，托盘图标需要 `libayatana-appindicator3`，把登录信息保存在系统密钥环中需要 `libsecret`。`.deb` 和 `.rpm` 会安装这些依赖，Flatpak 自带它们；如果使用 AppImage 或压缩包，请运行：

  ```sh
  # Debian and Ubuntu
  sudo apt install libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
  # Fedora
  sudo dnf install gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
  ```

  MixMusic 不自带中文、日文和韩文字体，在 Linux 上，已安装的字体里都没有的字会显示为空白。`.deb` 和 `.rpm` 会要求安装一款这样的字体；用其他方式安装的话，请自己装一款，然后重启 MixMusic：

  ```sh
  # Debian and Ubuntu
  sudo apt install fonts-noto-cjk
  # Fedora
  sudo dnf install google-noto-sans-cjk-vf-fonts
  # Arch
  sudo pacman -S noto-fonts-cjk
  ```
- **所有系统**：大多数 YouTube Music 歌曲的音频，MixMusic 都能自己找到。其余的歌曲（比如面向儿童的内容），它会使用 [yt-dlp](https://github.com/yt-dlp/yt-dlp)：你可以在 Settings › Playback（设置 › 播放）中让 MixMusic 替你安装 yt-dlp，之后它也会让 yt-dlp 保持最新。yt-dlp 需要一个 JavaScript 运行时：[Deno](https://deno.com)、Node.js、Bun 或 QuickJS。Flatpak 版自带 Node.js。

### Flatpak 版

Flatpak 版在沙盒中运行 MixMusic。沙盒能访问网络、声音服务器、显示和 GPU，看不到你的主文件夹，只有从 Flathub 安装的浏览器的配置文件除外，它们是只读的，可以从中导入 YouTube Music 的登录信息。它提供 x64 和 Arm64 两种版本，Arm64 适用于 Raspberry Pi 5、Asahi Linux 和 Snapdragon 笔记本等。MixMusic 暂时不在 Flathub 上：请安装发布页里的 `.flatpak`。

Flatpak 版把设置和音乐库保存在 `~/.var/app/io.github.fzn0x.mixmusic` 中，与 `.deb`、`.rpm`、AppImage 或压缩包安装的那一份分开，两者可以同时运行。如果要把另一份的设置和音乐库搬过来，先在 Settings › Desktop › Backups（设置 › 桌面 › 备份）中备份，再在 Flatpak 版中恢复这个文件。如果想手动复制文件，先退出两者，然后把 `settings.json`、`library.json`、`plays.jsonl`、`local-copies.json`、`queue.json`、`positions.json` 以及 `addons` 和 `mix-covers` 文件夹从 `~/.config/MixMusic` 复制到 `~/.var/app/io.github.fzn0x.mixmusic/config/MixMusic`。一定要先退出两者：运行中的 MixMusic 会把 `library.json` 和其他大多数文件保存在内存里，下次保存时会覆盖复制过来的文件。然后重新登录你的账号：登录信息留在另一份所用的密钥环中，不会跟着过来。[数据与隐私页面](docs/data-and-privacy.md#the-data-folder)说明了每个文件里存的是什么。

在 Flatpak 版中，开机自启要经过你的桌面环境，它可能会先征求你的同意；如果要用你自己的 yt-dlp，它必须放在 `~/.var/app/io.github.fzn0x.mixmusic` 中，因为沙盒看不到其他文件夹。不是 Flatpak 版的 Discord 应用，只有在 MixMusic 启动时已经在运行，才会显示正在播放的内容，而且 Discord 重启后就不再显示；Flatpak 版的 Discord 则不受此限制。Flatpak 版刚推出不久：欢迎在 [issue](https://github.com/fzn0x/mixmusic/issues) 中告诉我们它在你的桌面上运行得怎么样。

### 账号（均为可选）

- **YouTube Music**：在 Settings › Accounts（设置 › 账号）中选择 Sign in to YouTube Music（登录 YouTube Music），然后在弹出的窗口中登录 Google。你输入的内容只会发送给 Google。MixMusic 会加密保存登录会话；系统有密钥环时，密钥就存放在密钥环中。如果那个窗口打不开，或者 Google 拒绝它，可以用你自己的浏览器登录，MixMusic 会用一个它自己的新配置文件打开浏览器，你登录完成后再把这个配置文件删除；也可以导入你已登录的某个浏览器的会话（MixMusic 用它的 yt-dlp 读取，并且只保留 YouTube 的 cookie）；或者粘贴已登录的 music.youtube.com 标签页的 Cookie 请求头。
- **Spotify**：Spotify 规定一个应用最多只能服务五个用户，而且只有在应用的所有者订阅了 Premium 时才行，所以你需要用自己创建的 Spotify 应用登录。只需两分钟：[docs/spotify.md](docs/spotify.md) 会一步步带你完成。
- **Last.fm**：在 [last.fm/api/account/create](https://www.last.fm/api/account/create) 创建一个 API 账号（名称随意，不用填回调 URL），把它的 API key 和 shared secret 粘贴到 Settings › Accounts 中，然后在浏览器中授权 MixMusic。
- **ListenBrainz**：把你的 [ListenBrainz 设置](https://listenbrainz.org/settings/)中的 user token，或你自己服务器上的 user token，粘贴到 Settings › Accounts 中。
- **Libre.fm**：在 Settings › Accounts 中连接，然后在浏览器中，于 Libre.fm 的页面（或其他 GNU FM 服务器的页面）上批准 MixMusic。
- **Discord**：Discord 应用运行时，你的个人资料上会显示你正在听的内容。可以在 Settings › Desktop（设置 › 桌面）中关闭。

[账号](docs/accounts.md)逐步介绍了每种登录方式。

### 已知限制

- **Arm 版 Windows** 的测试不如 x64 充分，便携版 zip 也不会注册 `mixmusic://` 链接。
- **处于开发模式的 Spotify 应用**（你用来登录的那个就是）：每种类型只返回十条搜索结果，你不拥有的歌单里的歌曲仍然是隐藏的。
- **在 Linux 上**，Spotify 默认通过 Spotify Connect 播放，因为 WebKitGTK 无法播放 Spotify 的受保护音频。
- **YouTube** 可能会更改它的接口，导致搜索、播放或登录出问题，直到 MixMusic 或 yt-dlp 跟上为止。

遇到问题？[疑难解答](docs/troubleshooting.md)列出了常见问题和已知问题。

## 从源码构建

你需要 [Go](https://go.dev/dl/) 1.27.3 或更高版本、[Bun](https://bun.sh) 1.4 或更高版本（用来运行 MyGo 的命令行工具），以及 Git。MyGo 不需要 C 编译器，所以在任何系统上都能为所有系统构建 MixMusic。

### Windows

```powershell
winget install GoLang.Go
winget install Oven-sh.Bun
winget install Git.Git
```

打开一个新的终端，然后运行：

```powershell
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

登录和 Spotify 播放需要 WebView2 Runtime，Windows 11 已自带。Windows 10 上请从 [Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/) 安装。

### macOS

使用 [Homebrew](https://brew.sh)：

```sh
brew install go oven-sh/bun/bun
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Linux

从 [go.dev/dl](https://go.dev/dl/) 安装 Go（发行版的软件包往往低于 1.27.3），然后安装 Bun 和 MixMusic 运行所需的库：

```sh
curl -fsSL https://bun.sh/install | bash
# Debian and Ubuntu
sudo apt install git libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
# Fedora
sudo dnf install git gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
```

然后：

```sh
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Nix

如果你用 [Nix](https://nixos.org)，`nix develop` 会提供构建正式版本所用的工具，版本由 `flake.lock` 锁定：Go 1.27、Bun、GoReleaser、jq、zip 和 Zola，在 Linux 上还有 NSIS、osslsigncode 和 binutils。`nix build` 会从源码构建 Linux 版 MixMusic，输出到 `result/`，`nix run github:fzn0x/mixmusic` 则直接运行它。

### 开发

`bun run dev` 会构建并启动应用，并在你修改代码时重新构建。它的数据以 `MixMusic Dev` 的名称单独存放，与已安装的 MixMusic 互不干扰。

```sh
bun run build:plugins          # the plugins' JavaScript, from their TypeScript
go test ./...                  # the app's tests
bun run typecheck              # the plugin SDK and the community's plugins
go run ./tools/registry -check # the community registry
bun run build                  # the packaged app for this system, in build/
bun run build -- -platform windows/amd64,linux/amd64,linux/arm64
```

在 Nix shell 中，请像正式发布那样，用 `.github/scripts/build.sh linux/amd64,linux/arm64` 构建 Linux 下载包。在那里直接运行 `bun run build`，得到的 Linux 程序会要求使用 Nix 自己的加载器，只能在 NixOS 上启动。`go run ./tools/elfloader build/linux-amd64` 可以检查构建出来的程序。

[网站](https://mixmusic.functionx.dev/)的源码在 `site/` 中，用 [Zola](https://www.getzola.org) 构建，`docs/` 中的指南就是网站的 `/docs/` 页面。运行 `go run ./tools/sitedocs && zola --root site serve` 可以预览网站。正式版本在 Nix shell 中构建，由 [GoReleaser](https://goreleaser.com) 发布（`.goreleaser.yaml`）。

[CONTRIBUTING.md](CONTRIBUTING.md) 列出了提交 pull request 之前要运行的检查，[docs/architecture](docs/architecture/README.md) 则用一页介绍一个部分，说明各个部分是如何配合工作的。

### 目录结构

应用本身是根包，其下是 `internal/` 中的各个包。`internal/` 之外的包任何人都可以导入，文档见 [pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic)。

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

[docs/architecture](docs/architecture/README.md) 介绍了 `internal/` 中的其他包，每个部分一页。

可以导入的包：

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

## 社区

任何人都可以从市场（侧边栏中的商店图标）安装社区主题和插件，你也可以添加自己的作品。

### 制作主题

主题就是一个 `theme.json` 文件，用来设置颜色、字体和圆角，它们会显示在用户选择的布局中。[docs/themes.md](docs/themes.md) 列出了所有 token，并给出了一个完整的主题。想试试你的主题，就把它的文件夹放进主题文件夹（用 Settings › Appearance › open themes folder 打开，即“设置 › 外观 › 打开主题文件夹”），然后在 Settings › Appearance 中选择它。

### 制作插件

插件是用 TypeScript 或 JavaScript 编写的小模块，可以添加页面、命令、歌曲操作和小组件。[docs/plugins.md](docs/plugins.md) 是 API 的文档，还附有供编辑器使用的[类型定义](sdk/index.d.ts)。社区里的 [sleep timer](community/plugins/sleep-timer) 和 [spectrum](community/plugins/spectrum) 都是能直接运行的示例。

### 分享作品

1. Fork 本仓库。
2. 添加一个文件夹：主题放在 `community/themes/<id>/`，并附上它的 `theme.json`；插件放在 `community/plugins/<id>/`，并附上它的 `plugin.json` 和 `index.ts`。
3. 运行 `bun install`、`bun run build:plugins`、`go run ./tools/registry` 和 `go run ./tools/registry -check`。
4. 提交一个 pull request，并附上截图。

合并之后，CI 会把它发布到 [registry 分支](https://github.com/fzn0x/mixmusic/tree/registry)，它就会出现在所有人的市场中。详情见 [community/README.md](community/README.md)。

## 隐私

MixMusic 没有自己的账号体系、服务器、统计分析或遥测。它会与以下服务通信：

- **YouTube 和 Google**：YouTube Music 的曲库、音频流和每首歌的响度（这些都在未登录状态下请求），以及封面图片；部分音频流通过 yt-dlp 获取。登录后，还有你在那里的音乐库；另外，除非你在 Settings › Accounts（设置 › 账号）中关闭，你播放的歌曲也会发送过去，进入你的 YouTube Music 播放记录。YouTube 本身（未登录，且不带 cookie）：用于搜索时选择 YouTube、检查是否为 AI 制作的歌曲、把字幕用作歌词和文字稿，以及与音乐视频对时的歌词。
- **Spotify**（登录之后）：你的音乐库、你的 Spotify Connect 设备；在 MixMusic 内播放 Spotify 时，还有从 `sdk.scdn.co` 加载的 Spotify 播放器。
- **Better Lyrics**：正在播放的歌曲的标题、第一位艺人、专辑和时长，用来查找歌词；仅当歌曲有专辑和时长时才会发送。
- **Unison**：正在播放的歌曲的视频 ID，没有视频 ID 时则是它的标题、艺人、专辑和时长，用来查找歌词。只有在你于歌词旁打开翻译，或为阿拉伯语打开罗马化时，才会发送正在播放的歌曲的歌词和它的视频 ID，由 Unison 交给 Google 翻译或罗马化。
- **LRCLIB**：正在播放的歌曲的标题、艺人、专辑和时长，用来查找歌词。
- **SponsorBlock**：只有在你打开 Skip the parts SponsorBlock marks（跳过 SponsorBlock 标记的片段）时，才会发送每个视频 ID 的哈希值的前 4 个字符，从不发送 ID 本身，用来查找它标记的片段。
- **GitHub**：启动时以及之后每隔几小时检查一次新版本，可以在 Settings › About（设置 › 关于）中关闭；打开市场时获取社区注册表；下载 yt-dlp；AutoEq 的耳机列表和你选的那一份配置；以及第一次在日文歌词旁打开罗马音时，下载汉字读音词典（11.7 MB）。罗马音，以及阿拉伯文以外其他文字的拉丁字母转写，都在你的电脑上生成：歌词不会被发送出去。
- **Last.fm**（连接之后）：你播放的内容。
- **ListenBrainz** 或你自己的兼容其 API 的服务器（连接之后）：你播放的内容。
- **Libre.fm** 或其他 GNU FM 服务器（连接之后）：你播放的内容。
- **Discord**（除非你关闭）：正在播放的内容，发送给你电脑上的 Discord 应用，由它显示在你的个人资料上。
- **你投放到的设备**（在你的网络内）：正在播放的歌曲的标题、艺人和封面。它们从 MixMusic 获取音频。
- **其他装有 MixMusic 的电脑**：用于远程控制和和朋友一起听，经过加密，在你的网络内进行；如果你允许网络之外的朋友加入，则通过互联网进行。此时 MixMusic 会请求你的路由器转发一个端口，你的邀请链接里会带有你的互联网地址。
- **你电脑上的 AI 助手**（在你允许的期间）：正在播放的内容、播放队列、它们的搜索结果和你的歌单名称，助手可能会把这些转交给它所用的 AI 模型。
- **YouTube 和 Spotify**：用来了解网络是否有响应，即通过 HTTPS 发一个请求，得到一个空响应。

插件可以发送任何它想发送的内容：插件的权限只是展示给你看，并不会强制执行，所以 MixMusic 会在启用插件之前请你确认信任它。[你的数据与隐私](docs/data-and-privacy.md#what-each-feature-sends-where)里有一张表，列出了每项功能把什么发送到哪里。

它在你电脑上保存的内容：

- 你的设置和音乐库（喜欢和不喜欢的歌曲、混合歌单、统计所计入的每一次播放、关注的艺人和节目、置顶、文件夹和标签、播放历史、最近的搜索，以及你的音乐文件的索引），保存在 Settings › About 中显示的文件夹里。
- 你播放过的歌曲，存放在缓存中，大小以你在 Settings › Playback（设置 › 播放）中允许的为限；以及 YouTube 测得的这些歌曲的响度。你下载的歌曲，存放在下载文件夹中。
- 你看过的封面，最多 256 MB。
- 你打开过的歌词翻译（按歌曲和语言保存），以及罗马音用来读汉字的词典（打开罗马音之后才有）。
- 你的登录信息（包括 YouTube Music 会话），保存在系统密钥环中。在 Windows 上，如果凭据管理器（Credential Manager）存不下某条登录信息，它会存进一个针对你的 Windows 账户加密的文件；在没有密钥环的 Linux 上，则存进只有你的用户能读取的文件。
- 日志，其中不含你的登录信息。

在音乐库的播放历史页面可以清除播放记录，在 Settings › Playback 中可以清除缓存和封面，退出某个服务的登录会删除它的登录信息。[清除、迁移或重置](docs/data-and-privacy.md#clear-move-or-reset)介绍了如何迁移你的数据、用 `MIXMUSIC_DATA_DIR` 保留一份便携副本，或者从头开始。

在 Windows 上，可视化效果通过 Windows 的进程环回（process loopback）获取 Spotify 在这台电脑上播放的音频，无论是在 MixMusic 中还是在 Spotify 应用中播放；进程环回只采集那一个应用的声音。在 macOS 14.2 及更高版本上，可视化效果通过进程采集（process tap）获取在 MixMusic 中播放的 Spotify 的音频，需要你按 macOS 的提示允许。音频只在内存中分析，从不录制或发送。

## 致谢

MixMusic 基于以下项目构建：

- **[MyGo](https://github.com/egoist/mygo)**：MixMusic 所用的桌面框架。MixMusic 使用 MyGo 的原生 UI：从各个页面、播放栏到迷你播放器，每个窗口都由 MyGo 用 Go 自己绘制，不使用 webview。它的命令行工具负责打包应用，包括安装程序，以及应用自我更新所用的签名更新包。
- **[Go](https://go.dev)**：应用使用的语言；以及 [purego](https://github.com/ebitengine/purego)（无需 cgo 即可调用系统库）、[golang.org/x/image](https://pkg.go.dev/golang.org/x/image) 和 [golang.org/x/text](https://pkg.go.dev/golang.org/x/text)。
- **[Sobek](https://github.com/grafana/sobek)**：每个插件运行所在的 JavaScript 引擎。
- **[Nix](https://nixos.org)**：锁定构建正式版本所用的工具，并从源码构建 Linux 版 MixMusic。
- **[GoReleaser](https://goreleaser.com)**：发布正式版本；以及它的 [nFPM](https://nfpm.goreleaser.com)，用来制作 RPM 包。
- **[Bun](https://bun.sh)**：运行构建流程。**[TypeScript](https://www.typescriptlang.org)**：编译插件，并检查插件 SDK 的类型。
- **[Zola](https://www.getzola.org)**：构建网站。
- **[NSIS](https://nsis.sourceforge.io)**：制作 Windows 安装程序；[appimagetool](https://github.com/AppImage/appimagetool) 则用来制作 AppImage。
- **[yt-dlp](https://github.com/yt-dlp/yt-dlp)**：为 MixMusic 自己找不到音频的歌曲找到音频。
- **[SponsorBlock](https://sponsor.ajay.app)**：由 Ajay Ramachandran 及其贡献者建立的开放的、由众包建立的 YouTube 视频片段数据库，MixMusic 在你要求时会跳过其中标记的片段。Uses SponsorBlock data licensed under [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/) from https://sponsor.ajay.app/.
- **[Better Lyrics](https://betterlyrics.org)**：提供逐字同步歌词的服务。
- **[Unison](https://unison.betterlyrics.org)**：开放的、由众包建立的歌词数据库，采用 Unison Community License 许可；**[LRCLIB](https://lrclib.net)**：开放的同步歌词数据库。
- **[kagome](https://github.com/ikawaha/kagome)**：日语形态分析器，读出歌词中的汉字以生成罗马音；搭配 [kagome-dict](https://github.com/ikawaha/kagome-dict) 的 IPA 词典，它由 mecab-ipadic 制成，MixMusic 在第一次打开罗马音时下载。
- **[libopus](https://opus-codec.org)**：MixMusic 的 Opus 解码器移植自它的解码器。**[SharpJaad](https://github.com/jimm98y/SharpJaad)**：[JAAD](https://sourceforge.net/projects/jaadec/) 的 C# 移植版，MixMusic 的 AAC 解码器就源自它。
- **[Chromium](https://www.chromium.org)** 的 Web Audio：均衡器的滤波器、音量均衡和可视化效果，分别移植自它的 biquad、DynamicsCompressor 和 AnalyserNode。
- **[AutoEq](https://github.com/jaakkopasanen/AutoEq)**：由 Jaakko Pasanen 及其贡献者建立，它的配置文件用来校正耳机的音色，采用 [Equalizer APO](https://sourceforge.net/projects/equalizerapo/) 的格式。
- **[go-mp3](https://github.com/hajimehoshi/go-mp3)** 和 **[jfreymuth/vorbis](https://github.com/jfreymuth/vorbis)**：解码你电脑上的 MP3 和 Vorbis 文件；**[Xiph.Org](https://xiph.org)**：`flac` 和 `ogg` 所读取的 FLAC 和 Ogg 格式出自它。
- **[FFmpeg](https://ffmpeg.org)**：解码器的测试以它作为对照。
- **[Selawik](https://github.com/microsoft/Selawik)**、**[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily)**、**[Roboto Flex](https://github.com/googlefonts/roboto-flex)** 和 **[Archivo](https://github.com/Omnibus-Type/Archivo)**：窗口中使用的字体。
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities)**：Material 主题的颜色移植自它的 HCT 色彩科学。
- **[FCast](https://docs.fcast.org)**：MixMusic 用来在 FCast 接收器上播放的开放协议。MixMusic 是它的独立实现，与 FUTO 没有关联；FCast 是 FUTO 的注册商标。
- **[Kevin MacLeod](https://incompetech.com)**：网站中播放的是他的音乐，采用 [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) 许可。

[致谢](docs/credits.md)列出了 MixMusic 所基于的每个项目、论文和标准。

## 参与贡献

欢迎提交修复、新功能、主题、插件和 bug 报告。

- **报告问题**：在 Settings › About › report a problem（设置 › 关于 › 报告问题）中报告。它会新建一个 GitHub issue，并自动填好你的版本、系统、主题和插件。你也可以自己[新建 issue](https://github.com/fzn0x/mixmusic/issues/new)。
- **提交改动**：以 pull request 的形式提交。[CONTRIBUTING.md](CONTRIBUTING.md) 说明了需要运行的检查、提交信息的风格，以及用户能感知到的改动需要附带的更新日志条目。
- **分享主题或插件**：见[社区](#社区)。
- **私下报告安全漏洞**：按 [SECURITY.md](SECURITY.md) 的说明进行。

所有参与者都要遵守[行为准则](CODE_OF_CONDUCT.md)。[CONTRIBUTORS.md](CONTRIBUTORS.md) 列出了以代码、想法和问题报告帮助塑造 MixMusic 的人，感谢他们。

## 许可证

MixMusic 采用双重许可：你可以任选 [MIT 许可证](LICENSE-MIT)或 [GNU General Public License, version 3](LICENSE-GPL) 来使用它。贡献同时以这两种许可证接受。`fonts` 中的字体采用 SIL Open Font License：[Selawik 的许可证](fonts/OFL.txt)、[Quicksand 的许可证](fonts/quicksand-OFL.txt)、[Roboto Flex 的许可证](fonts/robotoflex-OFL.txt)和 [Archivo 的许可证](fonts/archivo-OFL.txt)。Opus 解码器移植自 libopus，采用[它的 BSD 许可证](opus/LICENSE.libopus)；AAC 解码器源自 SharpJaad，采用[它的 MIT 许可证](aac/LICENSE.sharpjaad)；HCT 色彩科学移植自 Material Color Utilities，采用[它的 Apache 许可证](internal/hct/LICENSE.material-color-utilities)。其他部分沿用它们所源自项目的许可证，例如 Chromium 的 Web Audio 的滤波器、go-mp3、jfreymuth/vorbis、kagome 和 Unihan 数据：[许可证](docs/license.md)逐一列出了它们。

MixMusic 与 Spotify 和 Google 没有任何关联，也未获得它们的认可或赞助。它通过这些服务自己的接口，播放它们向用户提供的内容；请在它们的条款允许的范围内使用。Spotify 和 YouTube Music 是其各自所有者的商标。
