# License
<!-- docs: group=project order=30 -->

MixMusic is free and open source, under the MIT license or the GNU GPL version 3, whichever you choose. Here is what that means, and what else it includes.

MixMusic's own code is dual-licensed: use it under the
[MIT license](../LICENSE-MIT) or under the
[GNU General Public License, version 3](../LICENSE-GPL), whichever you
prefer. Settings › About says so too. Some parts it includes, such as the
typefaces and the code it ports, keep licences of their own, listed below.

> [!NOTE]
> This page explains the licences in plain words. It is not legal advice:
> the licence texts decide.

## What the two licences mean

- **Using MixMusic** is free under either licence, for any purpose. You
  need to accept neither to run it.
- **The MIT license** lets you use, copy, change, merge, publish, share,
  sublicense and sell MixMusic's code, in open or closed programs, as long
  as copies keep its copyright notice and the licence's text. It comes
  without any warranty.
- **The GPL, version 3**, lets you do the same, as long as what you share,
  changed or not, comes with its source code under the GPL too, with its
  notices, so that the people you give it to have the same freedoms.
  It comes without any warranty too.
- **Choosing.** You pick the licence that suits what you do, and follow
  that one. The MIT license asks the least, as when you use one of
  MixMusic's decoders in a program of your own. The GPL fits a program
  that is under the GPL itself.
- **Contributions** are accepted under both licences, so that the whole of
  MixMusic stays under both. That includes the community's themes and
  plugins: opening a pull request for one agrees to it.

The Go packages anyone can import, `opus`, `aac`, `ac3`, `webm`, `mp4`,
`flac`, `ogg`, `resample`, `lyrics` and `brand`, come under the same
choice, with the licences of the code they derive from, below.

## Parts with licences of their own

| Part | What it is | Licence | Its text |
|---|---|---|---|
| The Opus decoder, `opus` | a port of libopus 1.6.1's decoder, by Xiph.Org and others | libopus's BSD license | [opus/LICENSE.libopus](../opus/LICENSE.libopus) |
| The AAC decoder, `aac` | derived from SharpJaad, the C# port of JAAD, whose authors placed it in the public domain | MIT | [aac/LICENSE.sharpjaad](../aac/LICENSE.sharpjaad) |
| The equalizer's filters, the compressor and the analyser, in `internal/audio` | ports of Blink's biquad, DynamicsCompressor and AnalyserNode, of Chromium | Blink's BSD licenses | [internal/audio/LICENSE.blink](../internal/audio/LICENSE.blink) |
| The colours of the Material themes, `internal/hct` | a port of Material Color Utilities, by Google | Apache 2.0 | [internal/hct/LICENSE.material-color-utilities](../internal/hct/LICENSE.material-color-utilities) |
| The readings of Chinese, built in | made from the Unihan database of Unicode, and from mozillazg's pinyin-data and phrase-pinyin-data | the Unicode License v3, and MIT | [LICENSE.unicode](../internal/romanize/LICENSE.unicode), [LICENSE.pinyin-data](../internal/romanize/LICENSE.pinyin-data), [LICENSE.phrase-pinyin-data](../internal/romanize/LICENSE.phrase-pinyin-data) |
| kagome, which reads kanji for romaji | a Go module, by ikawaha | MIT | [internal/romaji/LICENSE.kagome](../internal/romaji/LICENSE.kagome) |
| go-mp3, which decodes MP3 files | a Go module, by Hajime Hoshi and its authors | Apache 2.0 | [internal/audio/LICENSE.go-mp3](../internal/audio/LICENSE.go-mp3) |
| jfreymuth/vorbis, which decodes Vorbis files | a Go module, by Johann Freymuth | MIT | [internal/audio/LICENSE.vorbis](../internal/audio/LICENSE.vorbis) |
| The IPA dictionary, downloaded the first time you turn romaji on | kagome-dict's, made from mecab-ipadic 2.7.0-20070801 | MIT for kagome-dict, and the notice of mecab-ipadic | [LICENSE.kagome-dict](../internal/romaji/LICENSE.kagome-dict), [NOTICE.ipadic](../internal/romaji/NOTICE.ipadic) |
| Selawik, the interface's face | by Microsoft | SIL Open Font License 1.1 | [fonts/OFL.txt](../fonts/OFL.txt) |
| Quicksand, the Mochi theme's face | by Andrew Paglinawan | SIL Open Font License 1.1 | [fonts/quicksand-OFL.txt](../fonts/quicksand-OFL.txt) |
| Roboto Flex, the Material themes' face | by Google Fonts | SIL Open Font License 1.1 | [fonts/robotoflex-OFL.txt](../fonts/robotoflex-OFL.txt) |
| Archivo, the Walnut themes' headings | by Omnibus-Type | SIL Open Font License 1.1 | [fonts/archivo-OFL.txt](../fonts/archivo-OFL.txt) |

The Go modules MixMusic is built with keep their own licences: MyGo and
Sobek under MIT, purego under Apache 2.0, and golang.org/x/image and
golang.org/x/text under Go's BSD license. [go.mod](../go.mod) lists them
all.

Copies of MixMusic must keep these notices, as the licences ask:
Settings › About › Licenses shows each of them in the app.

## What MixMusic doesn't include

- **Lyrics** come from Unison, LRCLIB and YouTube Music as you play, and
  aren't part of MixMusic. Unison's are under the
  [Open Database License](https://opendatacommons.org/licenses/odbl/),
  which asks apps that show them to credit them. Unison asks for the
  person who submitted them too, and the lyrics panel credits both: "lyrics
  from Unison · submitted by <name>", linked to their pages on Unison.
- **yt-dlp and the JavaScript runtimes** are programs of their own, which
  MixMusic runs, under their own licences. MixMusic downloads yt-dlp from
  its releases when you install it.
- **The music on this website** is by Kevin MacLeod
  ([incompetech.com](https://incompetech.com)), under
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), and is not
  part of the app. [Credits](credits.md) names each song.

## Trademarks

MixMusic is not affiliated with, endorsed by or sponsored by Spotify or
Google. It plays what these services offer their users, through the
services' own interfaces; use it as their terms allow. Spotify and YouTube
Music are trademarks of their owners.
