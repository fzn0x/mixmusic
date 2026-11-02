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
- **`ac3`** decodes AC-3 and E-AC-3, the 5.1 surround of YouTube's
  formats 380 and 328: a pure Go decoder written from ATSC A/52 and its
  Annex E, of every audio coding mode and tool, spectral extension and
  AHT among them, into six channels, with the stereo downmix the
  stream's own mix levels ask for. It decodes streams of ffmpeg's encoders
  as ffmpeg's decoder does to about 110 dB, and its tests edit those
  streams, or write them again, to use the tools the encoders don't.
