# The windows

The root package draws the main window and the mini player. Pages fetch
what they show beside the main thread (`fetch`), and a window's state
belongs to its main thread: what finishes elsewhere comes back through
`update`.

- **Refresh** (F5, ⇧⌘R on macOS, `refresh.go`) asks again for the keys
  the page shown and the side panel fetched in the frame before, as
  `fetch` collects them, and renders the plugins' views again
  (`View.Remount`). The page keeps its answers until the new ones come,
  and keeps them when a refresh fails, which one toast tells. Home's own
  tile and shelves read the library as it is, and need no refresh.
- **Pull to refresh** (`pullrefresh.go`): the scroll container of the
  page shown (`pageScroll`) takes its scrolling first, with
  `HandleInput`, before MyGo scrolls anything. A pull starts only on a
  page at rest at its top: scrolling that came up to the top goes on to
  its gesture's end without pulling, as Windows and Linux send a fling's
  momentum as any scrolling, and macOS's, which MyGo marks (`Momentum`),
  never pulls. A touchpad pulls by its distance (`Precise`), a wheel by
  its notches, three of which go far enough. A gesture ends once no
  scrolling came for a quarter of a second, or as the fingers leave,
  which macOS and Linux tell with an event that scrolls nothing; let go
  past the threshold, the pull runs Refresh. A strip above the page's
  content, which is keyed so that it keeps its state, opens with the
  pull. Scrolling sideways, or with a modifier, as Shift+wheel, never
  pulls, nor does scrolling a scroller inside the page, whose own
  handler marks it (`scrollsInside`), as the lyrics of the now playing
  page do.

- **Long playlists** (`playlistread.go`): a playlist's page, and the
  songs liked on YouTube Music in the library, show their first page of
  tracks at once and read the rest beside it, telling how many came, how
  many could be read when a page failed for good, with a way to try
  again, and where a list longer than 20,000 was cut. Play and Shuffle
  take the whole list: pressed while it's read, they wait for it, unless
  another track started meanwhile. The items of the page's menu that
  take its tracks, and its download, show once the list is whole. The
  queue builds only its entries near the view, as it may hold thousands.

- **Menus** are the system's: MyGo builds one as it opens, and again for
  the item chosen, never while it shows, so what a menu lists is fetched
  before it opens. Signed in to YouTube Music, the main window reads the
  user's playlists there at once (`keepYouTubePlaylists`), which a
  track's "Add to YouTube Music playlist" lists, and keeps them: one made
  goes in first, and one deleted leaves, without reading the list again
  (`keepPlaylist`, `dropPlaylist`); signing in or out, or choosing
  another channel, reads it again. An add from those menus to a
  playlist, or to a mix, that holds the track already says so in a
  toast, with Add anyway (`alreadyin.go`): YouTube Music refuses it, and
  a mix leaves it out (`AddNewToPlaylist`); Add anyway adds it a second
  time.
- **Keep on top** (`keepontop.go`): the main window opens above the
  others while `keepOnTop` is on (`WindowOptions.AlwaysOnTop`), and each
  frame of it compares the setting with what the window was told last,
  so that the switch, the command palette, macOS's Window › Keep on Top
  and settings.json all reach it, and tells the window once
  (`Window.SetAlwaysOnTop`, through `keepAbove`, which tests replace).
  The Window menu's item is a checkbox whose label is fixed, as MyGo
  builds the menu as it opens; the frame checks it as the setting
  changes. The mini player's window is always on top, by its own options.

- **The top bar and the rows pinned under it** (`compacthead.go`). The
  page shown keeps where it scrolled in a `ui.ScrollState`
  (`pageScroll`), which the top bar, built before the page, reads in the
  same frame. The heads of playlists, albums, artists
  (`collectionHead`) and the liked songs (`likedHead`) tell how far down
  the page their buttons end, as the frame before laid them out; past
  there, the top bar shows the head small, from the same data: artwork,
  title, play, shuffle and download, fading in with `AnimateTo`, at once
  while the computer asks for less motion. Where the bar is narrow, the
  download goes into its menu. The search and the sort of a list of
  tracks (`trackTools`) stay pinned under the top bar once the page
  scrolls past them (`pinRow`): the same element, made absolute at the
  page's offset over the ground repainted (`groundOn`), so that the box
  typed in keeps the focus. A search or a sort there shows the list from
  its start under them, and the page keeps the room for it. Nothing
  scans a list for this in a frame.

- **Artwork** downloads only for what shows in the window, scaled to the
  size it shows at, and the images shown most recently are kept, up to 48
  MB of pixels (`artwork.go`). An image that couldn't load tries again
  after 1 to 2 seconds, then less often, up to every 2 minutes, and at once
  when another of its host loads; one whose address gives no image waits
  half an hour. Both windows, the accent, Terminal's tint and the
  system's media controls read the images through `artwork.Store`
  (`internal/artwork`), which downloads each once, keeps it on disk as it
  came, up to 256 MB, and logs a host's failures at most once a minute,
  without the addresses.
- **Themes** give their tokens: colours, fonts, radius (`theme.go`), and
  the accent follows the artwork of what plays. A colour file's colours
  replace the theme's, kept readable (`colorfile.go`). The layout design
  the user chose, Blend, Expressive, Crate or Tiles, sets the shapes, the
  density and the styles of the parts in whatever theme (`layout.go`).
  The windows draw the effects of the built-in themes, as their options
  say (`effects.go`): Mix's textures, Aurora's drifting lights and frosted
  dialogs, Sakura's petals, Terminal's scanlines, glow and phosphor
  artwork. The Material themes make their colours of a seed in HCT
  (`material.go`, `internal/hct`, a port of Material Color Utilities),
  unless a colour file gives them.
- **Lyrics** light the line sung now, and its words as they are sung when
  they have times (`lyricswords.go`); in Latin letters made on the
  computer, under the line or in its place, each word's romanization
  lights at its word's times (`lyricsromanize.go`, `sungText`), where
  Unison's and romaji's, which have no words, light whole, as below.
  Lines of TTML synced word by word
  keep their words' times (`lyrics.Line.Words`): each span with a begin
  time is a word, or a syllable, and a line with sung text outside of them
  has none. The line's text element is clear, and its Draw paints the
  glyphs of a layout of its own, the same, cut where the song is, the part
  sung in the accent. The song's time moves on from the position the
  player last told, at the track's speed, for 1.5 s at most, so the words
  fill in frames that only paint again; Reduce Motion lights each word
  whole as it starts. Lines shown in Latin letters in their place light
  whole.
- **The mini player's volume** (`minivolume.go`) is a button that mutes,
  and the player bar's slider above it, in a popover that shows while the
  pointer is over either, a drag holds it or the keyboard's focus is on
  either, and for 0.4 s after. The wheel over it, as over the player
  bar's volume, steps the volume (`wheelVolume`, `volume.go`). Escape
  gives the focus back to the button itself: MyGo gives an overlay's
  focus back to what had it as the overlay opened, which it may take
  from a tip that showed in its place before. `miniRoom` keeps the
  button's room beside the artwork.
- **Deep links** and second launches open their pages in the main window
  (`App.OnNavigate`).
