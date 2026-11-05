# How Spotify tracks play
<!-- docs: group=inside order=40 -->

MixMusic plays Spotify tracks three ways: in MixMusic through Spotify's own player, on Spotify Connect, or from their recording on YouTube Music.

You choose the way in Settings › Playback › Spotify tracks. Each needs a
Spotify sign-in, made with a Spotify app of your own:
[Spotify](spotify.md) walks you through it, and tells what to do when
Spotify refuses the sign-in. For the code, read
[the player's part of the architecture](architecture/player.md) and
[its web views](architecture/webviews.md).

## The three ways at a glance

| | In MixMusic, from Spotify | On Spotify Connect | In MixMusic, from YouTube Music |
|---|---|---|---|
| Who plays the audio | Spotify's player, inside MixMusic | a Spotify app or device | MixMusic's own engine |
| The audio | Spotify's, at Spotify's quality | Spotify's, at Spotify's quality | the track's recording on YouTube Music |
| Needs Premium | yes | yes | no |
| Systems | Windows | every system, with a device that has Spotify open | every system |
| Equalizer, crossfade, speed, Even loudness | no | no | yes |
| Visualizer | on Windows | on Windows, for the Spotify app on this computer | yes |
| Cast, 5.1 | no | no | yes |
| Podcast episodes | yes | yes | no |
| Default | on Windows | on macOS and Linux | no |
| In settings.json | `"spotifyPlayback": "here"` | `"connect"` | `"match"` |

When the way you chose can't play a track, as when no device answers or
Spotify can't play in MixMusic, the track plays from YouTube Music
instead, and MixMusic says why once. A Spotify track always downloads as
its recording on YouTube Music, whatever the way, and while you're
offline it plays from its download or the offline cache that way.

## In MixMusic, from YouTube Music

MixMusic finds the track's recording on YouTube Music, by its title, its
artists and its length, and plays it in its own engine, with everything
[the audio engine](audio-engine.md) does. The listener needs no Premium.
This way is called Match in `settings.json` and in the code.

How the match is made:

1. **The search.** MixMusic searches YouTube Music's songs for the title
   and the first artist, and when none is close enough, its videos.
2. **The score.** Each result scores from 0 to 1:
   - 45% for the title: the share of the words of the shorter title that
     the other has, once both are in lower case, without accents and
     punctuation, and without what names a release rather than the song,
     as "(Remastered 2011)", "- Radio Edit" or "[Official Video]";
   - 35% for the artists: one of the track's artists is the result's, or
     its title names them, as uploads called "Artist - Song" do;
   - 20% for the length: within 2 seconds counts whole, within 5 seconds
     0.8, within 10 seconds 0.5, within 20 seconds 0.2, and beyond that
     nothing; half when either length isn't known;
   - and 0.05 more for a song over a video, as songs are the released
     recordings.
3. **The choice.** The best result plays when it scores 0.6 or more.
   Otherwise YouTube Music has no recording close enough, and the track
   can't play this way.
4. **The memory.** MixMusic keeps each match in `matches.json` in its
   cache folder, so a track is searched for once.

When a match is wrong, choose **Find another YouTube Music match** in the
track's menu: the next play searches again, and a track you downloaded
downloads again as its new match.

## In MixMusic, from Spotify

MixMusic plays the track itself, with Spotify's own player, at Spotify's
quality, with no Spotify app open. It needs Spotify Premium, and Windows.

- **How.** Spotify's Web Playback SDK, which MixMusic loads from
  `sdk.scdn.co`, plays Spotify's protected audio in a hidden window of a
  page of MixMusic's own. Only WebView2, the web view of Windows, decrypts
  that audio: WebKitGTK on Linux doesn't, and macOS's web view isn't known
  to, so this way is greyed out there.
- **A device of its own.** The page makes MixMusic a Spotify Connect
  device, named MixMusic, which shows among your Spotify devices while it
  plays. MixMusic plays tracks on it through the Web API, as on any
  device, and follows it by asking the page, which tells at once when
  its track changes. The window opens at the first Spotify track that
  plays this way, and closes when you sign in again or change the
  setting.
- **Your sign-in.** Playing Spotify in MixMusic needs a permission that
  sign-ins made before 0.5.0 lack: Settings › Playback asks you to sign in
  again, and until you do, Spotify tracks play from YouTube Music.
- **The sound.** Spotify's player plays at your volume, on the same curve
  as MixMusic's own. The equalizer, crossfade, Even loudness and speed
  don't reach it: Spotify keeps its own loudness and speed. On Windows 10
  build 20348 and later, the visualizer hears it through Windows' process
  loopback.

### What 0.5.9 changed

- **Songs follow one another without a gap.** MixMusic gives Spotify the
  track with up to 20 of the Spotify tracks the queue moves on to after
  it, turns the device's shuffle and repeat off so that it plays them in
  the queue's order, and Spotify goes on to each by itself. Between 1 and
  2 seconds of silence came between them before (#187).
- **A YouTube Music track after a Spotify one starts at once**, as
  MixMusic hears the moment Spotify's song ends. When the queue changed
  meanwhile, as when you chose Play next, what Spotify went on to is
  silenced, and the song you chose plays (#187).
- **A seek into a song's last seconds** no longer leaves a gap or skips a
  song of the queue. Spotify stops on the song a moment while it loads the
  next: MixMusic now gives it up to 2 seconds to go on before it plays the
  next track itself (#273).
- **A track plays once you hear it.** Spotify says a track plays about
  0.3 seconds before its sound starts. Until it sounds, or for 10 seconds
  at most, the track loads: it can't be paused or moved, so a click can't
  start its load over (#184).

## On Spotify Connect

A Spotify device plays the track: the Spotify app on this computer, your
phone, a speaker or a TV. It needs Spotify Premium, and a device with
Spotify open.

- **Which device.** Settings › Playback › Spotify tracks › Device, or the
  devices button of the player bar, chooses it; by default, the active
  device. Open Spotify on a device for it to show there.
- **How MixMusic follows it.** MixMusic sends the device commands through
  the Web API, asks it what it plays every 3 seconds, and moves the
  progress on each second between. Spotify's reports lag its commands by
  a few seconds, and may name the track by the ID of the copy Spotify
  plays in your country, so MixMusic waits for the device to settle, and
  knows a track by more than its ID.
- **The queue stays in MixMusic**, which tells the device to play the
  next track as one ends.
- **The visualizer** hears the Spotify app on this computer, on Windows;
  other devices it can't hear, and it draws nothing then.
- **Sleep.** As the computer goes to sleep, MixMusic pauses its own
  player, and a device elsewhere plays on.

## Premium and development mode

Spotify keeps apps that it hasn't reviewed in development mode, and every
MixMusic user signs in with such an app, made in two minutes on Spotify's
developer dashboard. Spotify limits these apps, more so since February
2026:

- **The app's owner needs Premium**, or Spotify refuses every sign-in with
  the app. The other accounts need Premium only to play Spotify's own
  audio, in MixMusic or on Spotify Connect: without it, their Spotify
  tracks play from YouTube Music.
- **Five users.** Up to five Spotify accounts, added under Users
  Management in the app's settings, can sign in with it. To let a friend
  in, add them there, or have them make an app of their own.
- **Smaller answers.** A search gives at most ten results of each kind;
  the app sees the tracks of the playlists you own or collaborate on, not
  of playlists others made, which MixMusic tells you; Spotify gives no
  artist's top tracks, which MixMusic finds with a search instead; and
  shows come without their publisher.
- **A quota.** When MixMusic makes too many requests, Spotify asks it to
  wait, and MixMusic says so.

## Podcast episodes

Spotify's podcast episodes play only from Spotify, in MixMusic or on
Spotify Connect: YouTube Music has no recording of them, so the YouTube
Music way doesn't play them, and says so.

- **Where you left them.** MixMusic keeps where you left each episode, and
  starts it 5 seconds before. An episode you never played in MixMusic
  starts where Spotify says you left it, as on your phone. That takes a
  permission sign-ins made before 0.5.9 lack: sign in to Spotify again for
  it.
- **What's different.** Episodes don't crossfade, play at Spotify's own
  speed, and don't scrobble. A search asks Spotify for ten shows and ten
  episodes.
