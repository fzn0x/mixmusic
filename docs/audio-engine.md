# The audio engine
<!-- docs: group=inside order=20 -->

MixMusic plays YouTube Music through an audio engine of its own, written in Go. This page covers its decoders, loudness, equalizer, crossfade, speed and 5.1.

The engine plays everything MixMusic plays itself: YouTube Music tracks
and podcast episodes, live streams, and Spotify tracks played from their
recording on YouTube Music. Spotify tracks that Spotify plays, in MixMusic
or on Spotify Connect, play in Spotify's own player instead, which the
engine's equalizer, crossfade and speed don't reach:
[how Spotify tracks play](spotify-playback.md) tells why. For the code,
read [the audio engine's part of the architecture](architecture/audio.md)
and [its codecs](architecture/codecs.md).

## Decoding

MixMusic decodes the audio YouTube serves it in Go, with decoders of its
own. It uses no FFmpeg and no codec of the system.

| Format | What YouTube uses it for | Decoder | Written from |
|---|---|---|---|
| Opus in WebM | almost every track | `opus`, a port of libopus 1.6.1's decoder | RFC 6716, with the updates of RFC 8251, whose test vectors it decodes as libopus does, bit for bit |
| AAC-LC in MP4 (format 140) | the few tracks YouTube has only in AAC | `aac`, derived from SharpJaad, the C# port of JAAD | ISO/IEC 14496-3; it decodes YouTube's AAC within about 136 dB of FFmpeg's decoder |
| AAC in HLS packed audio (formats 234 and 233) | live streams | `aac`, after the packed audio's ID3 tags | RFC 8216 |
| AC-3 and E-AC-3 in MP4 (formats 380 and 328) | the 5.1 surround mix some tracks have | `ac3` | ATSC A/52 and its Annex E, every channel layout and every tool; it matches FFmpeg's decoder to about 110 dB |

- **The containers** are read by `webm` (Matroska's EBML, seeking by the
  file's Cues) and `mp4` (ISO/IEC 14496-12, seeking by the sample table,
  the `sidx` or the `moof` boxes).
- **No added silence.** The decoders drop the samples an encoder puts
  before the music, as the Opus header's pre-skip and the MP4 edit list
  tell, so they add no silence before a track, and its times are exact.
- **What it doesn't play.** HE-AAC (format 139), which YouTube also
  serves, is never picked: a video in AAC alone plays its AAC-LC stream.
  A track in no format the engine decodes fails, naming its format.

These decoders are Go packages anyone can import, documented on
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic): `opus`, `aac`,
`ac3`, `webm`, `mp4` and `resample`.

### Files on this computer

The [music on this computer](local-music.md) plays through the same
engine, from its files, which the engine tells apart by their first bytes:

