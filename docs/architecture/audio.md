# The audio engine

- **`internal/audio`** is MixMusic's audio engine: two decks, each a
  goroutine that reads a track, Opus in WebM with `webm` and `opus`, AAC
  in MP4 with `mp4`, `aac` and `resample`, or the AAC of a live stream's
  packed audio, its ID3 tags passed over, which has no length and can't
  seek, or a file of the music on this computer (`pcm.go`): FLAC with
  `flac`, Opus and Vorbis in Ogg with `ogg`, `opus` and jfreymuth/vorbis,
  MP3 with go-mp3, the delay and padding of its LAME tag dropped, and
  WAV, each converted to 48 kHz and mixed down to stereo, as its first
  bytes tell, and decodes it into
  a ring the audio thread plays from, at 48 kHz, each turning a
  track louder than -14 LKFS down to it, from the loudness its `Measured`
  source tells (Even loudness), and the preamp, equalizer, levelling
  compressor, volume and analyser of the web player MixMusic had, ported
  from Chromium's Web Audio, which they match sample for sample. A deck
  plays its track faster or slower at its own pitch by WSOLA
  (`stretch.go`), the waveform-similarity overlap-add of Verhelst and
  Roelands, between its ring and the mix; at 1× it plays the ring as it
  is, as a live stream always plays, with nothing ahead of its live edge
  to play faster. The six channels of 5.1 surround stretch alike, each
  window found on the mix of them all.
  `MIXMUSIC_STRETCH_WAV=<dir> go test ./internal/audio -run StretchListen`
  writes the test files at each speed, to listen to.
  `MIXMUSIC_LIVE=1 go test ./internal/audio -run Live` plays songs of both
  codecs, and a 24/7 live stream, as the app finds them on YouTube,
  against ffmpeg's decode. `Probe` tells whether a deck plays a file, and
  how long, reading little of it, for the index of the music on this
  computer; with ffmpeg on the PATH, the tests play a file of each kind
  it makes against its decode.
  `internal/audio/output` plays it on the system's output, without cgo:
  through WASAPI on Windows, an Audio Queue on macOS, and PulseAudio,
  which PipeWire serves too, or ALSA on Linux. On Windows it also hears
  what a process and its children play, through WASAPI's
  process loopback: `internal/app`'s listener hears Spotify there, in
  MixMusic's webview or the Spotify app, into a `Tap`, the analyser of
  the engine for audio outside it, which the visualizer and the plugins'
  analyser read while Spotify plays.
