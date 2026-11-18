# The player

- **`internal/player`** plays the queue: one goroutine owns the queue and
  what plays, and the network calls run beside it. It plays YouTube Music,
  and Spotify tracks matched to their recording, through the engine, and
  Spotify tracks on a Spotify device: a Spotify Connect device, or MixMusic
  itself, as below. It shuffles, repeats, crossfades, starts radios when the
  queue runs out, or at once after a suggested song (`PlayWithRadio`, `PlayRadio`),
  passes over the songs the user dislikes but those they
  pick, counts plays and keeps up to 500 tracks of the queue,
  around the one playing, in `queue.json`, as it changes and as MixMusic
  quits. A track that can't play for want of the network gives way at once
  to the next that plays without it, a download or a track the offline
  cache holds, as the app tells; once none is left, it waits for the
  network where it was, and plays on once it answers. A track that plays
  without the network, whose audio fails, doesn't wait for it: it is tried
  once more, at once, and then fails with why. The player pauses as the
  computer sleeps.
  As it moves on by itself it passes over, on one path
  (`skip.go`), the entries the settings skip, but those the user picks:
  songs the user dislikes, and songs YouTube labels as made with AI
  (`labels.go`); and explicit songs while the settings hide them, those
  the user picks too ([explicit.md](explicit.md)). A track whose label the app doesn't know is checked
  before it's announced, beside the search for its stream, and the next
  track before it's preloaded or crossfaded into.
