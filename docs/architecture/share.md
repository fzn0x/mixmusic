# Share

Share, in the menus of songs, albums, playlists and mixes, copies the
links to a page and saves an image card (#356). There is no server: links
go to the services' pages, and cards are drawn on this computer.

- **The card** is drawn by `internal/sharecard`, which knows nothing of
  the app: a `Card` has a `Size`, `Colors`, an optional cover, `Lines` and
  a `Foot` of `Line`s (text, size, weight, ink, most rows), and
  `Render` or `PNG` draws it with a `Family` of faces, offscreen, with
  `image/draw`, `golang.org/x/image` and `github.com/go-text/typesetting`.
  `Story` (1080 by 1920), `Square`
  (1080 by 1080) and `Wide` (1200 by 630) are the sizes, and sizes of
  text are for a card 1080 wide, which other widths scale. The yearly
  recap draws its cards with it too.
- **The layout.** The cover is cut to a square with rounded corners. On
  a tall card (the story) it sits above the lines across their whole
  width, so its margins are equal; on a card about as wide as it is tall
  (the square and the wide one) it sits beside the lines; and with
  `CoverFoot` it is small at the foot, before the foot's lines. Lines run from the top and wrap at spaces, anywhere in text of a
  script written without them, and end with an ellipsis past their most
  rows. If all do not fit, their text shrinks to 55% of its size, then
  rows go from the end. The foot keeps its place at the bottom. The ground
  is the background, tinted by the accent at the top and plain from 60% of
  the way down, so the foot's corners are exactly the background.
- **Scripts.** The bundled faces have Latin letters; Selawik, the
  default, has no Greek or Cyrillic ones. A letter they lack comes from
  `Family.Fallback`, which the app fills with `SystemFallback`: fonts the
  system has, read once, in the order a letter is looked up in them. On
  Windows they are Segoe UI and Arial, then Nirmala UI (the scripts of
  India), Leelawadee UI (Thai, Lao, Khmer), Ebrima and Gadugi (Ethiopic
  and more), Himalaya, Javanese Text, Myanmar Text, Sylfaen, then the Yu
  Gothic, Microsoft YaHei and Malgun Gothic faces, then Segoe UI Emoji
  and Segoe UI Symbol. On macOS they are looked up by name in the system,
  Supplemental, `/Library` and `~/Library` font folders (Arial Unicode,
  Geeza Pro, the Kohinoor and Sangam MN faces, Thonburi, Kefa, PingFang,
  Hiragino Sans GB, Apple Color Emoji). On Linux, files named like Noto
  Sans's (`-Regular`, `-VF`, the axes in brackets, `UI`, the CJK
  collection) and Noto Color Emoji's are found under
  `~/.local/share/fonts`, `~/.fonts`, `$XDG_DATA_DIRS/fonts` and
  `/usr/share/fonts`, and `fc-match` is asked for the font of each
  language, so a font the user set counts whatever its file is called. A
  collection gives its first face. Where none has the letter, it stays
  blank. Fonts are read with go-text (`font.go`): which letters a system
  font has is read up front, the rest when a card needs one of them, and
  held weakly, so it goes once the card is drawn.
- **Shaping** (`shape.go`). go-text's segmenter splits a row into runs of
  one direction and script, and the fallback order splits those by font,
  a mark, a joiner, a variation selector or a space staying in the font
  of the letter before it. Each run is shaped by go-text's port of
  HarfBuzz at the size of its font's units, which any size scales:
  Devanagari and the other scripts of India form their conjuncts, Thai
  marks stack, and Arabic joins. Runs show in the order of their
  bidirectional levels, and a line that reads right to left starts at
  the right. Rows break between clusters, never inside one. A variable
  font is set to the weight of its line. The outlines, TrueType, CFF and
  CFF2, are filled with `golang.org/x/image/vector` (`glyph.go`).
- **Colour** (`color.go`). A glyph in colour is drawn from the layers of
  a COLR table's first version, in the colours of the first CPAL palette
  (Segoe UI Emoji), from an sbix bitmap (Apple Color Emoji, whose
  bitmaps are read from the file one at a time), or from a CBDT bitmap
  (Noto Color Emoji). COLR's second version, whose glyphs are gradients,
  is not drawn. A letter followed by U+FE0F comes from a font that has it
  in colour.
- **The menu** (`share.go`) has a `Share` submenu, built by `shareItem`
  beside Copy link in each menu: `songShare`, `albumShare` and
  `playlistShare` say what a target is. A Spotify song's links include the
  page of its YouTube Music match (`Player.Matched`). `saveCard` asks for
  the path with the system's save dialog (`saveFileDialog`, which tests
  replace) beside the main thread, fetches the cover through the same
  artwork source the windows use, so a mix's own cover works, draws the
  card in the theme's colours (`cardColors`) and typeface (`cardFamily`),
  and writes it.
- **Lyrics** (`lyricsshare.go`): `lyricPick` holds the lines chosen by
  index. While it is on for the track playing, `pickLine` makes a click
  choose a line instead of seeking, and `saveLyricsCard` makes the card.
- **The yearly recap** (`recap.go`, `internal/library/recap.go`):
  `Library.Recap(year)` sums the play log for one calendar year in the
  user's time zone, and `RecapYears` lists the years it has plays in.
  The log keeps every play since it was made, and the last 1,000 of the
  history before it; new artists are counted only when the log reaches
  back before the year. `recapYears` offers the past years, and the
  current one from December, going by `recapClock`, which tests replace.
  The card is built by `recapCardOf` for `saveCard`, with no cover: a
  square holds the minutes and the top songs, a story the top artists and
  albums too.
