# The player

- **`internal/player`** plays the queue: one goroutine owns the queue and
  what plays, and the network calls run beside it. It plays YouTube Music,
  and Spotify tracks matched to their recording, through the engine, and
  Spotify tracks on a Spotify device: a Spotify Connect device, or MixMusic
  itself, as below. It shuffles, repeats, crossfades, starts radios when the
  queue runs out, or at once after a suggested song (`PlayWithRadio`),
  passes over the songs the user dislikes but those they
  pick, counts plays and keeps up to 500 tracks of the queue,
  around the one playing, in `queue.json`, as it changes and as MixMusic
  quits. A track that can't play for want of the network gives way at once
  to the next that plays without it, a download or a track the offline
  cache holds, as the app tells; once none is left, it waits for the
  network where it was, and plays on once it answers. The player pauses as
  the computer sleeps.
  As it moves on by itself it passes over, on one path
  (`skip.go`), the entries the settings skip, but those the user picks:
  songs the user dislikes, and songs YouTube labels as made with AI
  (`labels.go`). A track whose label the app doesn't know is checked
  before it's announced, beside the search for its stream, and the next
  track before it's preloaded or crossfaded into.
- **`internal/online`** tells the player whether the network answers: a
  check of YouTube's and Spotify's addresses over HTTPS, shared by its
  callers, which audio coming from YouTube answers too, and a wait for the
  network, with checks further apart the longer it lasts. The system's
  word on the network, through MyGo, only has it check at once.
