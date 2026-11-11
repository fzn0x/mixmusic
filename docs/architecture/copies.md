# Copies of one song

The music on this computer often holds a song more than once, as an MP3
and a FLAC of it, or a single and its album's copy. Those files are one
song, which shows once and plays its best copy (#340).

- **Which files are copies** (`internal/localfiles/copies.go`). `group`
  works the songs out of the entries of the index and what was kept of
  them. Files of the same match key, their titles and their artists as
  `fold` reads them, the artists in any order, whose lengths are within
  three seconds of the shortest, are copies; a file without a title or
  an artist of its own, as a voice memo, has no key. Those the user
  linked by hand are copies of their song whatever their tags say, and
  those the user took out of one (`Apart`) are copies only by hand.
- **Which song a group of copies is.** A song with copies has an ID of
  its own, `file:song:<key>`, whose key is its best copy's path in
  base64 when it's made, so that the same files make a song of the same
  ID again should its links be lost; `FileOf` plays the file the ID
  names then. A group is the song whose copies it holds most of, those
  linked by hand counting most, then those of the song's key; else, of a
  song that lost all its files, as when they moved, the one of its key
  and length; else, of two files or more, a new song. A song whose files
  all went to another's group became that one. One without files is
  kept, with its key, for them to come back.
- **The best copy** (`compareCopies`): lossless before lossy, then the
  higher bitrate, of its file's size over its length, then the higher
  sample rate. Until `Index.Inspect`'s function, `audio.Inspect` in the
  app, read a copy's audio, its extension tells its codec; what it read
  is kept by the copy's size and time of change.
- **Kept in the data folder** (`internal/localfiles/songs.go`), in
  `local-copies.json`, beside the index, never in the files: the songs,
  their copies best first, those linked by hand, their keys and lengths,
  the files taken apart, and what each copy's audio is. A scan, an
  update of the watch and a reread group them again (`relink`), as do
  `Link` and `Unlink`. The views show a song once, as its first copy,
  under its ID, in the tracks, the search, which reads every copy's
  album, and an artist's songs; an album shows its own files.
- **What follows the song** (`internal/app/copies.go`). The liked and
  disliked songs, the mixes and the history keep the song's ID: once in
  each run, and as the songs change, `OnLinked` tells the app which
  tracks, each copy's file and each song merged into another, are a
  song's now, and `library.Retarget` gives them the song's track, once
  in a list where two were, keeping when each played. What the user does
  with a copy, as one an album shows or one played alone, is done with
  its song: `asSong` in Collection's likes, dislikes, plays and mixes,
  and `songID` in `IsLiked` and `IsDisliked`; `PlayCounts` counts a
  song's plays for each of its copies. A copy unlinked leaves them to its
  song.
- **What plays.** `prepareFile` plays a song's best copy whose file is
  there, and goes on to the next when one is found gone. Track info,
  Show in Explorer, Edit info… and Move to album… of a song use the copy
  that plays (`FileOf`, `fileID`); Edit info… writes to every copy
  (`copyIDs`), so that they stay one song.
- **The windows** (`copies.go`): Play this copy and "2 copies…" in a
  song's menu, beside its other local items, Link a copy… of a file
  without copies, the dialog of the copies side by side, with Play,
  Unlink and a search of the songs to link, and the copies in Track info
  (`trackCopies`).
