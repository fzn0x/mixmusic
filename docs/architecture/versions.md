# Versions of albums

The services list an album that came out explicit and clean twice, and its
editions beside it. The lists of albums show one tile for each album,
whose page switches between its versions (#345).

- **Which albums are versions of one.** `media.GroupVersions`
  (`internal/media/versions.go`) groups a list. `VersionOf` sets apart
  the marks of versions at the end of a title: a part in brackets, or
  after a dash, made only of the words of versions (clean, explicit,
  edited, deluxe, remastered, edition, anniversary and the like, with
  years and ordinals), so that "(Live)" or "(Taylor's Version)" stay in
  the title. Albums of one service, kind and artists, of the same title
  without those marks, are one album, but for two cases. Of one edition,
  the albums whose years, or numbers of tracks where both tell them,
  differ are other releases. And where more than one release has no
  edition's mark, they're albums that share a title, as a self-titled
  album and a live one of that name: each stays apart, and an edition
  joins the one of its year, if only one is of its year. Albums of this
  computer and MixMusic's own aren't grouped.
- **Which is explicit.** `Album.Explicit` comes from YouTube Music's
  badge on the cards and rows of albums (`markExplicit`), and from the
  tracks of an album's page on both services. Spotify's lists of albums
  don't tell: `VersionsTold` says when a group's list tells, by its
  badges or its titles' marks, which version is explicit.
  `PreferredVersion` picks, of the versions of the first one's edition,
  the explicit one, or the clean one with `cleanVersions` or
  `hideExplicit` on (`PrefersClean`, Settings › Playback › Explicit
  content); with `hideExplicit`, the switch leaves out the explicit
  ones ([explicit.md](explicit.md)). `VersionLabels` names each
  version for the switch, by its edition and, where an edition comes
  both ways, by whether it's explicit.
- **The tiles and the page** (`versions.go`). `albumTiles` groups the
  list it shows once for each change of its albums, by page and key
  (`albumGroups`), and keeps each group of several by the ID of each of
  its albums. The tile shows the preferred version, and says how many
  there are. Opening it (`openAlbum`) goes to that version's page; when
  the list didn't tell which is explicit, the page reads every version
  first (`versionShown`) and shows the one their tracks tell. The switch
  under the page's head keeps the version chosen in `albumVersions.shown`,
  by the page's ID, without moving in the history. A tile's Play reads
  the same version (`albumToPlay`), from the pages read already where
  they were.
