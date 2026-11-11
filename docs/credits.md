# Credits
<!-- docs: group=project order=20 -->

MixMusic is built on open-source projects, open data and published standards. This page credits each, with what MixMusic uses it for.

The licences of the parts MixMusic includes are on [the license page](license.md),
and Settings › About › Licenses shows their texts in the app.

## The people

MixMusic is made by [fzn0x](https://github.com/fzn0x) with the people whose
code, ideas and reports shaped it. [CONTRIBUTORS.md](../CONTRIBUTORS.md)
thanks each of them by name, with the issues and pull requests they
helped with. Thank you all. Helped too? Add yourself, or someone you'd
like to thank, in a row there, in the pull request of your change or one
of its own.

## The framework and the language

- **[MyGo](https://github.com/egoist/mygo)**, the desktop framework
  MixMusic is built with. MyGo draws every window itself, in Go, without a
  web view, from the pages and the player bar to the mini player. It gives
  MixMusic its windows, menus, tray, web views, file watching and
  credential store, and its command line packages the app, with its
  installers and the signed updates it updates itself from.
- **[Go](https://go.dev)**, the language of the app, with
  [purego](https://github.com/ebitengine/purego), which calls the
  system's libraries without cgo, for the audio output and the media
  controls; [golang.org/x/image](https://pkg.go.dev/golang.org/x/image),
  which reads WebP covers and scales images; and
  [golang.org/x/text](https://pkg.go.dev/golang.org/x/text), whose Unicode
  normalization matches titles, searches lists regardless of accents and
  helps write lyrics in Latin letters.
- **[Sobek](https://github.com/grafana/sobek)**, Grafana's JavaScript
  engine in Go, a fork of goja with ES modules, which each plugin runs in.
  It builds on [goja](https://github.com/dop251/goja) by Dmitry Panov and
  [otto](https://github.com/robertkrimen/otto) by Robert Krimen.
- The modules these depend on, listed in [go.mod](../go.mod), among them
  [go-text/typesetting](https://github.com/go-text/typesetting), which
  shapes MyGo's text, and [regexp2](https://github.com/dlclark/regexp2),
  which Sobek's regular expressions use.

## The audio

- **[libopus](https://opus-codec.org)**, by Xiph.Org and the other authors
  of the Opus codec: MixMusic's Opus decoder is a port of libopus 1.6.1's.
- **[SharpJaad](https://github.com/jimm98y/SharpJaad)**, the C# port of
  [JAAD](https://sourceforge.net/projects/jaadec/), which the AAC decoder
  derives from.
- **[Chromium](https://www.chromium.org)**'s Web Audio, in Blink: the
  equalizer's filters, the compressor of volume levelling and the
  visualizer's analyser are ports of its biquad, DynamicsCompressor and
  AnalyserNode, so MixMusic sounds as its web player did.
- **[go-mp3](https://github.com/hajimehoshi/go-mp3)**, by Hajime Hoshi
  and its authors, a port of PDMP3, which decodes the MP3 files on your
  computer, and **[jfreymuth/vorbis](https://github.com/jfreymuth/vorbis)**,
  by Johann Freymuth, which decodes their Vorbis.
- **[Xiph.Org](https://xiph.org)**, whose FLAC (RFC 9639) and Ogg
  (RFC 3533) MixMusic's `flac` and `ogg` read, as their specifications
  have them.
- **[FFmpeg](https://ffmpeg.org)**, which the tests of the decoders and of
  the resampler compare with.
- **W. Verhelst and M. Roelands**, whose WSOLA, the waveform-similarity
  overlap-add of their paper at ICASSP 1993, plays tracks faster or
  slower at their own pitch.

## The streams

- **[yt-dlp](https://github.com/yt-dlp/yt-dlp)**, which finds the audio of
  the tracks MixMusic can't find itself, and the 5.1 surround mixes, and
  reads a browser's YouTube sign-in.
- **[Deno](https://deno.com)**, **[Node.js](https://nodejs.org)**,
  **[Bun](https://bun.sh)** and **[QuickJS](https://bellard.org/quickjs/)**,
  the JavaScript runtimes yt-dlp solves YouTube's challenges with.

## Lyrics and their Latin letters

- **[Better Lyrics](https://betterlyrics.org)**, whose service keeps
  lyrics timed word by word, which MixMusic asks first. The lyrics panel
  credits it under each of its lyrics: "lyrics from Better Lyrics
  (https://betterlyrics.org)".
- **[Unison](https://unison.boidu.dev)**, the open, crowdsourced database
  of lyrics, most of them timed word by word, under the
  [ODbL](https://opendatacommons.org/licenses/odbl/). As its licence asks,
  the lyrics panel credits it under each of its lyrics: "lyrics from
  Unison (https://unison.boidu.dev)". Unison also translates lyrics, and
  romanizes Arabic, while you turn those on.
- **[LRCLIB](https://lrclib.net)**, the open database of synced lyrics,
  which MixMusic asks when Unison has none.
- **[kagome](https://github.com/ikawaha/kagome)**, by ikawaha, the
  Japanese morphological analyser that reads the kanji of lyrics for
  their romaji, with the IPA dictionary of
  [kagome-dict](https://github.com/ikawaha/kagome-dict), made from
  mecab-ipadic 2.7.0-20070801 of the Nara Institute of Science and
  Technology, much of it from ICOT's free software. MixMusic downloads the
  dictionary the first time you turn romaji on.
- **The [Unihan database](https://www.unicode.org/charts/unihan.html)** of
  the Unicode Consortium, and mozillazg's
  [pinyin-data](https://github.com/mozillazg/pinyin-data) and
  [phrase-pinyin-data](https://github.com/mozillazg/phrase-pinyin-data),
  from which MixMusic makes the readings of Chinese it builds in.
- **The romanization standards** MixMusic writes lyrics in Latin letters
  by, on your computer:
  - Japanese: Hepburn romaji;
  - Korean: the Revised Romanization of Korean (2000);
  - Chinese: Hanyu Pinyin, with tone marks;
  - Cyrillic: BGN/PCGN for Russian, Kazakh and Kyrgyz, Ukraine's national
    system of 2010, Belarus's of 2007, Bulgaria's Streamlined System,
    Gaj's Latin alphabet for Serbian, Macedonia's official system of 2008,
    and MNS 5217:2012 for Mongolian;
  - Greek: ELOT 743, as the UN adopted it;
  - Armenian: BGN/PCGN (1981); Georgian: its national system of 2002;
  - the Brahmic scripts of India and Sri Lanka: Hunterian, India's
    national system;
  - Thai: the Royal Thai General System;
  - Lao: BGN/PCGN (1966); Khmer: the UN's system of 1972; Burmese:
    BGN/PCGN (1970);
  - Tibetan: Extended Wylie;
  - Hebrew: the rules of the Academy of the Hebrew Language (2006);
    Arabic, Persian and Urdu: the plain form of ALA-LC's;
  - Amharic: BGN/PCGN (1967).

## The look

- **The typefaces of the windows**, under the SIL Open Font License:
  - [Selawik](https://github.com/microsoft/Selawik), by Microsoft, the
    interface's face;
  - [Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily), by
    Andrew Paglinawan, the Mochi theme's;
  - [Roboto Flex](https://github.com/googlefonts/roboto-flex), the
    Material themes';
  - [Archivo](https://github.com/Omnibus-Type/Archivo), by Omnibus-Type,
    the Walnut themes' headings, in its semi-condensed extra bold.
- **[Material Color Utilities](https://github.com/material-foundation/material-color-utilities)**,
  by Google, whose HCT colour science the Material themes' colours are a
  port of.
- **[pywal](https://github.com/dylanaraps/pywal)** and
  **[matugen](https://github.com/InioX/matugen)**, whose colour files
  MixMusic follows.

## Standards MixMusic is written from

- Opus: RFC 6716, with RFC 8251, and the OpusHead of RFC 7845.
- AAC: ISO/IEC 14496-3, and the ADTS of ISO/IEC 13818-7.
- AC-3 and E-AC-3: ATSC A/52, and its Annex E.
- WebM: Matroska's EBML. MP4: ISO/IEC 14496-12.
- Live streams: HLS, RFC 8216.
- Loudness: ITU-R BS.1770, which YouTube measures with.
- Casting: multicast DNS, RFC 6762, and the Cast v2 protocol, from its
  public descriptions.

## Building and publishing

- **[Nix](https://nixos.org)**, which pins the tools releases are built
  with, and builds MixMusic for Linux from source.
- **[GoReleaser](https://goreleaser.com)**, which publishes the releases,
  and its [nFPM](https://nfpm.goreleaser.com), which makes the RPM
  packages.
- **[Bun](https://bun.sh)**, which runs the build, and
  **[TypeScript](https://www.typescriptlang.org)**, which compiles the
  plugins and checks the plugin SDK's types.
- **[NSIS](https://nsis.sourceforge.io)**, which makes the Windows
  installers, and [appimagetool](https://github.com/AppImage/appimagetool),
  the AppImages.
- **[Flatpak](https://flatpak.org)** and flatpak-builder, with
  [Flathub](https://flathub.org)'s GNOME runtime and its shared modules,
  which make the Flatpak.
- **[Zola](https://www.getzola.org)**, which builds this website.

## The music on this website

The songs of the website's player and of the README's walkthrough are by
**[Kevin MacLeod](https://incompetech.com)**, licensed under
[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/):
"Local Forecast - Elevator", "Carefree", "Wallpaper", "Inspired",
"Fluffing a Duck" and "Monkeys Spinning Monkeys", converted to AAC for the
website. They are
royalty-free, because we can't use copyrighted songs on our website or in
our README. None of them is part of the app.
