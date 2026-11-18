# Casting

MixMusic plays on cast devices, Chromecasts, Google Home and Nest speakers
and TVs with Chromecast built in, through their default media receiver,
which plays the media at a URL it's given and shows its title, artist and
cover on a screen; and, the same way, on DLNA and UPnP media renderers and
on FCast receivers.

- **`internal/cast`** finds the devices and speaks to them, in Go, with no
  library: the Cast v2 protocol, from its public descriptions.
  - `Browse` asks for `_googlecast._tcp.local.` by mDNS, as a one-shot
    querier (RFC 6762, 6.7): from a port of its own, to the mDNS group on
    each of the computer's networks, so the devices answer it directly, and
    MixMusic listens on no port of mDNS's. The few DNS records it reads,
    PTR, SRV, TXT and A, it reads by hand (`dns.go`); a device that
    answers the first question alone is asked for the rest. The TXT record
    tells the device's ID, its name (`fn`) and its model (`md`). A group
    of speakers, made in the Google Home app, answers as a device of its
    own, with the model "Google Cast Group", on a port of its own at the
    address of the speaker that leads it: it is a `Device` with `Group`
    set, and plays as any device.
  - A `Session` connects over TLS to the device's port, 8009, with the
    protocol's frames: a 4-byte length, then a `CastMessage` protocol
    buffer, whose seven fields `message.go` writes and reads by hand. It
    pings the device every 5 s, answers its pings, and takes it for lost
    after 16 s of silence. It launches the default media receiver,
    `CC1AD845`, or joins it where it runs, connects to its transport, and
    loads, plays, pauses, seeks and stops the media, sets the volume, and
    hears the device's statuses: of the media, which come as it changes,
    statuses of media a later load took over from left out, and of the
    receiver, whose volume shows in MixMusic and whose end, as another app
    takes the device over, ends the session. Devices have certificates of
    their own, which aren't checked: the connection is encrypted, and
    MixMusic doesn't ask the device to prove what it is.
  - `casttest` is a fake device for tests, a TLS server on the loopback
    address that speaks the protocol and keeps the messages it gets, and an
    mDNS responder for it, with DNS and frames of their own, apart from
    the package's.
- **`internal/upnp`** finds media renderers and plays on them, with no
  library. `Search` sends SSDP's M-SEARCH for `MediaRenderer:1` three
  times in 2.5 s, from a port of its own, never UDP 1900, which Windows'
  SSDP service holds, to the SSDP group on each of the computer's networks
  (`dnssd.SendToGroup`). It keeps the answers from private or link-local
  IPv4 hosts whose description, and the control addresses of their
  services, are their own, with an AVTransport of any version. A `Session`
  sends SOAP actions: `SetAVTransportURI` with DIDL-Lite metadata, `Play`,
  `Pause`, `Seek`, `Stop`, `SetNextAVTransportURI`, RenderingControl's
  volume and mute, and ConnectionManager's `GetProtocolInfo`, whose sink
  tells the formats the renderer plays. It asks `GetTransportInfo` and
  `GetPositionInfo` every 1 s while the renderer plays, every 0.6 s
  otherwise, and the volume every 5 s, and turns them into `cast`'s media
  statuses: a stop within 4 s of the end is the end, a track address that
  isn't MixMusic's ends the session as another app's, and so do 10 failed
  polls in a row. Media loaded to wait paused isn't sent until it plays.
  `upnptest` is a fake renderer over HTTP on the loopback address, and an
  SSDP responder for it.
- **`internal/fcast`** finds FCast receivers under `_fcast._tcp` with
  `internal/dnssd`, and speaks protocol v4 to one whose `v` TXT value is 4
  or more: the plaintext `Version`, then TLS 1.3 on the same connection,
  the receiver's key checked against its `fp` TXT value (the SHA-256 of its
  SubjectPublicKeyInfo, in base64), never v3 then; then FlatBuffers
  messages, which `flat.go` writes and reads by hand, each offset aligned
  as FlatBuffers' verifier asks. An older receiver gets v3, in JSON. The
  session pings a receiver silent for 3 s and ends after 6 s. FCast has no
  mute: a muted receiver's volume is 0. `fcasttest` is a fake receiver of
  both versions, with a key of its own and its own FlatBuffers, which it
  checks as the verifier does.
- **The player** (`internal/player/output.go`) plays the queue as it does,
  through a `Stream`; `SetOutput` gives it the cast device's in the
  engine's place, on its goroutine, and loads the current stream again
  there, where it was, playing or paused as it was. On a cast device the
  volume is the device's, which the settings don't keep, and tracks don't
  crossfade: it plays one stream at a time, and the player sends the next
  as one ends. Spotify tracks that play on a Spotify device stay there.
- **The app** (`internal/app/cast*.go`) looks for devices a few seconds
  after start, then every 45 s and as the cast menu opens, and lists those
  seen in the last 3 minutes. `castOut` turns the player's stream commands
  into the session's, one after another beside the player's goroutine, and
  the device's statuses into the engine's events: the position moves on
  between them, and the device is asked where it is every 5 s. The engine's
  events go nowhere while casting. A renderer's or a receiver's session is
  a `castOut`'s as a Google Cast session is (`castkinds.go`), and picks
  the format its device plays (`castformats.go`): Opus in WebM only where
  it lists it, else the stream's AAC (`stream.AACPath`), and a song on this
  computer in its own format. A Google Cast session takes a stream as it
  comes, and a song on this computer in its own format
  (`googleSource`): the default media receiver plays every kind the
  library plays, but FLAC beyond 96 kHz and 24 bits and WAV of
  floating-point samples, which are refused, with why (`googleBeyond`). The next track goes to a session that queues
  (`castQueue`, a renderer's `SetNextAVTransportURI`) as the player
  preloads it, and the next load takes in what the device moved on to.
- **The audio** comes from MixMusic: the addresses the player plays are
  MixMusic's own, which no device can open, and YouTube's streams need the
  proxy, which fetches them in chunks, waits for YouTube to serve them, and
  serves downloads and the cache first. `castServer` serves the proxy's
  `/yt/` paths, only while casting, at a port of its own on the address
  that reaches the device, to that device's address alone, under a random
  128-bit token. It serves the file of a song on this computer, and the
  cover the index keeps of it, at the addresses handed to the device, only
  while that track is the one cast or the one queued after it
  (`castServer.keep`), and no other path. A request from another address
  is turned away, and the log tells it once: which speaker of a group
  fetches the audio is to be checked on a real group, where the one that
  leads it is taken to. A renderer or a receiver fetches the addresses handed to
  it, with a file extension, served as the type it was told, a renderer with
  DLNA's `transferMode.dlna.org` and `contentFeatures.dlna.org` headers
  (`DLNA.ORG_OP=01`) over at most 8 connections at once. A load the device
  fails without fetching anything is told as a firewall's doing.

Not yet: live streams, gapless playback through a Google Cast receiver's
own queue, converting a song beyond a Google Cast device's bounds, lyrics
on screens, a receiver of MixMusic's own, AirPlay, and video.
