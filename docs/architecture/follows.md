# Following artists

The artists the user follows, on their services or in MixMusic, and what
they put out lately, for Home and Library › Artists.

- **Follows.** Signed in to an artist's service, Follow follows them there,
  as before: `Collection.SetFollowing` subscribes on YouTube Music, or
  saves the artist to the Spotify library. Signed out,
  `Collection.SetArtistFollowed` keeps them in `library.json`
  (`library.FollowedArtist`: their ID, name, artwork and link), as the
  shows followed are kept. Nothing moves one to the other: signing in
  later follows nothing on the service. The page says Following for
  either (`followsArtist` in `follow.go`): its own `Following`, signed in,
  or else the list of the artists followed there, as read. Stopping stops
  in both, where the user follows. The service's list, as fetched, takes
  the change in place (`followedOn`), as the podcasts' does: the service
  may answer a read as it was a moment before. Tests replace
  `followOnService`.
- **Releases.** `Catalog.NewReleases` gathers the artists followed
  (`followedArtists`): MixMusic's, then Spotify's (`/me/following`) and
  YouTube Music's (`FEmusic_library_corpus_artists`) where the user signed
  in, each once. A look (`releaseBook.look`) asks for the releases of the
  artists due: never asked, read more than `releasesFresh` (12 hours) ago,
  or failed more than `releasesRetry` (an hour) ago; at most
  `releasesPerLook` (30) of them, those never asked first, then those
  asked longest ago, `releasesAtOnce` (3) at a time. YouTube Music's come
  from the artist's page (`youtube.Client.Artist`): its Albums and Singles
  & EPs, ten of each, which tell only their years. Spotify's come from
  `/artists/{id}/albums`, the albums then the singles, `limit` 10, the
  most it takes since February 2026, with `release_date` as far as
  `release_date_precision` goes (`spotify.Client.ArtistReleases`). Neither
  gives a feed of the releases of the artists followed: YouTube Music's
  New releases are everyone's, and Spotify removed `/browse/new-releases`
  in February 2026. An artist that fails keeps what was read before; one
  the look's time ran out on isn't counted as failed.
- **The file.** `releases.json`, in the cache folder, keeps for each
  artist when it was asked and when it answered, and their releases of
  the last year. A look forgets the artists no longer followed, but those
  of a service whose list didn't read; one the user stops following from
  MixMusic goes at once (`releaseBook.forget`).
- **New** (`newReleases`): `media.Release.Span` reads `Released`, a day, a
  month or a year, as the first and the last day the release may have
  come out on. A release is new while its last day is within
  `releasesNewFor` (8 weeks) and its first day has come: one known by its
  year alone is new all that year, and 8 weeks into the next. At most
  `releasesPerArtist` (4) of an artist show, their newest, and
  `releasesShown` (30) in all, each album once. They come by their first
  days, the newest first, then by their places among their artists' new
  releases, then by their artists' names, so that the list known before
  a look and the one after it come in the same order.
- **The windows (`releases.go`).** Home, after the user's own shelves,
  and Library › Artists, first, show `releasesShelf`. Until this run's
  look comes (the `fetch` of `"releases"`), it shows `KnownReleases`,
  what the file holds, without asking anything; a refresh asks again.
  Following asks again; stopping drops the artist's releases from both at
  once. A release's tile is its album's, with the day it came out, or
  its year, where an album's year goes.
