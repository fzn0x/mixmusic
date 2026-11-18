# Containers and codecs

- **`webm`** reads the audio of WebM files, as YouTube serves it:
  the packets of the first audio track with their times, and seeking by the
  Cues, reading a file played from its start straight through.
- **`opus`** decodes Opus: a pure Go port of libopus 1.6.1's decoder
  (`celt`, `silk` and the range decoder `rangedec`, in `opus/internal`), which decodes the test
  vectors of RFC 8251 as libopus does, bit for bit.
- **`mp4`** reads the audio of MP4 files, as YouTube serves its
  AAC: the packets of the first audio track with their times, and the
  codec delay of the edit list, from progressive files and fragmented
  ones, seeking by the sample table, the `sidx` or the `moof` boxes.
- **`aac`** decodes AAC-LC, in mono or stereo, with all of its
  tools, and reads the headers of ADTS frames: a pure Go decoder derived from SharpJaad, the C# port of JAAD,
  under SharpJaad's MIT license, which decodes YouTube's AAC within about
  136 dB of ffmpeg's decoder, but for the few frames whose channels share
  their noise, as the standard has it and ffmpeg's decoder doesn't.
  **`resample`** converts it from its rate, 44.1 kHz for
  YouTube, to the engine's 48 kHz, with a polyphase filter of
  Kaiser-windowed sincs.
- **`flac`** decodes FLAC, written from RFC 9639: the STREAMINFO and
  SEEKTABLE, and every kind of frame and subframe, checking their CRCs and
  passing over the frames that fail them, seeking by the SEEKTABLE's
  points, else by halving the file, searching for frames by their sync
  codes and headers. Its tests decode FFmpeg's files to the MD5 of their
  STREAMINFO, and, with ffmpeg on the PATH, encode sound at each of its
  levels and ways of coding stereo and decode it back as it was.
- **`ogg`** reads the packets of an Ogg file's first logical bitstream,
  RFC 3533, with the granule positions of their pages, checking their
  CRCs, and seeks by them, as the Opus and Vorbis of files on a computer
  are kept.
- **`ac3`** decodes AC-3 and E-AC-3, the 5.1 surround of YouTube's
  formats 380 and 328: a pure Go decoder written from ATSC A/52 and its
  Annex E, spectral extension and AHT among its tools, into six
  channels, with the stereo downmix the
  stream's own mix levels ask for. Of AC-3 it decodes every audio coding
  mode, 1/0 to 3/2, with or without the LFE, at 48, 44.1 and 32 kHz, of
  bsid 8 and below, and reads the Lo/Ro mix levels of Annex D's alternate
  syntax (bsid 6) for the downmix. Of E-AC-3 (bsid 11 to 16) it decodes
  frames of 1, 2, 3 or 6 blocks and independent substream 0, and skips the
  frames of the others: of a stream of more than 5.1 channels, or of joint
  object coding, it plays the 5.1. Enhanced coupling and the reduced
  sample rates return `errors.ErrUnsupported`, and transient pre-noise
  processing, optional for a decoder, is left out. The six channels are
  float32, interleaved as Left, Right, Center, LFE, LeftSurround,
  RightSurround, those the stream lacks silent. It applies the blocks'
  dynamic range gains, leaves dialnorm to the player, checks each
  syncframe's CRCs, and makes errors of malformed frames, never panics.
  `ParseDAC3` and `ParseDEC3` read the payload of an MP4's `dac3` and
  `dec3` box (header left out) into a `Config`: sample rate, `BSID`, mode,
  LFE and bitrate, with `Channels()` giving 6 for 5.1; `ParseDEC3`
  describes independent substream 0. A `Decoder` needs neither, as each
  syncframe says it all again. It decodes streams of ffmpeg's encoders
  as ffmpeg's decoder does to about 110 dB, and its tests edit those
  streams, or write them again, to use the tools the encoders don't.
