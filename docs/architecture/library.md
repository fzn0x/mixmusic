# The library and downloads

- **`internal/library`** keeps liked and disliked songs, mixes, the history
  and stats in `library.json`.
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
  as a video again. The proxy serves downloads before the cache
  (`stream.Store`), and the windows read a snapshot of what they hold in
  each frame, a map lookup a track row. Downloads hold stereo audio only:
  the proxy serves 5.1 surround audio from the cache or YouTube, and
  without the network it fails, for the player to play the download's
  stereo audio instead.
