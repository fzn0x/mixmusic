# Explicit content

Hide explicit content, in Settings › Playback › Explicit content
(`hideExplicit`, #427), keeps the songs, videos and albums marked explicit
out of the lists the services make, and from playing. The user's own lists
keep them, dimmed.

- **What's explicit** (`internal/media/explicit.go`). `Track.Explicit`
  comes from Spotify's tracks, YouTube Music's badge on its rows and cards
  (`isExplicit`), and the content advisory of a file's tags:
  `internal/tags` reads MP4's `rtng` item, and `ITUNESADVISORY` or
  `EXPLICIT` as an ID3v2 TXXX frame, a Vorbis comment or an MP4 item
  (`advisory.go`), and `internal/localfiles` keeps it in `Entry.Explicit`;
  the index's version 3 reads every file again for it. `ExplicitSong`
  leaves podcasts' episodes out, which the setting leaves alone.
  `ExplicitAlbum` is an album its service badges, or whose title marks it
  explicit; `ExplicitVersions` tells, of an album's versions, the
  explicit ones as `versionRatings` does. `WithoutExplicit` and its kin
  give a list, a search's results or an artist's page without them, the
  list itself when it holds none.
- **The lists** (`explicit.go`, in the root package). One filter for each
  kind of list, where the pages read it, while the setting is on, so that
  turning it on or off shows at once, without asking the services again:
  `shownResults` for a search, `shownItems` for what the search box
  suggests and `shownShelf` in `itemShelf`, which Home, Explore, the moods
  and the now playing page's shelves go through, `shownArtist` for an
  artist's page, and `shownPlaylist` for a playlist that isn't the user's
  own (not a mix, nor one they may edit), kept in the loads beside the
  page's so that its track tools' view stays. Home's tile of what played
  last, the new releases and the songs suggested for a playlist check
  each with `hides` and `hidesAlbum`. An album's page leaves the explicit
  versions out of its switch, and its tile out of its count
  (`shownVersions`); `PrefersClean` has its tile and its page open the
  clean version. Rows of a track the setting hides, in the user's own
  lists, the library and on an album's page, are dimmed, with their "e"
  and "hidden by your settings" (`explicitMark`), and so are the entries
  of the queue (`queueTitle`).
- **Radios** (`internal/app/catalog.go`). `Catalog.Radio`, which autoplay
  and Start radio read, leaves out the explicit songs while the setting
  is on, as it leaves out disliked ones.
- **The player** (`internal/player/explicit.go`). An entry of a track the
  setting hides is passed over as `SkipExplicit`, from the moment it's
  made (`skipOf`), and unlike the other reasons, the user choosing it
  doesn't lift it (`keptSkip`): a move passes over it without a notice,
  and `playAt`, `playTracks` and `PlayWithRadio` refuse it with a notice
  that says why (`refuses`), leaving the queue as it was. `PlayAll` of
  explicit songs alone tells so (`explicitPassedOver`). Turning the
  setting on marks the queue again (`hideChanged`), and moves on from the
  current entry when it's one of them, as a dislike does.
