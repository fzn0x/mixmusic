# Casting
<!-- docs: group=using order=70 -->

MixMusic casts to Chromecast, Google Home and Nest speakers, and TVs with
Chromecast built in; to DLNA and UPnP renderers, such as smart TVs, AV
receivers and network speakers without Chromecast; and to FCast receivers:
what plays there, what stays, and what to allow.

## Cast what plays

The cast button shows in the [player bar](getting-started.md#the-player-bar),
beside the queue button, while a cast device is on your network, or while
MixMusic plays on one.

1. Click the cast button. Its menu lists **This computer** and each device
   MixMusic found, by its name and its model, as "Living Room TV · Chromecast"
   or "Kitchen · Google Nest Mini". A DLNA renderer that names no model
   shows as "Study · DLNA renderer", and an FCast receiver as
   "Bedroom TV · FCast receiver". The mini player's Cast section lists the
   same.
2. Choose a device. What plays moves there, from where it was, playing or
   paused as it was; the menu says "connecting…" beside it meanwhile.
   The button's tip then says "Playing on" and the device's name.
3. To bring the music back, open the menu again and choose **This
   computer**. It plays on here from where the device was.

MixMusic looks for devices a few seconds after it starts, then every 45
seconds, and again as you open the cast menu: Google Cast devices and FCast
receivers by mDNS, DLNA renderers by SSDP. A device stays listed for three
minutes after it was last seen.

A group of Google Cast speakers, made in the Google Home app, shows by its
name with "Speaker group" in place of a model, as
"Everywhere · Speaker group", and plays as one device: its speakers play
together, and its volume is the group's.

Below the devices, the menu lists the other computers with MixMusic's
[remote control](remote-control.md) on, which you control from here
rather than play on, and the rooms of
[Listen with friends](listen-with-friends.md), which you start or join.

## What plays on the device

- **YouTube Music tracks**, music videos' audio included, with their
  cover, title and artist on the device's screen.
- **Spotify tracks** played from YouTube Music, as they are with **In
  MixMusic, from YouTube Music** in
  [Settings › Playback › Spotify tracks](playing-music.md#how-spotify-tracks-play):
  the device plays their recording on YouTube Music.
- **Your [downloads](library.md#downloads) and the
  [offline cache](library.md#the-offline-cache)**: MixMusic sends the
  device what it holds before it asks YouTube.
- **Songs on this computer**, from your [music folders](local-music.md),
  with their cover: the device plays the file itself.
  - **Google Cast devices** play every kind the library plays: MP3, FLAC,
    Ogg Vorbis, Opus, AAC in M4A and MP4 files, and WAV. Their media
    player plays FLAC up to 96 kHz and 24 bits, and WAV of whole-number
    samples: a file beyond that doesn't play there, and MixMusic says
    why, as "Living Room TV plays FLAC only up to 96 kHz and 24 bits,
    which this file goes beyond".
  - **A DLNA renderer** plays those of the formats it tells it plays, as
    MP3, FLAC or AAC.
  - **An FCast receiver** plays those of the formats it lists.

Spotify tracks that Spotify plays, in MixMusic itself or on Spotify
Connect, stay where they play. To cast your Spotify tracks, choose In
MixMusic, from YouTube Music for them.

## What stays in MixMusic

- **The queue.** MixMusic keeps the queue, and sends the device each track
  as the one before it ends. [Shuffle, repeat](playing-music.md#shuffle-and-repeat)
  and [autoplay](playing-music.md#autoplay-and-radios) work as they do on
  this computer.
- **The controls.** Play, pause, seek, next, previous and the volume act on
  the device. The volume is the device's own, which MixMusic shows
  as it changes, and doesn't keep in its settings. What you do with the
  device's own buttons or its remote shows in MixMusic too.
- **The sound.** The device plays the audio itself: tracks don't crossfade
  there, as it plays one at a time, and the equalizer and the other
  effects of MixMusic's [audio engine](audio-engine.md) stay on this
  computer, as does the [output device](playing-music.md#the-output-device)
  chosen for it. MixMusic asks the device to play at the
  [speed](playing-music.md#speed) you set.

A cast that ends by itself, on any kind of device, brings the track back to
this computer, paused, and a note says why. "Casting to {device} stopped:
the connection was lost" means the device stopped answering. "...it stopped
playing from MixMusic" means the device ended the cast, or, on a DLNA
renderer or an FCast receiver, that another app took it over, as someone
casting to it from a phone. "...{app} plays on the device now" means
another app took a Google Cast device over. The device's name stands where
{device} is.

## DLNA and UPnP renderers

A renderer plays what it says it plays: MixMusic asks it, and sends
YouTube Music tracks as Opus in WebM only to one that lists that. Others
get the track's AAC audio, from YouTube, even when your download or the
offline cache holds the track as WebM. When a renderer plays neither, the
track doesn't play, and MixMusic names the format it would need, as
"Living Room TV doesn't play AAC audio".

- **Its screen** shows the cover, title, artist, album and length.
- **The next track** goes to the renderer ahead, as the one before plays,
  so it plays on without a gap. A renderer that can't take it ahead stops
  at the end, and MixMusic starts the next one.
- **The controls** act on it as on any cast device. MixMusic asks it where
  it is every second while it plays, and reads its volume every 5 seconds,
  so a change from its own remote shows a moment later. Most renderers play
  only at their own speed: the speed button is dimmed then, and its tip
  says so.
- **When it stops answering**, ten times in a row, the music comes back to
  this computer, paused, as when a Google Cast device goes away, and the
  note says the connection was lost.

## FCast receivers

FCast receivers are apps for TVs, TV sticks and computers that play what a
sender gives them. MixMusic finds them on your network by mDNS.

- **A secure connection.** With a receiver of FCast's version 4 or later,
  MixMusic connects over TLS 1.3, and checks the receiver's key against the
  one it announced on the network: when they differ, MixMusic doesn't
  connect, and says why. It never falls back to an unencrypted older
  version with such a receiver. An older receiver gets FCast's version 3.
- **Formats.** A receiver that lists Opus in WebM gets that; others get
  the track's AAC audio, as renderers do.
- **When it stops answering.** MixMusic asks a receiver that has said
  nothing for 3 seconds whether it is there. One silent for 6 seconds, or a
  broken connection, ends the cast, and the note says the connection was
  lost. A receiver that loads other media ends the cast too, and the note
  says it stopped playing from MixMusic.
- **The volume.** FCast has no mute: muting sets the receiver's volume to
  nothing, and unmuting gives it back.

FCast is a registered trademark of FUTO. MixMusic implements the FCast
protocol on its own, and isn't affiliated with FUTO.

## Other speakers

- **Spotify Connect speakers** play Spotify tracks that Spotify plays, with
  Spotify Premium, from the Spotify Connect devices button by the player
  ([how Spotify tracks play](playing-music.md#how-spotify-tracks-play)).
- **Another computer** with MixMusic on plays what plays here, with
  [remote control's](remote-control.md#move-what-plays-here-there)
  **Move to …**, and plays here what plays there, with its
  [move here](remote-control.md#move-what-plays-there-here). Its page's
  **Output device** casts what plays there to a device it found.

## Firewalls and the local network

While casting, the device fetches the audio from MixMusic, over your local
network: MixMusic serves it only while it casts, only to that device, at
an address no one else could guess, a song on this computer and its cover
only while it plays there or comes next, and to a DLNA renderer over at
most 8 connections at once
([how casting works inside](architecture/cast.md)). So the first time you
cast, your system may ask about it:

- **Windows**: Windows Defender Firewall may ask whether MixMusic may take
  connections from your network. Allow it on private networks, such as the
  one at home. When a cast device can't reach MixMusic, **Settings ›
  Desktop › Windows Firewall** tells why, and its **Allow** lets MixMusic
  in on private networks, as
  [remote control's firewall part](remote-control.md#firewalls-and-the-local-network)
  tells.
- **macOS**: macOS may ask whether MixMusic may find and connect to
  devices on your local network. Allow it; System Settings › Privacy &
  Security › Local Network changes it later.
- **Linux**: a firewall such as ufw or firewalld that turns away incoming
  connections keeps the device from fetching the audio. MixMusic serves it
  at a port it picks each time, so allow incoming connections from your
  local network, as `sudo ufw allow from 192.168.1.0/24` does for the
  addresses 192.168.1.x.

When the device can't reach MixMusic, MixMusic tells you: "The cast device
couldn't reach MixMusic: let MixMusic take connections from your network in
the firewall".

## When it doesn't work

- **No cast button.** MixMusic hasn't found a device. The device must be on,
  and on the same network as your computer: a guest network, or a router
  that keeps its devices apart, hides it. Some networks don't pass the
  messages devices are found by (mDNS and SSDP) between Wi-Fi and wired
  devices. MixMusic lists only renderers at a private or link-local
  address of your network, which serve their own description.
- **"Couldn't connect to …"**: check that the device is on, and on this
  computer's network.
- **"The cast device didn't answer"**, or **"couldn't play this track's
  audio"**: try the track again, or another one.

## What doesn't cast yet

- [Live streams](playing-music.md#live-streams), such as YouTube's 24/7
  radio channels: MixMusic says "live streams don't play on cast devices
  yet".
- Spotify tracks that Spotify plays (see above).
- Gapless playback on Google Cast devices and FCast receivers, and lyrics
  or the picture of music videos on a TV's screen.
- Songs on this computer beyond what a Google Cast device plays, as FLAC
  of 192 kHz: MixMusic doesn't convert them to fit.
