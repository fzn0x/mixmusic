# Podcasts

Shows and their episodes, on YouTube Music and Spotify: found by search,
opened on a page of their own, followed in the Library, and played where
the user left them.

- **`internal/media`.** A show is a `Podcast`, with a page of its episodes
  and `More`, which reads the next. An episode is a `Track` with an
  `Episode`: its show, the day it came out, and where the user left it on
  Spotify. Its artists name its show, so that the player bar, the tray,
  notifications and the media controls name it too. Shelves hold shows as
  items of `ItemPodcast`.
- **YouTube Music (`internal/youtube/podcast.go`).** Search has a
  podcasts and an episodes filter. A show is the browse page `MPSP` and
  the playlist of its episodes: its head is an album's, and its episodes
  are `musicMultiRowListItemRenderer` rows, which play a video of the type
  `MUSIC_VIDEO_TYPE_PODCAST_EPISODE`. Their days ("8h ago", "Oct 26",
  "Dec 29, 2025") and lengths ("2 hr 18 min") are read in English, as
  MixMusic always asks. Pages of about a hundred more come from browse's
  continuation. An episode plays as any video, through the resolver. The
  shows the user saved are `FEmusic_library_non_music_audio_list`, and
  saving one likes its playlist, as for a playlist.
- **Spotify (`internal/spotify/podcast.go`).** Search asks for `show` and
  `episode`, 10 of each, the most since February 2026. A show is
  `GET /shows/{id}`, its pages of 50 `GET /shows/{id}/episodes` by offset,
  the shows saved `GET /me/shows`, and saving is `/me/library` with
  `spotify:show:` URIs. `resume_point` comes with the
  `user-read-playback-position` scope, which sign-ins made before it lack.
  Shows have no publisher for apps in development mode. Episodes play only
  from Spotify, as `spotify:episode:` URIs, on Spotify Connect or the Web
  Playback SDK: `PrepareStream` refuses them, as YouTube Music has no
  recording of them. `GET /me/player` asks with `additional_types`, or
  Spotify would tell no item while an episode plays, and the player would
  take the device for playing something else.
- **Where episodes were left (`internal/library/positions.go`).** The
  player's reports, each second, keep the place of the episode that
  plays, in memory, written to `positions.json` once they rest 10 seconds
  and as MixMusic quits, for the latest 500 episodes. Under 15 seconds in,
  nothing is kept; within 2 minutes of the end, or 3% of a short episode,
  it counts as played. An episode that becomes current plays from 5
  seconds before its place, through the player's optional `Resumer`, or
  from its service's while MixMusic has none: Spotify's `resume_point`,
  or YouTube Music's, as below. Moving within the current episode, as
  Previous does, never resumes.
- **Where YouTube Music saw them (`internal/youtube/progress.go`,
  `internal/library/resume.go`, `internal/app/youtube_watchtime.go`).**
  Signed in, a row of a show's episodes tells how far the user got, as
  `playbackProgressPercentage` of its length, and "Played" in its
  `playbackProgressText` once they finished it: the episode keeps it as
  `ResumeMs` and `Played`, with `ToldAt`, when it was read. That place
  outranks MixMusic's own when it is newer: it changed since MixMusic kept
  its own, whose track keeps the place YouTube Music told then, and was
  read after. When MixMusic never saw it, the place further along wins.
  One within 30 seconds and a percent of MixMusic's own keeps MixMusic's,
  exact to the second. Once an episode's play counts, the play goes to
  the history with a `youtube.Watch`, which reports the listening to the
  player response's `videostatsWatchtimeUrl` as YouTube's player does,
  with the same playback nonce: what was heard since it started, then on
  the schedule the response tells, 10, 20 and 30 seconds after it started
  and every 40 after, a few seconds early at random as the player has
  them; as it pauses and plays again; and a last time, `final=1`, as
  another track plays or none does. The history setting governs it as it
  does the plays: off, or signed out, nothing goes.
- **The player.** Episodes never crossfade, the next episode's audio
  isn't fetched ahead, and the queue that ends on an episode stops rather
  than start a radio. Episodes don't scrobble, and the stats leave them
  out: the history keeps them.
- **Speed.** Episodes play at a speed of their own, `podcastSpeed`, and
  songs at `songSpeed`: the player's `speedOf` picks by the track that
  becomes current. The speed button by the player, the mini player's
  menu, the command palette and the rate the media controls set change
  the speed of the kind that plays (`speed.go`). Spotify's players, and
  live streams, keep their own speed: the button is dimmed for them.
- **Details (`episodedetails.go`).** Where a song shows its lyrics, in
  the side panel, on the now playing page and in the notch, an episode
  shows its details, which `Catalog.EpisodeDetails` asks for the first
  time they show, then keeps: `media.EpisodeDetails`, its show, views,
  day and length, and its description line by line, in runs of plain
  text, links and times. YouTube Music's come from the page of the Details
  tab its player has where a song has Lyrics, browsed by `MPPE` and the
  video ID, whose runs mark the links and the times; the episode's own
  page, `MPED`, has the same description, and the player's answer none.
  Spotify's come from `GET /episodes/{id}`, whose plain `description`
  `media.ParseDescription` reads, finding web addresses and times as
  12:34 or 1:02:03. Lines that start or end with a time are chapters
  (`media.ChaptersOf`): the panel lights the one that plays, and the notch
  shows it. Links open only web pages, http and https, and times past the
  episode's length don't seek.
- **Follows.** `library.json` keeps the shows the user follows, without
  their episodes, signed in or not; following one saves it on its service
  too, where the user signed in.
- **The windows (`podcasts.go`).** `/podcast/<id>` is a show's page, the
  search's podcasts pivot asks only once it shows, and Library › Podcasts
  lists the episodes in progress, the shows followed, and those saved on
  each service. Episode rows are built only near the view.
