# The streams

- **`internal/stream`** finds the audio of a YouTube video through YouTube's
  player API, asked signed out as YouTube's visionOS app (`Direct`), else
  with yt-dlp: its Opus stream, else, of a video YouTube has only in AAC,
  its AAC-LC one (itag 140), never HE-AAC (itag 139). It keeps the stream
  until its URL expires, and opens it for the audio engine
  (`Proxy.Open`): chunked fetches from YouTube in the background, a
  new URL when YouTube refuses the old one, a new connection, from the byte
  it reached, when a download gets no bytes for 15 seconds or its
  connection drops, and an on-disk cache of tracks played to their end.
  A `Source` tells whether its audio comes from this computer, a download
  or the cache (`Source.Local`), whose failure the engine words as the
  file's.
  YouTube lists no 5.1 surround audio to the visionOS app, so the direct
  route asks as YouTube's app for Samsung's TVs too, at once, which it
  tells of a video's AC-3 and E-AC-3 (formats 380 and 328). The stream
  doesn't wait for that answer: one that comes after it goes with it
  (`toldLater`), the resolver keeps it with the stream it keeps and tells
  the app (`Resolver.Told`), and the app the player
  (`Player.SurroundFound`), whose track then offers the 5.1 audio, and
  plays it from where it is while 5.1 is on. Their URLs
  need a challenge solved, which yt-dlp does with a JavaScript runtime:
  it finds the 5.1 audio as it is asked for (`Resolver.Surround`), once,
  in some 3 s, and YouTube serves it a few seconds after. Without yt-dlp
  and a runtime, no 5.1 audio is offered.
  A live video's audio is an HLS media playlist (RFC 8216) of AAC-LC in
  packed audio, format 234, or 233 with data saver, which `Proxy.OpenLive`
  follows from near its live edge: it loads the playlist again about once
  per target duration, fetches each new segment once, and finds the
  stream again before its URL expires, never caching it.
  After `pauseAfter` refusals of the player API in a row, the direct
  route asks for the last once more with a new visitor ID, then rests
  (`Direct.Stream`, `directrest.go`): `restFirst`, 2 minutes, then twice
  as long each time it is refused again after a rest, up to `restMax`,
  30 minutes, until it finds a stream. A try that runs out of its time
  counts as a refusal, without the new visitor ID, and the one after it
  is bounded by `slowTimeout`, 2 s, rather than the resolver's 4 s.
  The URL of a track's stream names each of YouTube's servers that hold
  it: when one doesn't connect within 3 seconds, the TLS handshake
  included, the stream's requests, and the direct route's check of it,
  ask the next, as YouTube's player does, and for 10 minutes ask the one
  that didn't answer last (`mediaServers`).
  yt-dlp runs quickly first (`Resolver.findWithYTDLP`): as its visionos
  client alone, `youtube:player_client=visionos;skip=hls,dash`, without
  `--js-runtimes`, the only client of yt-dlp 2026.10.26 whose streams need
  neither a challenge solved nor a PO token. When that finds no audio, it
  runs the full way (`Resolver.ytdlp`), with its default clients and the
  runtime, which alone finds 5.1 audio (`findSurround`); at once when the
  player API told the visionOS client that the video has no stream for it
  (`quickHelps`). The log tells which run found a stream, and how long
  each took.
  The player API also tells how loud YouTube measured each video
  (`Stream.Loudness`, `Direct.Loudness`). Both list the streams of a
  video's picture beside its audio (`Stream.Videos`), which the now
  playing page plays (`Proxy.Picture`, `Proxy.ServeVideo`): a cluster or
  fragment at a time, as the index at the start of YouTube's DASH
  streams tells (`parseIndex`), since WebView2 hands a page a custom
  scheme's response only once it is whole, and WebKitGTK plays no media
  from a custom scheme. Its answers are tested against recorded ones in
  `testdata/`, and against YouTube with
  `MIXMUSIC_LIVE=1 go test ./internal/stream -run Live`.
- **`internal/tools`** finds yt-dlp and a JavaScript runtime, and installs
  yt-dlp from its releases, checked against their published checksums. It
  downloads from the latest release's address, `releases/latest/download/`,
  and when GitHub fails that one alone, as it can, from the release's own
  address, by the version its page or GitHub's API names (#431). On
  macOS it installs the unpacked build, `yt-dlp_macos.zip`, into
  `bin/yt-dlp_macos/`: the single file starts in 8 s, the unpacked one in
  0.2 s (#245). A yt-dlp that comes with MixMusic (`Manager.Bundled`), as
  in Flathub's build, runs unless the user chose another, and none is
  downloaded beside it.
