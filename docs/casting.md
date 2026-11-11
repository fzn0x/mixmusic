# Casting
<!-- docs: group=using order=70 -->

MixMusic casts to Chromecast, Google Home and Nest speakers, and TVs with
Chromecast built in: what plays there, what stays, and what to allow.

## Cast what plays

The cast button shows in the [player bar](getting-started.md#the-player-bar),
beside the queue button, while a cast device is on your network, or while
MixMusic plays on one.

1. Click the cast button. Its menu lists **This computer** and each device
   MixMusic found, by its name and its model, as "Living Room TV · Chromecast"
   or "Kitchen · Google Nest Mini".
2. Choose a device. What plays moves there, from where it was, playing or
   paused as it was; the menu says "connecting…" beside it meanwhile.
   The button's tip then says "Playing on" and the device's name.
3. To bring the music back, open the menu again and choose **This
   computer**. It plays on here from where the device was.

MixMusic looks for devices a few seconds after it starts, then every 45
seconds, and again as you open the cast menu. A device stays listed for
three minutes after it was last seen. Groups of speakers aren't listed.

Below the devices, the menu lists the other computers with MixMusic's
[remote control](remote-control.md) on, which you control from here
rather than play on.

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

When another app takes the device over, as someone casting to it from a
phone, the cast ends, and MixMusic says so.

## Firewalls and the local network

While casting, the device fetches the audio from MixMusic, over your local
network: MixMusic serves it only while it casts, only to that device, at
an address no one else could guess
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
  messages devices are found by (mDNS) between Wi-Fi and wired devices.
- **"Couldn't connect to …"**: check that the device is on, and on this
  computer's network.
- **"The cast device didn't answer"**, or **"couldn't play this track's
  audio"**: try the track again, or another one.

## What doesn't cast yet

- [Live streams](playing-music.md#live-streams), such as YouTube's 24/7
  radio channels: MixMusic says "live streams don't play on cast devices
  yet".
- Spotify tracks that Spotify plays (see above).
- Gapless playback through the device's own queue, lyrics or the picture of
  music videos on a TV's screen, and groups of speakers.
