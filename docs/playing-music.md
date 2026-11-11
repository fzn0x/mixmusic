# Playing music
<!-- docs: group=using order=10 -->

MixMusic plays Spotify and YouTube Music from one search and one queue, with radios, crossfade, an equalizer, speed control and offline playback.

This page covers finding music, the queue, the sound settings and the ways Spotify tracks play. Most of what it names is in [Settings › Playback](settings.md#playback); the box at the top of the Settings page [finds any setting](settings.md#find-a-setting-by-typing) as you type its name.

## Search

The search box at the top of the window, "Search Spotify and YouTube Music", searches both services at once. Ctrl+F (⌘F on macOS) puts the cursor in it. As you type, it suggests searches from YouTube Music; Enter, or a suggestion, opens the results. Below the searches, it suggests songs, albums, artists and playlists from YouTube Music: a song plays at once, with its radio after it when Autoplay is on, and the others open their page. The arrow keys move through both. With nothing typed, the Search page lists your recent searches, which its clear link forgets.

- **The pivots** at the top of the results are All, Songs, Videos, Albums, Artists, Playlists and Podcasts, each with how many it found. All shows the first ten songs, the first five videos, then shelves of artists, albums and playlists.
- **Videos** are YouTube's videos that aren't songs of YouTube Music's catalog, such as DJ sets, live recordings and fans' uploads, and they play as songs do. Only YouTube Music has them.
- **The services:** under the pivots, choose both, Spotify or YouTube Music. With both, the results of the two services take turns, so that each leads.
- **On this computer:** with both services chosen, the [music on this
  computer](local-music.md#search) that matches shows first, in its own
  section, without the network too.
- **Each result** has a mark of its service. YouTube Music's songs say how many times they played, or how many views a video has, as "1.2B plays" or "905M views", so that the original stands out from covers. Spotify gives apps no such count.
- **Without Spotify,** the page says it searches YouTube Music only, with a link to [sign in](spotify.md#sign-in). Spotify gives an app in [development mode](spotify.md#what-spotify-allows-apps), as your Spotify app is, ten results of each kind.

![The search page for YOASOBI: the All, Songs, Videos, Albums, Artists, Playlists and Podcasts pivots with how many each found, the Both, Spotify and YouTube Music choice, a note that it searches YouTube Music only until you sign in to Spotify, and the songs found, each with its plays and the mark of its service](shots/search.webp)

The [command palette](settings.md#the-command-palette) (Ctrl+K) searches too: type anything, and its first item is "Search for" what you typed.

## Playing from anywhere

- **A track's row:** double-click it, or click its cover, to play the list from that track: the album, the playlist or the search results. Doing so on the track that plays pauses it.
- **A track's menu,** from the … button of its row or a right click: Play, Play next, Add to queue, Start radio, Add to liked songs, Dislike, [Add to library](library.md#youtube-music-playlists-albums-and-artists) (YouTube Music, signed in), [Add to mix](library.md#mixes), [Add to YouTube Music playlist](library.md#youtube-music-playlists-albums-and-artists) (signed in), [Download](library.md#downloads), Go to the artist, Go to album, Open in Spotify or YouTube Music, Copy link, and [Share](#sharing). A Spotify track that plays from YouTube Music also has Find another YouTube Music match, for when the recording found is the wrong one.
- **An album's or a playlist's page:** Play and Shuffle, and the More menu: Play next, Add to queue, Start radio, Save as a new mix, Save as a YouTube Music playlist (signed in), Open on its service, Copy link and [Share](#sharing). The tiles of albums, playlists and mixes have Share in their menus too.
- **An artist's page:** Play top songs and [Follow](library.md#artists-you-follow), then Top songs, Albums, Singles & EPs, and Fans also like. Its More menu follows the artist or stops, and starts a radio of the artist.
- **A tile,** on Home, Explore, the search or the library: its play button plays it, and clicking the tile opens its page.
- **Home's Jump back in** plays the track you played last again.
- **[Synced lyrics](lyrics.md#synced-and-plain-lyrics):** click a line to play the song from there.

A track that loads can't be paused or moved until you hear it: the play button shows that it loads, and the keys and media controls wait too. Choosing another track still leaves it.

## Selecting tracks

Select several tracks of a list to act on them all at once:

- **Ctrl-click** a track's row (⌘-click on macOS) to select it, or choose **Select** in its menu. Once a track is selected, the rows show check boxes, and a click on a row, or on its box, selects it or lets it go.
- **Shift-click** a row to select every track from the one you clicked last to it.
- **The bar** above the player bar tells how many are selected, and has select all, Add to queue, Add to mix and Download, which take the tracks in the list's order. Download shows while some of them aren't downloaded yet.
- **Escape**, the bar's close button, or another page or search lets them go.

## Sharing

Share, in the menu of a song, an album, a playlist or a mix, has what to send to a friend. There is no server in it: the links go to the services' own pages, and the images are made on your computer.

- **Copy link** copies the page of the song, album or playlist on its service. A Spotify song that MixMusic has found on YouTube Music, as it does when the song plays or downloads, has both: Copy the link on Spotify and Copy the link on YouTube Music. A mix and a file on this computer have no page, so they have no link to copy.
- **Save image…** saves a square card, for a chat or a feed, as a PNG. **Save image for a story…** saves a tall one, 1080 by 1920 pixels. The system's dialog asks where. The card has the cover, the kind (Song, Album, Single, EP, Playlist or Mix), the title and the artist, in the colours of the theme you use now, in its typeface where MixMusic has it. Titles that don't fit end with an ellipsis.

Letters of Hebrew, Arabic, Chinese, Japanese and Korean come from the fonts your system has. MixMusic doesn't carry them, so a computer without one shows those letters blank.

For lyrics, see [Share lines](lyrics.md#share-lines).

## Track info

**Track info,** in a track's menu, tells what the track is and how it plays:

- where it comes from: Spotify, YouTube Music, a download, or a file on this computer, and of a Spotify track, the recording on YouTube Music that plays for it;
- its format, as Opus in WebM, AAC in MP4 or FLAC, its bitrate, sample rate, bit depth (of FLAC and WAV), channels and length;
- the file it plays from, with its size: a file on this computer, with its tags, its download, or the offline cache's copy. Show in Explorer (Show in Finder on macOS) opens the folder of a file on this computer or of a download;
- of the track playing, what MixMusic's audio engine plays: from where, its 5.1 surround audio when it plays it, and whether it converts it to 48 kHz;
- how loud it plays: YouTube's measure of it, and how far [Even loudness](#even-loudness-volume-levelling-and-the-volume) turns it down.

It tells what MixMusic knows without asking a service: a track that hasn't played yet tells its format once it plays, as MixMusic finds its stream then. A Spotify track that Spotify plays, in MixMusic or on a Connect device, plays at the quality your Spotify account and Spotify's settings choose, and Spotify tells MixMusic nothing of its audio.

## Versions of an album

Spotify and YouTube Music list an album that came out explicit and clean twice, and its editions, as a deluxe or a remastered one, beside it. On an artist's page, in the search and in the library, MixMusic shows one tile for the album, which says how many versions it has, as "2019 · 3 versions".

- **The tile** opens the album on its explicit version, and its play button plays that one. With **Prefer clean versions** on, in [Settings › Playback](settings.md#playback), it shows and opens the clean one.
- **The album's page** has a switch of its versions under its head, named as the titles name them, as Explicit, Clean, Deluxe Edition or Remastered 2009, and Standard for the album as it first came out. The page keeps the version you chose until you open the album from its tile again.
- **On Spotify,** whose lists don't tell which version is explicit, the page reads every version first, and tells by their tracks.

Albums are versions of one when they're of the same service and the same artists, and have the same title once its last part in brackets, or after a dash, is set aside, as long as that part names a version: Clean, Explicit, Edited, Deluxe, Remastered, an anniversary or an edition, with its year. A live album, a re-recording, as "(Taylor's Version)", or remixes are albums of their own. Versions of the same edition are one album only when their years, and their numbers of tracks where the service tells them, agree; and two albums of the same title from different years, as a self-titled album and a live one of that name, stay two tiles. Albums on this computer aren't grouped.

## The queue and Up next

The Queue button by the player, or Ctrl+J (⌘J), shows Up next at the right of the window: the track playing, then what plays after it.

- **Play next,** in a track's or a list's menu, puts it right after the track playing. **Add to queue** puts it after the tracks you added, and before those of the radio.
- **Drag** a track in Up next to move it, and use the × on its row to take it out. Clear takes out every track after the one playing. Double-click a track to play it.
- **Marks** after the artists tell what a track is: "radio" for those a radio added, "skipped" for a song the queue passes over as you dislike it, and "made with AI" for one it passes over as YouTube labels it so. Clicking a skipped track's cover still plays it.
- **In a wide window,** the player bar shows the track that plays next under the one playing, while Up next is closed.
- **After a restart,** the queue comes back on the track that played, where it was, with up to 500 tracks around it.

## Shuffle and repeat

- **Shuffle** (Ctrl+S, ⌘S): the tracks after the one playing play in a random order, with the tracks before it that haven't played yet among them. Your own tracks stay ahead of the radio's. Turning it off puts the queue back in its order, and the tracks you added with Play next or Add to queue back where you added them. The Shuffle buttons of albums, playlists, mixes and liked songs start at a random song you don't dislike.
- **Repeat** (Ctrl+R, ⌘R) goes from off to Repeat: all, which plays the queue again from its start, to Repeat: one, which plays the track again, and back to off.
- **Previous** (Ctrl+←) starts the track again once it played for more than 3 seconds, and goes to the track before otherwise. **Next** (Ctrl+→) goes to the next one.

## Autoplay and radios

A radio plays tracks like one you choose, from YouTube Music.

- **Autoplay,** in Settings › Playback › Sound and on by default: when the queue runs out, a radio of what you played carries on.
- **Start radio,** in a track's menu or a list's More menu, plays the track, then a radio of it. The command palette's "Start a radio of what plays" does it for the track playing; [Settings › Shortcuts](settings.md#change-a-shortcut) can give it a key.
- **What a radio plays:** songs like the one it started from, leaving out the songs you dislike and, while Skip songs made with AI is on, passing over those YouTube labels as made with AI. It passes over other uploads and the music videos of songs already in the queue, and leaves out covers, mashups and other uploads unless it has nothing else. The radio of a music video starts from its song, so it carries on with songs, not more videos.
- A queue that ends on a podcast episode stops, rather than start a radio.

## Crossfade and gapless playback

- **Crossfade,** in Settings › Playback › Sound: tracks overlap by 0 to 12 seconds, the next fading in as the one before fades out. It's off by default.
- **With Crossfade off,** the next track starts as the one before ends: MixMusic finds the next track a second into the one playing and loads it ahead, so skipping ahead is quick too.
- **A pause in a song's last seconds** holds, with Crossfade on: the next song doesn't fade in a moment later.
- **Spotify songs that play in MixMusic itself** follow one another without a gap, as Spotify goes on to the next by itself, and a YouTube Music song after them starts without a wait.
- Podcast episodes never crossfade, and tracks don't crossfade on a [cast device](casting.md).

## Even loudness, volume levelling and the volume

- **Even loudness,** in Settings › Playback › Sound and on by default, turns loud YouTube Music tracks down to one level (-14 LKFS) from their first note, using YouTube's own measure of each, so none jumps out. Quiet recordings keep their level. Loud masters play 4 to 7 dB quieter than without it: turn up your system's volume, or turn Even loudness off, if they sound too quiet. A track keeps its level offline too, once measured. Spotify tracks that Spotify plays keep Spotify's own loudness, and live streams play at their own level, as YouTube measures none. [The audio engine](audio-engine.md#even-loudness) tells how it's done.
- **Volume levelling,** off by default, squeezes loud and quiet moments closer together as they play, and lifts quiet recordings.
- **The volume slider** has even steps of loudness: each tenth of it is 6 dB, and ↑ or ↓ changes it by about 3 dB. The mouse wheel over it turns it up and down, M mutes, and the speaker button beside it mutes and unmutes.
- **Volume curve,** in Settings › Playback › Volume: **Natural**, the default, gives the slider those even steps of loudness. On **Linear**, the level grows evenly along the slider: its middle plays at half the level, 6 dB down rather than 30, and the steps near the top are small, those near the foot large. The slider stays where it is when you change the curve; how loud that is changes. Spotify tracks play on the same curve, in MixMusic and on Spotify Connect, and the volume keys, the mouse wheel and the mini player's volume follow it too.

## The equalizer

Settings › Playback › Equalizer shapes the sound of what MixMusic plays itself.

![Settings › Playback scrolled to the Equalizer group: the Equalizer switch turned on, the Preset menu set to Custom, and the sliders of the preamp and the ten bands from 32 Hz to 16 kHz, each with its gain above it](shots/equalizer.webp)

- **Equalizer** turns it on and off.
- **Preset:** Flat, Bass boost, Treble boost, Vocal, Electronic, Rock, Acoustic, Late night or Lo-fi. Choosing one turns the equalizer on.
- **Bands:** a preamp and ten bands, at 32, 64, 125, 250 and 500 Hz and 1, 2, 4, 8 and 16 kHz, each from -12 to +12 dB in half steps. Moving a band makes the preset Custom.

It works on YouTube Music, and on Spotify tracks played from their recording there. Spotify's own player takes no equalizer from apps: for Spotify tracks that Spotify plays, use your system's equalizer. [The audio engine](audio-engine.md#the-equalizer) tells how the bands are made.

## Speed

MixMusic plays songs faster or slower, from half their speed to three times it, at their own pitch.

- **The speed button,** beside the volume, shows the speed while it isn't 1×. It opens a menu of steps, 0.75×, 1× (normal), 1.1×, 1.25×, 1.5×, 1.75× and 2×, with a slider below them for every speed from 0.5× to 3×, by 0.05.
- **Settings › Playback › Speed** has a slider for Songs and one for Podcasts, each with a normal link while it isn't 1×. Podcast episodes play at their own speed, apart from songs' (see [Podcasts](podcasts.md#speed)); the button, the commands and the media controls change the speed of the kind that plays.
- **The commands** Speed up and Slow down move the speed to the next quarter (1.1× goes up to 1.25×), and Normal speed sets 1×. They have no keys until you give them some in Settings › Shortcuts.
- **The [mini player](desktop.md#the-mini-player)** has the speed in its menu, on a right click.

The player bar shows the speed after the artists while it isn't 1×. The clock, the seek bar and synced lyrics keep the track's own time, a crossfade lasts the seconds you hear, and Last.fm counts how much of a song you heard. The system's media controls move on at the speed, and setting the rate over MPRIS on Linux sets it.

Spotify plays its own tracks and episodes at their own speed, in MixMusic or on Spotify Connect, and live streams play as they're broadcast: for those, the speed button is dimmed, and its tip tells why. Spotify tracks played from YouTube Music take the speed.

## Dislikes and skipping

Dislike a song you never want to hear again:

- with the thumbs-down beside the heart, in the player bar, on the now playing page and in the mini player;
- with Dislike in any track's menu;
- with Ctrl+D (⌘D on macOS) for the song playing, or from any app with a [global shortcut](desktop.md#global-shortcuts) of your own.

A disliked song that plays moves on at once, and a toast's undo takes the dislike back and goes back to the song where it was. From then on, the queue passes over the song as it moves on, forward and back, radios leave it out, and the play buttons of albums, playlists, mixes and artists start at their first song you don't dislike. A song you pick yourself still plays, and Up next shows the songs it skips.

A song is liked or disliked, not both: disliking a liked song unlikes it, on Spotify too. Signed in to YouTube Music, your dislikes of its songs go there too; Spotify takes none from apps, so those of its tracks stay in MixMusic. Library › Disliked lists them all (see [Your library](library.md#disliked-songs)), and "Skip songs you dislike" in Settings › Playback › Sound turns the skipping off.

## Skipping songs made with AI

"Skip songs made with AI", in Settings › Playback › Sound and off by default, passes over the songs YouTube labels as made with AI, as the queue moves on by itself: at the end of a song, on Next and Previous, in autoplay's radios, and from the play and shuffle buttons of a list.

- YouTube labels only some of them, so others still play.
- A song you pick by name plays anyway.
- Skipped songs stay in the queue, dimmed and marked "made with AI", and one notice tells of them.
- MixMusic asks YouTube about each song once, signed out, and keeps the answer for 30 days. That can add up to half a second before a song you skip to starts; a song YouTube doesn't answer for within 10 seconds plays.
- Spotify songs are checked by their recording on YouTube Music, when they play from there.

## Live streams

YouTube's live streams play, such as its 24/7 radio channels, from about 15 seconds behind the broadcast, for as long as it goes on. [Where the audio streams come from](streams.md#live-streams) tells how they play.

- The player bar and the mini player say "live" in place of the time and the length, and you can't seek. Previous goes to the track before.
- A live stream ends only when its broadcast does.
- After a pause of up to about an hour, it goes on from where it was; after a longer one, from near the live edge.
- It plays at its own level, as YouTube measures none for Even loudness, and at its own speed. Data saver plays its 22 kHz stream.
- It isn't kept in the [offline cache](library.md#the-offline-cache), doesn't download, and isn't scrobbled to Last.fm, which hears only that it plays. [Discord](desktop.md#discord) shows how long it played. Live streams don't [cast](casting.md) yet.

## The picture of music videos

Music videos can show their picture on the now playing page, in place of the cover. Turn on "Show the video of music videos" in Settings › Playback › Music videos, press the video button beside the play and next buttons of the now playing page while a music video plays, or use the command "Show or hide the video of music videos".

- The picture is muted and follows MixMusic's own sound, pausing and seeking with it.
- It shows only for official music videos and other uploads, never for songs or live streams.
- It needs the system's web view, WebView2 on Windows and WebKitGTK on Linux: without one, the cover stays, and the setting says why. [Where the audio streams come from](streams.md#the-picture-of-music-videos) tells how the picture keeps in time.
- It's fetched as it plays, about 3 MB a minute at 360p, and under 1 MB with data saver. Nothing is fetched while it's paused or not shown: on another page, with the window hidden or minimized, or with the mini player open.
- While your system asks for less motion, a button on the cover plays the picture.
- A web view runs beside MixMusic while pictures show, which takes memory until a minute after the last one.

## The output device

MixMusic plays on the system's default output device, and moves to the new one when you change it in your system's sound settings. To play music on headphones or a DAC while calls and the system's sounds stay on the speakers, choose the device MixMusic plays on: **Output device**, at the top of Settings › Playback › Sound.

- **The choices** are **System default**, which names the device it is now, and each output device of your computer by the name your system gives it. Two of one name, as two monitors of one model, are told apart by a number. The refresh button beside it looks for devices again.
- **The speaker button** beside the volume chooses too, where the window is wide enough for it beside the player bar's other buttons, and while there's a choice to make: more than one device, or one chosen. It turns to the accent while a device is chosen, and its tip names it.
- **When the device goes away,** as headphones you unplug, MixMusic plays on the system's default device and says so, and goes back to the device a second after it's back, saying so too. A device that's there but can't play, as one another app holds, gives way to the default for 10 seconds before MixMusic tries it again. Settings and the menu still show the device chosen, as not connected.
- **Where it doesn't apply:** a [cast device](casting.md) plays what you cast on itself, and Spotify tracks that Spotify plays, in MixMusic itself or on Spotify Connect, play where Spotify plays them: in MixMusic, on the system's default device. The menu says so while one plays.
- **What each system lists:** on Windows, its playback devices that are enabled; on macOS, its devices with outputs, as Sound settings shows them; on Linux, PulseAudio's or PipeWire's sinks, or, without either, ALSA's devices. [The audio engine](audio-engine.md#the-output-on-each-system) tells more.

5.1 surround, the visualizer and Even loudness work on any device.

## 5.1 surround

Some YouTube Music tracks have a 5.1 surround mix beside their stereo one. While one plays, a 5.1 button shows by the player: pressing it plays the surround mix from where the stereo one was, and pressing it again goes back to stereo. The choice stays for later tracks, as "Play 5.1 surround when a track has it" in Settings › Playback › Sound, which the button turns on and off. It's off by default.

- A track doesn't wait to learn whether it has a 5.1 mix: it starts at once, and the 5.1 button may show a moment later, once YouTube has said so. With 5.1 on, the track then plays its surround mix from where it is.
- On a 5.1 output device, the six channels play as they are. On headphones or two speakers, they're mixed down to stereo, as the track's own downmix levels say.
- While it's on, stereo tracks play on the front left and right speakers of a 5.1 system.
- The surround mix [needs yt-dlp and a JavaScript runtime](streams.md#surround-mixes-and-what-they-need), which find it in a few seconds the first time the track plays.
- The surround mix goes to the offline cache beside the stereo audio. Downloads keep the stereo audio, which plays while you're offline, and a surround mix that can't play gives way to the stereo one.

## How Spotify tracks play

"How Spotify tracks play", in Settings › Playback › Spotify tracks, has three choices. Signing in to Spotify comes first: see [Spotify](spotify.md). [How Spotify tracks play](spotify-playback.md), under How it works, tells what each way does inside.

- **In MixMusic, from Spotify,** the default on Windows and macOS. MixMusic plays the track itself, at Spotify's own quality, with no Spotify app open, and shows among your Spotify devices while it plays. It needs Spotify Premium, and Windows or macOS, whose web views can play Spotify's protected audio; on Linux the choice is greyed out and says why. A sign-in made before MixMusic played Spotify itself doesn't allow it: Settings › Playback then asks you to sign in again, and until you do, Spotify tracks play from YouTube Music.
- **On Spotify Connect,** the default on Linux. A Spotify device plays the track: your Spotify app on this computer, your phone or a speaker. It needs Spotify Premium and a device with Spotify open. The Device row under the choice, or the devices button by the player, picks the device; by default, the active one. The player bar says "on Spotify".
- **In MixMusic, from YouTube Music.** MixMusic plays the track's recording on YouTube Music, with the equalizer, crossfade and the visualizer. It works without Premium. The player bar says "via YouTube Music", and Find another YouTube Music match in the track's menu looks again when the recording is wrong. In `settings.json`, this is `"spotifyPlayback": "match"`.

When Spotify can't play a track the way you chose, MixMusic plays it from YouTube Music instead, and says why once. Spotify's [podcast episodes](podcasts.md) play only from Spotify, in the first two ways.

While Spotify plays a track, in MixMusic or on Spotify Connect, its sound is Spotify's: the equalizer, Even loudness, the speed and crossfade leave it as it is.

## Data saver

Quality, in Settings › Playback › Sound, chooses between best and data saver.

- **Data saver** plays the smallest audio YouTube offers for each track, and a live stream's 22 kHz stream. For the few videos that also come in YouTube's stable-volume version, which is no smaller and squeezes the sound, it plays the original.
- The picture of music videos takes its lowest quality.
- [Downloads](library.md#how-they-download) take the quality chosen here.

## Without the network

- **The [offline cache](library.md#the-offline-cache)** keeps the tracks you played, as much as Settings › Playback › Offline cache allows, and they play again without the network. Downloads play without it too: see [Your library](library.md#downloads).
- **While you're offline,** the queue skips the tracks that aren't downloaded or cached, without an error for each, and waits for the network only once none is left. A toast says that your downloads still play, and Home offers to open them.
- **When the network drops** partway through a song, the song waits for it, and plays on from where it stopped once it's back. The player bar says that it waits for the network, and the play button chooses whether the song plays then. Songs whose audio MixMusic holds still play meanwhile.
- **A downloaded song, or one on this computer, that can't be read,** as a damaged file or one on a drive unplugged, doesn't wait for the network, which wouldn't help: MixMusic tries it again at once, and then moves on, saying why, as File not found or Can't read the file.
- **A song whose audio stops arriving,** as on a slow connection, loads again where it was, up to three times. When three songs in a row can't play, MixMusic stops, rather than step through the rest of the queue.
- **When the computer goes to sleep,** MixMusic pauses, and stays paused when it wakes, ready where it was. A Spotify Connect device elsewhere plays on.

## A sleep timer

MixMusic has no sleep timer of its own. The community's Sleep Timer plugin, in the [marketplace](marketplace.md#install-and-enable-a-plugin) (the store icon in the sidebar), adds one: it fades the music out over 30 seconds and pauses it after 15, 30, 45, 60 or 90 minutes, or at the end of the track. It adds a sleep timer page to the sidebar and commands to the command palette, such as "Sleep timer: 30 minutes" and "Sleep timer: cancel".