| Format | Decoder | Notes |
|---|---|---|
| FLAC | `flac`, written from RFC 9639 | every frame and subframe kind, up to 8 channels and 32 bits; it decodes FFmpeg's FLAC as it was encoded, sample for sample, as the MD5 of its STREAMINFO checks; seeking by the SEEKTABLE, else by searching for frames |
| Opus in Ogg | `opus`, after `ogg`, written from RFC 3533 and RFC 7845 | mono and stereo; seeking by the pages' granule positions |
| Vorbis in Ogg | [jfreymuth/vorbis](https://github.com/jfreymuth/vorbis), after `ogg` | within about 109 dB of FFmpeg's decoder |
| MP3 | [go-mp3](https://github.com/hajimehoshi/go-mp3), of Hajime Hoshi | MPEG-1, 2 and 2.5 Layer III, to 16 bits, within about 80 dB of FFmpeg's decoder; the delay and padding LAME's tag tells are dropped, as FFmpeg drops them, so that an album plays without gaps |
| WAV | the engine's own | PCM of 8 to 32 bits, and floats |
| AAC in MP4 (`.m4a`) | `aac`, after `mp4` | AAC-LC, as YouTube's |

- **Any rate, any channels.** A file's audio is converted to 48 kHz by
  `resample`, and more than two channels are mixed down to stereo, as
  ITU-R BS.775 mixes 5.1 down, scaled so that they don't clip.
- **The decoders MixMusic didn't write**, go-mp3's and Vorbis's, run
  guarded: a file made to break one fails as a broken file, and never
  stops MixMusic. Every reader of files, MixMusic's own and theirs, is
  fuzzed with broken files.

## Resampling

The engine plays everything at 48 kHz, the rate Opus decodes to. YouTube's
AAC is at 44.1 kHz (22.05 kHz for a live stream with data saver), so
`resample` converts it: a polyphase filter of Kaiser-windowed sincs, with
exact coefficients for each of its phases, 160 of them from 44.1 to 48 kHz.
From 44.1 kHz, it passes everything below 20 kHz untouched, within
0.001 dB, and keeps the images and aliases it makes 100 dB below the sound.

## What a track goes through

Each track plays on one of two decks, so that one can fade into the next.
In order:

1. **The deck** reads the track and decodes it into a buffer of about 1.4
   seconds of audio.
2. **Speed** stretches or squeezes it, at its own pitch, when it isn't 1×.
3. **Even loudness** turns the track down when it is loud, as below.
4. **The two decks are mixed**, which a crossfade moves from one to the
   other.
5. **The preamp and the equalizer** shape the sound.
6. **Volume levelling**, when it's on, squeezes loud and quiet moments
   closer together.
7. **The volume** of the slider.
8. **The analyser** reads what you hear, for the visualizer and plugins.
9. **The output** plays it on your device.

Steps 5 to 8 are ports of Chromium's Web Audio, the biquad filters,
DynamicsCompressor and AnalyserNode of Blink, which MixMusic used while it
ran in a web view, before 0.5.0. They match Chromium sample for sample, so
MixMusic sounds as it did. Pausing and playing fade over 5 ms, so that
they don't click.

## The output on each system

The engine plays 32-bit float audio at 48 kHz on the system's default
output device, in stereo, or in 5.1 where the device has it, and follows
the default device when you change it. It renders 10 ms of audio at a
time, at most 40 ms ahead of the device. After a second of silence, as
when you pause, it lets the device go, so that the computer can sleep.

- **Windows:** WASAPI, in shared mode, with Windows converting to the
  device's own format. The thread that renders the audio registers with
  Windows' multimedia scheduler, MMCSS, as Pro Audio (or runs at a higher
  priority where MMCSS isn't there), so the music doesn't stutter while a
  game or a browser keeps the computer busy (fixed in 0.5.2).
- **macOS:** an Audio Queue of Core Audio on the default output. A 5.1
  queue tells Core Audio its channel layout, which maps the channels to
  your speakers.
- **Linux:** PulseAudio's simple API, which PipeWire serves too, or else
  ALSA's default device. PulseAudio and PipeWire move the stream when the
  default device changes; ALSA's stays stereo.

No C library is linked in: the output calls the system's audio libraries
while MixMusic runs.

## Even loudness

Settings › Playback › Sound › Even loudness, on by default, turns loud
tracks down to one level, so that none jumps out.

- **Where the measure comes from.** YouTube measures the loudness of each
  video, the integrated loudness of ITU-R BS.1770 in LKFS, and its player
  API tells it with the video's streams. For a stream yt-dlp found, which
  doesn't carry it, MixMusic asks the player API for the measure apart.
- **What it does.** A track louder than -14 LKFS, YouTube's own target,
  plays turned down by the difference: a master at -8 LKFS plays 6 dB
  quieter. A quieter track is never turned up, as YouTube tells no peak
  level and a quiet recording turned up could clip. Loud masters are
  common, so many tracks play 4 to 7 dB quieter than with it off.
- **When.** A track takes its level with its first sample, so it never
  starts loud; a later change glides.
- **Offline.** MixMusic keeps the measures of the tracks you played in
  `loudness.json` in its cache folder, so cached and downloaded tracks
  keep their level without the network.
- **Not measured.** Live streams have no measure, and play at their own
  level. Spotify tracks that Spotify plays keep Spotify's own loudness.

**Volume levelling**, the switch below it and off by default, is
different: a compressor that squeezes loud and quiet moments closer
together as they play, and lifts quiet recordings. It starts at -24 dB,
with a knee of 24 dB, a ratio of 4 to 1, an attack of 10 ms and a release
of 250 ms, the settings of the web player MixMusic had.

**The volume slider** has even steps of loudness: each tenth of it is
6 dB, down to -40 dB, and one press of the volume keys is about 3 dB.

## The equalizer

Settings › Playback › Equalizer has ten bands and a preamp, each from
-12 dB to +12 dB in steps of half a decibel. The lowest band, 32 Hz, is a
low shelf, the highest, 16 kHz, a high shelf, and the eight between are
peaking filters at 64, 125, 250, 500 Hz and 1, 2, 4 and 8 kHz, with a Q of
1.1. Choosing a preset turns the equalizer on; moving a band makes it
Custom.

| Preset | 32 | 64 | 125 | 250 | 500 | 1k | 2k | 4k | 8k | 16k |
|---|---|---|---|---|---|---|---|---|---|---|
| Flat | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| Bass boost | +6 | +5 | +4 | +2 | 0 | 0 | 0 | 0 | 0 | 0 |
| Treble boost | 0 | 0 | 0 | 0 | 0 | +1 | +2 | +4 | +5 | +6 |
| Vocal | -2 | -1 | 0 | +2 | +4 | +4 | +3 | +1 | 0 | -1 |
| Electronic | +5 | +4 | +1 | 0 | -1 | +1 | 0 | +1 | +4 | +5 |
| Rock | +4 | +3 | +2 | 0 | -1 | 0 | +2 | +3 | +4 | +4 |
| Acoustic | +3 | +2 | +1 | +1 | +2 | +1 | +2 | +2 | +2 | +1 |
| Late night | +4 | +3 | +1 | 0 | 0 | 0 | 0 | +1 | +2 | +3 |
| Lo-fi | -3 | 0 | +3 | +4 | +2 | 0 | -2 | -4 | -6 | -8 |

The gains are in dB. The equalizer shapes only what the engine plays.
Spotify's own player, in MixMusic or on Spotify Connect, can't be changed
by any app, as [Spotify notes](https://support.spotify.com/us/article/equalizer/):
use a system-wide equalizer for it.

## Crossfade and the next track

**Crossfade**, in Settings › Playback › Sound, overlaps tracks by 1 to 12
seconds; at 0, the default, it is off.

- The next track starts on the other deck while the one playing fades
  out: one goes down and the other up, evenly, over the seconds you set.
- The seconds are those you hear: at 2×, a fade of 5 seconds starts 10
  seconds of the track before its end.
- A track doesn't crossfade when it is shorter than three times the fade,
  when it or the next is a podcast's episode, while you cast, or when
  Spotify plays it.
- A pause in a track's last seconds holds: the next track doesn't fade in
  behind it. Played again, it fades while there's time, or the next one
  starts as it ends.

**Without a crossfade**, the next track is ready before its turn: a second
into a track, MixMusic finds the next one's stream, and ten seconds in, it
fetches its audio and decodes its start on the other deck. When the track
ends, the decks swap and the next track starts at once, without loading.
Live streams and episodes are not fetched ahead.

## Speed, at the track's own pitch

Settings › Playback › Speed sets the speed of songs and, apart, of
podcasts' episodes, from 0.5× to 3× in steps of 0.05. The speed button by
the player, the mini player's menu and the command palette change it too.

The engine changes the speed without changing the pitch by WSOLA, the
waveform-similarity overlap-add of Verhelst and Roelands (1993). It cuts
the audio into windows of 20 ms, half a window apart in what you hear.
Each window is taken from about where the speed puts it, within 15 ms
either way, at the place that best continues the window before it, so
that their overlap adds up in phase rather than smearing. At 1× the track
plays as it is, with nothing in between. The six channels of 5.1 are cut
at the same places, so they stay together.

The clock, the seek bar and synced lyrics keep the track's own time.
Spotify's players and live streams keep their own speed, and the speed
button is dimmed for them.

## Surround sound

Some tracks on YouTube Music have a 5.1 surround mix beside their stereo
one, in AC-3 or E-AC-3. While one plays, a 5.1 button shows by the player,
which plays the surround mix from where the stereo one was. "Play 5.1
surround when a track has it", in Settings › Playback › Sound and off by
default, keeps the choice for later tracks. Finding the mix takes yt-dlp
and a JavaScript runtime: [where the audio comes from](streams.md#surround-mixes-and-what-they-need)
tells why.

- **On a 5.1 device**, the six channels play as they are: left, right,
  centre, LFE and the two surrounds, which go to side speakers where the
  device has them, else to rear ones. On Windows, a device whose speakers
  have a centre, an LFE and a pair of surrounds; on macOS, a device of
  six output channels or more; on Linux, a PulseAudio sink with them.
- **On headphones or two speakers**, the engine mixes the six channels
  down to stereo the way the track's own stream asks, by the LoRo downmix
  of ATSC A/52, section 7.8: the left output takes the left channel, the
  centre at its mix level and the left surround at its mix level, and
  the right output the same of the right.
- **The mix levels** come from the stream: the centre at -3, -4.5 or
  -6 dB, the surrounds at -3 or -6 dB or left out, and a stream may also
  carry levels meant for this downmix, from +3 dB to -6 dB in steps of
  1.5 dB. An E-AC-3 stream without them takes the middle levels of
  AC-3, the centre at -4.5 dB and the surrounds at -6 dB. The gains are
  scaled down so that neither output can overload, and the LFE is left
  out, as the standard allows, unless an E-AC-3 stream gives its level.
- **Stereo tracks**, while the setting is on, play on the front left and
  right speakers of a 5.1 device.
- **Offline**, a track plays its stereo audio: downloads keep stereo
  only. The offline cache keeps a surround mix beside the stereo audio of
  the tracks it played in 5.1.

## The visualizer

The now playing page draws the spectrum of what plays as 48 bars over the
foot of the cover, in the accent colour, and plugins read the same
analyser through `api.player.analyser()`. It is the Web Audio
AnalyserNode of the web player, ported: a Blackman-windowed FFT of 256
samples (128 bands), with a smoothing time constant of 0.8, from -100 to
-30 dB. It hears the
mono mix of what plays: the average of left and right, and of 5.1, its
stereo downmix at the standard's levels without the LFE, so a track draws
alike either way.

Spotify plays in Spotify's own player, which the engine never hears. On
Windows 10 build 20348 and later, MixMusic hears it anyway, in MixMusic's
own web view or in the Spotify app on this computer, through Windows'
process loopback, which hears one app and its children alone. It analyzes
that audio in memory and never records or sends it. Where MixMusic can't
hear what plays, as on a Spotify device elsewhere, the visualizer draws
nothing.

## Buffering

- **The stream.** A track's audio is fetched from its start in the
  background, in chunks of 4 MB, as YouTube throttles requests for whole
  files, and kept in memory while it plays, so seeking back costs
  nothing. A seek more than 512 KB past what came starts a new request
  there.
- **The engine** decodes about 1.4 seconds ahead on each deck, and the
  output holds at most 40 ms.
- **When the network stalls.** A request that gets no byte for 15
  seconds, or whose connection drops, connects again from the byte it
  reached, after a wait that doubles each time, for up to two minutes;
  the track plays on as soon as the network answers. A track that broke
  off after it played is loaded again up to three times, after 1, 3 and 9
  seconds, and gets all three tries back once it plays on for ten
  seconds.
- **When the network is gone.** After 8 seconds of buffering, MixMusic
  checks whether the network answers. While it doesn't, the queue plays
  on with the tracks it holds, downloads and the offline cache, and waits
  where it was once none is left.

## The offline cache

Settings › Playback › Offline cache keeps the tracks you played, so that
they play again without the network.

- **What goes in.** Each track whose whole audio came, as one you played
  to its end, kept as YouTube sent it, in the `audio` folder of the cache
  folder. Live streams never go in.
- **How much.** Size: Off, 512 MB, 1 GB (the default), 4 GB or 16 GB. The
  tracks played least recently go first. A full disk never interrupts
  playback: the track just isn't kept.
- **Clearing.** Tracks kept shows how many tracks it holds and their size,
  with Clear tracks; Covers kept does the same for the covers MixMusic
  keeps, up to 256 MB, with Clear covers.
- **Downloads are apart.** Downloads stay until you remove them, and
  clearing the cache leaves them:
  [downloads and the cache](streams.md#downloads-and-the-cache) compares
  the two.
