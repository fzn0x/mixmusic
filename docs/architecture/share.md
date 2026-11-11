# Share

Share, in the menus of songs, albums, playlists and mixes, copies the
links to a page and saves an image card (#356). There is no server: links
go to the services' pages, and cards are drawn on this computer.

- **The card** is drawn by `internal/sharecard`, which knows nothing of
  the app: a `Card` has a `Size`, `Colors`, an optional cover, `Lines` and
  a `Foot` of `Line`s (text, size, weight, ink, most rows), and
  `Render` or `PNG` draws it with a `Family` of faces, offscreen, with
  `image/draw` and `golang.org/x/image`. `Story` (1080 by 1920), `Square`
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
- **Scripts.** The bundled faces have Latin, Greek and Cyrillic letters.
  A letter they lack comes from `Family.Fallback`, which the app fills
  with `SystemFallback`: fonts the system has, looked up by name (Segoe
  UI and Arial, the Yu Gothic, Microsoft YaHei and Malgun Gothic of
  Windows; Arial Unicode, Geeza Pro and PingFang on macOS; DejaVu and Noto
  on Linux), read once. Where none has the letter, it stays blank.
  Hebrew and Arabic read right to left: rows are reordered with
  `golang.org/x/text/unicode/bidi` and start at the right, and Arabic
  letters take their joined shapes from the presentation forms (`arabic.go`).
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
  `Library.Recap(year)` sums the history for one calendar year in the
  user's time zone, and `RecapYears` lists the years it has plays in.
  The history keeps only the last 1,000 plays, so a recap covers what
  that still holds; new artists are counted only when the history reaches
  back before the year. `recapYears` offers the past years, and the
  current one from December, going by `recapClock`, which tests replace.
  The card is built by `recapCardOf` for `saveCard`, with no cover: a
  square holds the minutes and the top songs, a story the top artists and
  albums too.
