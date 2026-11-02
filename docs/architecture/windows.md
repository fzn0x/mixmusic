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
- **Deep links** and second launches open their pages in the main window
  (`App.OnNavigate`).
