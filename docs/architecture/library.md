# The library and downloads

- **`internal/library`** keeps liked and disliked songs, mixes, the history
  and stats, and the artists followed in MixMusic
  ([Following artists](follows.md)), in `library.json`.
- **`internal/localfiles`** keeps the index of the music on the user's
  computer, `local-files.json`: the music files of the folders the
  settings name (`musicFolders`), by path, with their size, time of
  change and tags, which `internal/tags` reads (ID3v2 and ID3v1, MP4's
  `ilst`, Vorbis comments and FLAC's PICTURE, WAV's INFO), and whether
  the engine plays them, as `audio.Probe` tells. A scan, at the start,
  when the folders change and on demand, reads only the files new or
  changed by their size or time of change, four at a time, drops those
  gone, and keeps those of a folder that can't be read until a scan
  reads it: a music folder that isn't there at the scan's end, as on a
  drive unplugged partway, and a folder in one whose list of files, or
  a file's information, fails to read other than as gone
  (`Folder.Unread`, which Settings counts; `Missing` looks at their
  files). Covers, held or
  beside the files, go to the cache's `local-art`, by their hash. A track's
  ID is its file's path in base64 (`file:<path>`), so that one in a mix
  or the queue plays as long as its file is there; the player plays it
  from the file (`prepareFile`), and it goes to no service: the matcher
  refuses it (`match.ErrLocal`), no radio starts from it, and a cast
  device isn't handed it. One whose file is gone is missing until a scan.
- **`internal/downloads`** keeps the tracks the user downloads, apart from
  the offline cache: a folder of their audio (`audio/<video id>.webm`, or
  `.m4a` for a track YouTube has only in AAC) and covers (`art/`), with
  an `index.json` of what it holds, its paths relative to the folder, so
  that it moves whole. A Spotify track downloads as its recording on
  YouTube Music, and tracks of one recording share a file. Two workers
  download what waits, oldest first, a few seconds apart, through
  `Proxy.Download`: the stream's chunks, its URL found again when YouTube
  refuses it, connecting again from the byte it reached, into a `.part`
  file that a later run goes on from when YouTube sends the same file
  (its itag, length and last change). A track the offline cache holds
  comes from the cache. Each file is read as the engine reads it, Opus in
  WebM or AAC-LC in MP4, before it's kept. Downloads wait while paused,
  while the network is gone, on a metered connection unless the settings
  allow it, for 15 minutes once YouTube turns one down, and on a full
  disk; a track YouTube won't send fails with why, and others try again
  after 30 seconds, 2 minutes and 10. A live stream has no file to keep:
  the index keeps the videos playback or a download found live, whose
  tracks it leaves out of their collections and refuses, until one plays
  as a video again. A collection downloaded keeps the tracks taken out of
  it since, until its download is removed: MixMusic can't tell them from
  those a service stopped listing, as a video made private. Its page
  brings it up to date (`syncDownload` in `downloadsync.go`), once for
  each of the page's answers: `SyncCollection` downloads the tracks its
  download never held, and leaves those it held as they are, so that one
  that failed or whose download was removed isn't asked for again. As
  many tracks as Download asks before wait for the head's button, and
  the setting `downloadAdded` turns it off. Nothing happens in the
  background: knowing what was added to a playlist means reading it
  whole. The proxy serves downloads before the cache
  (`stream.Store`), and the windows read a snapshot of what they hold in
  each frame, a map lookup a track row. Downloads hold stereo audio only:
  the proxy serves 5.1 surround audio from the cache or YouTube, and
  without the network it fails, for the player to play the download's
  stereo audio instead.
