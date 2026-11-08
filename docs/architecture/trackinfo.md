# Track info

Track info, in every track's menu, shows in a dialog what a track is and
how it plays (#342). Nothing in it asks a service: it tells what
MixMusic's files, the resolver and the engine hold.

- **What a file holds.** `audio.Inspect` reads the headers of a file as
  a deck opens it (`openTrack`), and `describe` tells, of the track it
  opened, its kind of file, codec, sample rate, bit depth and channels,
  from what its headers told: an MP4's sample entry, a WebM's or an
  Ogg's OpusHead, a FLAC's STREAMINFO, a WAV's format chunk, or of an
  MP3, its first frame's header, which `Inspect` reads as `Probe` does,
  without the scan of every frame a deck makes.
- **What the engine plays.** A deck keeps what `describe` told as it
  opens its track, and `Engine.Loaded` tells, of the active deck, the
  URL it loaded, that, and the loudness its `Measured` source told, with
  the trim Even loudness plays it at.
- **What a track plays from.** `Player.TrackInfo` (`internal/app`) takes
  the track, and how it plays when it's the one playing, as the player's
  state tells it. A file on this computer is its own file, read with its
  tags (`internal/tags`), its picture left out. A track of a service
  plays its recording on YouTube Music: the one playing, else the one its
  download holds, else the one the matcher found before
  (`Matcher.Matched`), and the proxy serves it, as `Player.TrackInfo`
  follows it, from its download (`Snapshot.VideoFile`), else the offline
  cache's copy (`Cache.Find`), else the stream the resolver holds of it
  (`Resolver.Known`, `Resolver.KnownSurround` while it plays its 5.1
  surround audio), which it doesn't find when it holds none. The book of
  loudness tells how loud YouTube measured it. Of the track that plays in
  the engine, while the engine's active deck plays its URL, or its 5.1
  surround audio's, what the engine tells takes the place of the rest.
  A Spotify track that Spotify's own player plays, in MixMusic or on a
  Connect device, tells only that: Spotify tells MixMusic nothing of its
  audio.
- **The dialog** (`trackinfo.go`) fetches it as the page's answers are
  fetched, once each time it opens, and of the track that plays, again
  once a second for 10 seconds at most, until the engine opened it or
  the track started or stopped playing. `trackFacts` words the rows, and
  tests give the dialog its answers through `trackInfoOf`. Show in
  Explorer, Finder or its folder shows the file of a track on this
  computer, or its download, through `showInFolder`.
- **Copies.** `trackCopies` is the place a song's copies are to show,
  side by side, the one that plays marked, once MixMusic links them
  (#340). It shows nothing yet.
