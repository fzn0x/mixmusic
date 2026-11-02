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
  those they saved by the menus YouTube sends with them. It lists the
  Google accounts of the session and their brand channels, from YouTube's
  website, and uses a brand channel by its page ID: one whose page ID the
  list doesn't tell is listed as `Unswitchable`, as without it the account
  itself would be used. Once the app switches, it checks that YouTube Music
  answers as the channel chosen, else goes back. It also reads
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
- **`internal/lastfm`** signs in to Last.fm with the user's API account and
  scrobbles by Last.fm's rules, keeping what fails offline for later.
- **`internal/discord`** shows what plays on the user's Discord profile, over
  Discord's local pipe or socket.