- **Parts of videos** (`parts.go`, #548): while Skip the parts
  SponsorBlock marks is on, the player asks the app for the parts of each
  stream (`Marked`) as it is found, the next track's as it is prepared,
  and tells the engine those of the kinds ticked before it loads,
  preloads or crossfades into it (`Skipper`, `audio.Engine.SetSkips`):
  playback never waits for them, and those that come late apply from
  then on. `State.Parts` and `State.Highlight` are the current track's,
  fitted to its length as the engine fits them, for the seek bars. Each
  part the engine passed over is told (`Notice.Undo` goes back to it, which
  then plays), but for one that ended the track. Plays count, and
  Previous's 3 seconds go, by what played without the parts, and half the
  length without them; the app hears how long they last (`Now.Skipped`)
  for Last.fm. A crossfade takes the start of a part that reaches the end
  as the track's end. Nothing is asked for while it's off, nor of a live
  stream, a Spotify device's track, or a cast device's, which plays them.
- **On a Spotify device** (`connect.go`), the player sends commands and
  asks what the device plays, and waits for its reports, which lag the
  commands, to settle. A track ends when the device stops on it, goes back
  to its start, plays another track, or tells it plays on well past the
  track's end: MixMusic's own device, given a track with nothing after it,
  may stay on it once it ended, its position counting on (#373). The time
  shown never passes the track's end. Leaving the device, the player
  pauses the track it follows there. A Spotify Connect device that plays
  something else is left to whoever plays it. MixMusic's own device,
  whose sound comes out of MixMusic, is hushed instead once the player sent
  it a play (`sounds`), whatever it plays: a track lost there, one whose
  play failed after it took it, or one it went on to by itself past the
  queue's (#745). A device MixMusic sent nothing to is told nothing.
- **The sleep timer** (`sleep.go`, #594): a timer of minutes runs on the
  monotonic clock, and its fade starts so that half a second of silence
  comes before the pause (`checkSleep`). A timer that ends with a track
  counts the tracks that end where the queue moves on after one
  (`advance`, `sleepEnded`), and as one fades into the next
  (`fadingInto`); the last plays to its end, with no crossfade
  (`sleepLast`), its fade in its last seconds (`sleepProgress`), and
  Spotify devices get no tracks ahead meanwhile (`aheadOnDevice`). At
  the end of the queue it stops before the radio's entries, unless the
  radio plays already, and doesn't go round. It pauses as `Sleeping`
  does, a Spotify Connect device elsewhere too; then the next entry waits
  at its start, held paused. The engine fades (`Fader`); a Spotify device
  or a cast output gets its volume stepped once a second instead, put
  back a moment after the pause, or at the next play. Bedtime starts a
  timer of the length the settings keep as playback starts, rather than
  plays on, inside a window of `SleepTimer.Window`; a timer cancelled
  inside one holds it off until it ends. `State.Sleep` and `SleepLeft`
  tell the windows what runs; the app keeps the length the user chose
  last (`KeepSleepMinutes`), and runs the tray's submenu, the global
  shortcuts' commands and the assistants' `sleep_timer` tool
  (`internal/app/sleeptimer.go`, `assistantsleep.go`).
- **Lists that play while they're read** (`queue_list.go`, #543):
  `PlayList` plays the first part of a list as `PlayAll` does, and
  returns a `List`, whose `Add` adds each part after it as it comes: in
  the queue's own order after the list's last entry and the current one,
  and shuffled at random among the entries after the current one, before
  the radio's (`scatter`). Once none of the list's entries is left in the
  queue, as when another queue replaced it, `Add` adds nothing and says
  so, which stops the reading.
- **Crossfades** (`crossfadeTo`): the next track fades in on the
  engine's other deck, and the crossfade ends once the engine's did
  (`Stream.Fading`), which a next track whose audio is late, or a pause,
  holds past its seconds. Until then the player starts no other, nor
  loads 5.1 audio, which would cut the track fading out off (#549).
  While the settings keep albums gapless (`gapless.go`, #601), the next
  song of the same album, right after the current one in the queue's own
  order, plays without one; a Spotify track only when both recordings
  are songs, as the next one's preparation tells (`preparedNext`).
- **The next track on the next sample** (`follow.go`, #600): the next
  entry's audio the player preloads (`holdAhead`) is armed to follow the
  current track on the engine's other deck, at its kind's speed, and at
  the current track's loudness for a song of the same album. After each
  task, the player tells the engine again when that changed (`arm`): the
  next entry must be the one the queue plays next, the engine must play
  the current stream here, with no crossfade under way, and the stream
  may refuse (`Joiner`), as a room's host's does while guests listen. As
  the engine follows (`Followed`), the next entry becomes current as after
  a crossfade (`becomeCurrent`); a Followed the state no longer lets moves
  on as an end does, and one after a load changes nothing.
- **5.1 surround** (`surroundready.go`): a stream plays its 5.1 audio
  while the settings ask for it, once the app tells it plays at once
  (`Surrounds.SurroundReady`). Until then the engine plays its stereo
  audio, while the app makes the 5.1 audio ready
  (`Surrounds.PrepareSurround`): yt-dlp finding it, and YouTube serving
  it. Once ready, the engine loads it from where the track is then, as
  pressing 5.1 does, after a crossfade under way (#457). Another entry,
  or 5.1 turned off, stops the preparation, and what it finds plays no
  more; 5.1 audio that can't be had leaves the entry in stereo, as 5.1
  audio that fails does (`stereoFor`).
- **Failures** (`failures.go`): the player keeps the last 10 tracks that
  couldn't play, until MixMusic quits (`Failures`, `FailureOf`), and the
  notice of each names its own (`Notice.Failure`). Each holds the steps
  tried of the track since it loaded, in turn, with why each failed: the
  Spotify device it fell back from (`fellBack`), the 5.1 audio it gave
  up, the stream and its tries again, one step for each run of the same
  error, and the error that failed it, named after how the track was to
  play (`way`): the file, a download or the offline cache, the stream,
  or finding it. An error that tells its own steps, as the resolver's
  does when yt-dlp failed after YouTube's player API (`stream.bothFailed`),
  gives them in its place: `await` keeps the error of the answer the
  player takes in, for `tried` to read.
- **`internal/online`** tells the player whether the network answers: a
  check of YouTube's and Spotify's addresses over HTTPS, shared by its
  callers, which audio coming from YouTube answers too, and a wait for the
  network, with checks further apart the longer it lasts. The system's
  word on the network, through MyGo, only has it check at once.
