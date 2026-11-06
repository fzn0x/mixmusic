# Music on this computer
<!-- docs: group=using order=25 -->

MixMusic plays the music files on your computer beside Spotify and YouTube Music: in one library, one search and one queue. It plays them from their files, and uploads nothing.

Your own recordings, the albums you bought, a folder of live sets: add the
folders they're in, or open the files, and they play as any track does,
with the equalizer, crossfade, speed and the visualizer of
[the audio engine](audio-engine.md).

## Add a folder

Settings › Playback › Music on this computer, or the add a folder button of
Library › On this computer, opens your system's folder picker. MixMusic
reads the folder and the folders in it, and keeps what it finds:

- **Folders** lists each folder with how many tracks it holds, and how
  many files it skipped: files of kinds it can't play, or broken.
- **Remove** takes a folder out of the library. Its files stay where they
  are, and its tracks stay in your mixes, greyed.
- **Rescan** reads the folders again now. MixMusic also reads them as it
  starts, quietly, and when you add or remove one.

A scan reads only what changed: a file new to it, or one whose size or
time of change differs from the last scan. A library of thousands of
files is read whole once, then in moments. MixMusic doesn't watch the
folders as you work in them: rescan, or start MixMusic again, to see what
you added. Hidden folders, whose names start with a dot, are passed
over.

## Open files without a folder

Open music files…, in the [command palette](settings.md#the-command-palette)
or with Ctrl+O (⌘O on macOS), and open files in Library › On this
computer, choose files to play now, as a queue. Drop files on the window,
or a folder, and they play the same way, in the order of their names.
MixMusic keeps none of them in the library: they play as long as their
files are where they were, from the queue, your mixes or your liked
songs.

## Library › On this computer

The [Library page](library.md#the-library-page)'s On this computer pivot
shows how many tracks your folders hold, and:

- **Songs:** every track, with Play, Shuffle and open files, a box that
  searches the list, and a Sort menu: the list's own order, by album and
  track number, Title, Artist or Length.
- **Albums:** the albums your files' tags name, with their covers. An
  album is grouped by its title and its album artist, or, without an
  album artist, by the folder its files are in, as an album's files
  usually are; one of several artists without an album artist is by
  Various artists. Its page plays it.
- **Artists:** the artists of your files, whose page lists their songs
  and albums.

From N folders, beside the sections, tells where they come from, and
manage opens Settings.

## Search

The search finds the music on this computer too, in its own section, On
this computer, above the results of the services, with both services
chosen. It finds tracks by their titles, artists and albums, regardless
of case and accents, and shows even without the network.

## What plays

| Kind | Files | Notes |
|---|---|---|
| MP3 | `.mp3` | MPEG-1, 2 and 2.5 Layer III; without the silence LAME's tag tells of at their start and end, so that an album's tracks follow each other without gaps |
| FLAC | `.flac` | up to 8 channels and 32 bits |
| WAV | `.wav` | PCM of 8 to 32 bits, or floats of 32 or 64 |
| AAC | `.m4a`, `.mp4` | AAC-LC; HE-AAC and Apple Lossless (ALAC) aren't played |
| Opus | `.opus`, `.ogg` | in Ogg, mono or stereo |
| Vorbis | `.ogg`, `.oga` | in Ogg |

Files of more than two channels play mixed down to stereo. Files of
other kinds, as `.wma` or `.aac` alone, aren't read. A file MixMusic can't
play counts as skipped in Settings, and never stops a scan.

## Tags and covers

MixMusic reads the title, artists, album, album artists, track and disc
numbers and year your files' tags tell: ID3v2 and ID3v1 of MP3, MP4's
tags, the Vorbis comments of FLAC, Opus and Vorbis, and WAV's INFO list.

- **A file without tags** shows by its file's name, by Unknown artist.
- **Covers:** the cover a file holds, the front cover when it holds
  several, else an image beside it named cover, folder, front or album
  (`.jpg`, `.png` or `.webp`). MixMusic keeps a copy of each in its cache
  folder, never beside your music, and never changes your files.

## When a file is gone

A track whose file moved, was deleted, or is on a drive that isn't
plugged in shows greyed, with File not found in place of its artists.
Playing it moves on to the next track, with a short notice.

- **A rescan** drops the files that are gone from the library. Those of a
  folder that can't be read at all, as on a drive unplugged, stay, greyed,
  until it's back.
- **Your mixes and liked songs** keep the track, greyed, and it plays
  again once its file is back where it was.

## What they do and don't do

Tracks on this computer join the queue, your [mixes](library.md#mixes)
and your liked songs, and you can dislike them, as any track. What needs
a service doesn't apply to them:

- No radio starts from them, Autoplay doesn't carry on after one, and no
  related tracks show under it.
- No YouTube Music match is looked for, and they don't go to Spotify or to
  YouTube Music playlists.
- They don't download: they're on your computer already.
- They don't play on [cast devices](casting.md), which can't reach your
  files.
- Show in Explorer, in a track's menu (Show in Finder on macOS), opens the
  folder of its file.
- The system's [media controls](desktop.md#media-keys-and-the-systems-media-controls)
  show it with its cover, as any track.

## Privacy

Nothing of your music files leaves your computer. MixMusic keeps an index
of what your folders hold, their paths and tags, in `local-files.json` in
your [data folder](data-and-privacy.md#the-data-folder), and their covers
in the cache folder's `local-art`.

A track on this computer is treated as any other by the features that send
what plays, as their settings say:

- **[Last.fm](accounts.md)**, once you connect it, scrobbles its title,
  artists, album and length. A track of no artist isn't scrobbled.
- **Discord**, unless you turn it off in Settings › Desktop, shows its
  title and artists, without a cover.
- **[Lyrics](lyrics.md)**, when you show them, are looked for on Unison
  and LRCLIB by its title, artists, album and length.

A file that names no artist, as a voice memo, shows by its file's name,
which may be anything: Last.fm and the lyrics hear nothing of it.

[Your data and privacy](data-and-privacy.md) lists every feature's.
