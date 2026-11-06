# Webviews

Three web pages still show, and only while they're needed:

- **Spotify in MixMusic itself** (`internal/app/spotifyhere.go`): Spotify's
  Web Playback SDK, which decrypts Spotify's protected audio, in the hidden
  window of `public/spotify.html`, one of the app's two pages, which it
  embeds.
  It opens at the first Spotify track that plays in MixMusic, and makes
  MixMusic a Spotify device of its own, which the player plays tracks on
  through the Web API and follows by asking the page (`Page.Eval`); the page
  gets its token from `SpotifyPage.Token`, and tells `SpotifyPage.Changed`
  when what it plays changes, for the player to ask at once. A track plays
  with the Spotify tracks the queue moves on to after it, which the device
  goes on to without a gap, and the player follows it there
  (`internal/player/connect.go`). At a track's end, the player gives it up
  to 2 s to go on before it plays the next track itself: after a seek into
  a track's last seconds, Spotify stops on it a moment while it loads the
  next. WebView2 decrypts Spotify's audio with Widevine, and macOS's
  WKWebView with FairPlay, where the window keeps its page running while
  hidden (`herePage`). WebKitGTK has no EME:
  [Spotify in MixMusic on each system](spotify-here.md) tells what each web
  view gives the SDK.
- **Google's sign-in to YouTube Music**, whose cookies MixMusic keeps.
- **The picture of music videos** (`video.go`, `internal/app/video.go`),
  with "Show the video of music videos" on in Settings › Playback: a web
  view of the main window, on the now playing page, in place of the
  cover, of `public/video.html`, the other page of the app. It is made
  when a picture first shows, and closed a minute after the last one. The
  page plays the picture, muted, through Media Source Extensions, fetching
  it from the proxy at the stream scheme, which serves that page alone,
  and follows the engine's time (`Player.Clock`), which the window sends
  it four times a second with the moment it read it. A box over the web
  view keeps the pointer and the keyboard in the native interface.
