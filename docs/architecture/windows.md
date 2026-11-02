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

- **Artwork** downloads only for what shows in the window, scaled to the
  size it shows at, and the images shown most recently are kept, up to 48
  MB of pixels (`artwork.go`). An image that couldn't load tries again
  after 1 to 2 seconds, then less often, up to every 2 minutes, and at once
  when another of its host loads; one whose address gives no image waits
  half an hour. Both windows, the accent and Terminal's tint read the
  images through `artwork.Store` (`internal/artwork`), which downloads each
  once, keeps it on disk as it came, up to 256 MB, and logs a host's
  failures at most once a minute, without the addresses.
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
  they have times (`lyricswords.go`): the line's text element is clear,
  and its Draw paints the glyphs of a layout of its own, the same, cut
  where the song is, the part sung in the accent. The song's time moves on
  from the position the player last told, at the track's speed, for 1.5 s
  at most, so the words fill in frames that only paint again; Reduce
  Motion lights each word whole as it starts. Lines shown in Latin letters
  in their place light whole.
- **Deep links** and second launches open their pages in the main window
  (`App.OnNavigate`).
