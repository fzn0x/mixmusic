# Where the audio streams come from
<!-- docs: group=inside order=30 -->

MixMusic finds the audio streams of YouTube Music tracks itself, through YouTube's player API, and with yt-dlp for the rest. Here are the formats and rules.

Every track MixMusic plays in its own engine is a YouTube video: YouTube
Music's songs, music videos, uploads, podcast episodes and live streams,
and Spotify tracks played from their recording on YouTube Music. This page
tells how MixMusic gets their audio, and what it does when YouTube makes
that hard. For the code, read
[the streams' part of the architecture](architecture/stream.md).

## YouTube's player API

For each track, MixMusic asks YouTube's player API itself, the way
YouTube's own apps do: as YouTube's app for visionOS, signed out.

- **Why that app.** YouTube answers it with Opus streams whose addresses
  play as they are given, with no signature to work out and no challenge
  to solve, and without a pre-roll ad, so YouTube serves them at once.
  The answer also tells how loud YouTube measured the video, which
  [Even loudness](audio-engine.md#even-loudness) uses.
- **Signed out, always.** MixMusic sends no cookie and no account with
  it, also while you're signed in to YouTube Music: with an account,
  YouTube answers with fewer streams, often none that plays.
- **A visitor ID.** YouTube gives each visitor an ID, which MixMusic keeps
  and sends back, as a browser would.
- **Checked before it plays.** MixMusic asks YouTube's server for the
  first byte of the stream it picked before the player gets it, so that a
  stream YouTube refuses goes to yt-dlp rather than failing in the player.
- **Fast.** It takes a fraction of the time yt-dlp takes, which is why
  tracks start in a moment since 0.5.6.

When YouTube changes what it asks of this app, MixMusic's requests change
with it, in an update of MixMusic.

## When yt-dlp is needed

MixMusic runs [yt-dlp](https://github.com/yt-dlp/yt-dlp) for what the
player API won't give it:

- **Videos YouTube gives that app no streams of**, such as those made for
  children.
- **Videos whose streams need a signature or a challenge solved**, which
  YouTube's web player does in JavaScript. yt-dlp solves them with a
  JavaScript runtime: Deno, Node.js, Bun or QuickJS. Without one, yt-dlp
  can't read most streams.
- **5.1 surround mixes**, as below.
- **A rest from the player API.** When YouTube's player API refuses
  three streams in a row, MixMusic leaves it alone for 30 minutes, and
  yt-dlp finds every stream meanwhile. The log says so.

yt-dlp runs signed out too, with its own cache in MixMusic's cache folder,
for at most 90 seconds a track. Install it, and choose the JavaScript
runtime, in Settings › Playback › Tools;
[how it updates](how-it-works.md#yt-dlp-and-the-javascript-runtime) is on
the first page of this section. When yt-dlp finds no stream, MixMusic says
why in plain words, such as "this track is not available in your
country", "only the members of its channel can play this track" or
"YouTube plays this track only to signed-in adults, on its own website".

yt-dlp runs twice at most for a track, a quick way first:

- **The quick run** asks the player API as the visionOS app alone, as
  MixMusic's own requests do, without the JavaScript runtime and without
  the HLS and DASH manifests. It solves no challenge, which takes the
  full run several seconds of the computer's time, and YouTube serves its
  streams at once. It runs when the player API rests, doesn't answer in
  time, or refuses MixMusic's own request.
- **The full run** asks as yt-dlp's default apps, with the JavaScript
  runtime: when the quick run finds no audio, and at once when the player
  API said that the visionOS app gets no stream of the video, as of one
  made for children. It alone finds 5.1 surround mixes, so a track the
  quick run found offers none.
- **The log** says which run found the stream, and how long each took.
  Before 0.5.11, yt-dlp ran the full way only.

## Formats and quality

Settings › Playback › Sound › Quality chooses between best, the default,
and data saver, which streams smaller files. Downloads take the same
quality.

| | Best | Data saver |
|---|---|---|
| Songs and videos | the Opus stream of the most bits | the smallest Opus stream |
| Videos YouTube has only in AAC | AAC-LC in MP4, format 140 | the same |
| Live streams | AAC-LC at 44.1 kHz in stereo, about 128 kbit/s, format 234 | HE-AAC at 22.05 kHz, format 233, whose AAC-LC core the engine plays |
| The picture of music videos | about the size of its frame on screen, at most 480 lines | the smallest picture |

Of a video's audio, MixMusic picks:

- **The original audio track**, before a dub, and audio descriptions last.
- **Stereo or mono.** A 5.1 surround format is never picked as a track's
  stereo stream, though it is the best of them; it comes beside it.
- **The original dynamics.** YouTube also serves a few videos in a
  stable-volume version, whose format names end in "drc", whose sound is
  squeezed and no smaller. MixMusic takes it only when the video has no
  other of its codec.
- **Unencrypted and whole**: never a stream cut short.
- **Never format 139**, YouTube's HE-AAC: a video in AAC alone plays its
  AAC-LC stream, format 140.

## Stream addresses expire

- **How long they last.** A stream's address works for some hours, as
  its `expire` parameter tells, or six hours when it doesn't tell.
  MixMusic keeps each stream it found until 30 minutes before it expires,
  then finds it again, so that a track started near the expiry plays to
  its end.
- **A refused address.** When YouTube refuses an address while a track
  plays (HTTP 403), MixMusic finds the stream again and goes on from the
  byte it reached.
- **`available_at`.** YouTube refuses a stream that yt-dlp found for a few
  seconds after it gave out the address: its own player would show an ad
  meanwhile. yt-dlp tells the moment YouTube starts to serve it,
  `available_at`, and MixMusic waits until then, up to 30 seconds, rather
  than ask at once and be refused. Before 0.5.1 it asked at once, and such
  a track failed with "YouTube refused to send its audio". Streams of the
  player API play at once, and the next track is looked up a second into
  the one before, so the wait seldom shows.
- **Never in the log.** A stream's address carries your IP address, so
  the log never keeps one.

## Servers that don't answer

YouTube keeps each stream on several of its media servers, and the
stream's address names all of them. When the one MixMusic asks doesn't
connect within 3 seconds, the TLS handshake included, MixMusic asks the
next, as YouTube's own player does, and for 10 minutes asks the others
before the one that didn't answer. Any answer of a server, a refusal too,
is the stream's: no other server is asked then. Before 0.5.9, such a track
waited on its one server and failed with "MixMusic could not reach
YouTube's audio servers", while other tracks played.

A stream that stops coming, or whose connection drops, connects again from
the byte it reached, for up to two minutes:
[the audio engine](audio-engine.md#buffering) tells how.

## Live streams

Live videos, such as YouTube's 24/7 radio channels, play for as long as
the broadcast goes on.

- **How.** YouTube gives a live video's audio as an HLS media playlist
  (RFC 8216) of AAC in packed audio, format 234, or 233 with data saver.
  MixMusic loads the playlist again about once per segment's length, and
  fetches each new segment once, in order.
- **How far behind.** It starts three segments from the end of the
  playlist, as RFC 8216 asks of a player, about 15 seconds behind the
  broadcast, which leaves room to ride out a slow network.
- **After a pause.** MixMusic holds up to four segments you haven't heard
  yet, then waits. Played again, the stream goes on from where it was
  while YouTube's playlist still has the next segment, which it does for
  about an hour; after a longer pause, it goes on from near the live
  edge.
- **What's different.** The player bar and the mini player say "live" in
  place of a time, with no seeking, and Previous goes to the track
  before. A live stream plays at 1× and at its own level, as YouTube
  measures no loudness of it. MixMusic finds its stream again before the
  address expires. A live stream is never kept in the offline cache,
  never downloaded and never scrobbled, and it doesn't cast yet. It ends
  when the broadcast does.

## Surround mixes and what they need

Some tracks have a 5.1 surround mix beside their stereo one, which
[the audio engine](audio-engine.md#surround-sound) plays: AC-3, format 380,
or E-AC-3, format 328.

- **Asking a TV app.** YouTube lists no 5.1 audio to the visionOS app.
  So, at the same time, MixMusic asks the player API as YouTube's app for
  Samsung's TVs, which YouTube tells of a video's AC-3 and E-AC-3. A
  track doesn't wait for that answer: it starts as soon as its stereo
  stream is found and checked, and when the answer comes after, the 5.1
  button shows a moment later, and on later plays of the track at once.
  Before 0.5.11, a track waited up to half a second for the answer,
  which on a slow connection often came late or not at all.
- **yt-dlp and a JavaScript runtime.** The addresses of these streams
  need a challenge solved, which only yt-dlp does. MixMusic has it find
  the mix only when it is asked for, as you press 5.1 or a track starts
  with "Play 5.1 surround when a track has it" on, once a track, in about
  3 seconds; YouTube serves it a few seconds after. Without yt-dlp and a
  runtime, no 5.1 is offered.
- **Which.** AC-3 first, then the mix of the most bits, of the original
  audio track.
- **Offline.** Downloads keep the stereo audio only, which plays while
  you're offline; the offline cache keeps the 5.1 mix beside it.

## The picture of music videos

Turn on "Show the video of music videos" in Settings › Playback › Music
videos, or press the video button by the transport while one plays.

- **What shows it.** It shows only for official music videos and other
  uploads, never for songs or live streams. MyGo draws no video, so a web
  view of the main window shows a page of MixMusic's own over the cover,
  which plays the picture, muted, through the browser's Media Source
  Extensions. The sound stays in MixMusic's engine, with its equalizer and
  its speed, and the picture follows the engine's clock, which MixMusic
  sends it four times a second.
- **Where it comes from.** YouTube lists a video's pictures beside its
  audio, as streams without sound. MixMusic picks the codec the web view
  plays best, and the smallest picture at least as large as the frame it
  shows in, of at most 480 lines, and of two of a size the one of at most
  30 frames a second; with data saver, the smallest. Its stream proxy serves it to the page a part
  at a time, as the index at the start of the stream tells.
- **What it costs.** About 3 MB a minute more than the sound, under 1 MB
  with data saver, fetched only while the picture shows and plays:
  nothing on another page, with the window hidden or minimized, or with
  the mini player open. The web view takes memory, and closes a minute
  after the last picture.
- **What it needs.** WebView2 on Windows, WebKitGTK on Linux. Without
  them, the cover stays. While your system asks for less motion, a button
  on the cover plays it.

## Downloads and the cache

MixMusic keeps audio on disk two ways:

| | Offline cache | Downloads |
|---|---|---|
| What | tracks you played, once their whole audio came | tracks, albums, playlists, mixes and liked songs you download |
| How long | until it is full: the tracks played least recently go first | until you remove them |
| Where | `audio` in the cache folder | the downloads folder, which Settings › Playback › Downloads can move |
| Quality | the quality they played in | the quality of Settings › Playback |
| 5.1 | kept beside the stereo audio, while 5.1 is on | stereo only |
| Clearing | Settings › Playback › Offline cache › Clear tracks | Remove download in its menu, or remove all downloads in Library › Downloads |

- **Which comes first.** A track plays from its download, then from the
  cache, then from YouTube. Downloading a track the cache holds moves it
  out of the cache, with no network.
- **How downloads go.** Two at a time, the oldest first, a few seconds
  apart. A download that stops goes on where it stopped, after a restart
  too, when YouTube sends the same file. Downloads wait while you're
  offline, on a metered connection unless you allow it, for 15 minutes
  once YouTube turns one down, and on a full disk. A track YouTube won't
  send fails with why; others try again after 30 seconds, 2 minutes and
  10 minutes.
- **What they hold.** The downloads folder holds each track's audio,
  `audio/<video id>.webm`, or `.m4a` for a track in AAC, its cover in
  `art/`, and an `index.json` of what it holds. Its paths are relative, so
  the folder moves whole.
- **Spotify tracks** download as their recording on YouTube Music. A live
  stream doesn't download.

## Proxies

MixMusic connects through a proxy when one is set, for every request it
makes: the catalog, the streams, the covers and yt-dlp.

- **On every system**, the proxy that the environment names, as
  `HTTPS_PROXY`, `HTTP_PROXY` and `NO_PROXY`, comes first.
- **On Windows**, failing that, MixMusic takes the proxy of Windows'
  settings, as browsers do: automatic setup first, by detecting the
  settings or by a setup script, which Windows answers for each site and
  MixMusic remembers for 5 minutes, then the proxy server you set, unless
  its bypass list names the address. It follows the settings as VPN and
  proxy apps turn them on and off, and hands yt-dlp the proxy where
  yt-dlp wouldn't find it itself. The log says what MixMusic connects
  through, without user names or passwords. Before 0.5.9, MixMusic
  connected directly, so on a network that lets traffic out only through
  a proxy, songs were found but didn't play.
- **On macOS and Linux**, only the environment's proxy is used, not the
  system's proxy settings.
- **Never** for loopback addresses, such as `127.0.0.1`, which always
  connect directly.

MixMusic has no proxy setting of its own.

## "Sign in to confirm you're not a bot"

YouTube sometimes answers a request with "Sign in to confirm you're not a
bot", which it asks of a visitor it doesn't trust.

- **Through the player API**, MixMusic drops the visitor ID YouTube
  doesn't trust, takes the new one YouTube's answer carries, or asks for a
  new one, and asks once more. Refused that way for three tracks in a
  row, it leaves the player API to yt-dlp for 30 minutes, as above.
- **Through yt-dlp**, the track says "YouTube asks to confirm this is not
  a bot; try again in a while". When yt-dlp hears "try again later", the
  track says "YouTube is turning down requests from this network for
  now".
- **What to do.** Wait a while and play the track again. Signing in to
  YouTube Music doesn't change it, as MixMusic asks for streams signed
  out. An old yt-dlp fails more often as YouTube changes: Settings ›
  Playback › Tools shows whether yours is up to date.
  [Troubleshooting](troubleshooting.md) has more.
