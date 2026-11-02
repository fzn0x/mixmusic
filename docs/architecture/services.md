# The services

- **`internal/media`** defines what every service shares: tracks, albums,
  artists, playlists, shelves. IDs carry their service: `yt:<video id>`,
  `sp:<track id>`, `mx:<mix id>`.
- **`internal/youtube`** reads YouTube Music through the API its website uses:
  search, suggestions, the home and explore pages, charts, moods and genres,
  albums, artists, playlists, related tracks, radios and plain lyrics.
  Signed in with the cookies of the user's session, which it signs its
  requests with as the website does, it reads and changes their library:
  liked songs, playlists, albums, subscriptions, likes, dislikes and
  saves, and it deletes the playlists the user made, which it tells from
  those they saved by the menus YouTube sends with them. It also reads
  whether YouTube labels a video as made with AI, from the answer of
  YouTube's own website, asked signed out with a field mask
  (`MadeWithAI`): nothing of YouTube Music carries the label. Its parsers
  are tested against recorded responses in `testdata/`, and against the
  live service with `MIXMUSIC_LIVE=1 go test ./internal/youtube -run Live`
  (`MIXMUSIC_YT_COOKIES` adds a signed-in account, and
  `MIXMUSIC_YT_WRITE=1` lets it make and delete a playlist there, and
  dislike a song and take the dislike back).
- **`internal/spotify`** signs in with PKCE on a loopback callback, refreshes
  tokens, reads the Web API (search, albums, artists, playlists, the
  library), and controls Spotify Connect devices.
- **`internal/match`** finds a Spotify track's recording on YouTube Music, by
  title, artists and duration, for playback without a Spotify device. It
  also finds the song of a music video, by its title and artists, which
  the video's radio starts from: YouTube Music makes the radio of a video
  out of videos.
- **`lyrics`** finds synced lyrics on Unison and LRCLIB, reads TTML and LRC,
  and translates and romanizes lyrics on Unison.
- **`internal/romaji`** writes Japanese lyrics in Hepburn romaji on the
  computer, as Unison gives none for Japanese: kana from a table, and kanji
  by the readings that kagome's tokenizer finds in its IPA dictionary,
  word by word. The dictionary isn't built in, which would grow the app by
  11.7 MB: `Dictionary` downloads it from kagome-dict's module on GitHub
  the first time romaji is on for Japanese lyrics, checks it against the
  SHA-256 written in the package, keeps it in the cache's `romaji/`, and
  checks it again each time it loads. A download that fails or isn't the
  dictionary goes, and is tried again after 30 seconds, twice as long
  after each failure in a row, up to 10 minutes. Until it is there, the
  lines' kana are in romaji and their kanji stay as they are. Loaded, the
  dictionary takes some 89 MB of memory: it loads in about 0.4 s when
  lines need it, and goes 5 minutes after they last did. The windows make
  each track's romaji once, with its lyrics. Its tests read the
  dictionary from the Go module cache, where
  `go mod download github.com/ikawaha/kagome-dict/ipa@v1.2.6` puts it.
- **`internal/lastfm`** signs in to Last.fm with the user's API account and
  scrobbles by Last.fm's rules, keeping what fails offline for later.
- **`internal/discord`** shows what plays on the user's Discord profile, over
  Discord's local pipe or socket.
