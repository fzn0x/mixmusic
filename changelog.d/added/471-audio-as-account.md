- Play audio as your account, in Settings › Playback › Sound beside
  Quality, and off at first: while you're signed in to YouTube Music,
  MixMusic asks yt-dlp for each track's audio as your account, so a
  YouTube Music Premium account hears Premium's higher bitrate, Opus of
  format 774 or AAC-LC of 141, near 256 kbit/s. YouTube may flag an
  account used this way, and tracks may start more slowly, as the switch
  says. A track it can't get this way, or whose audio YouTube refuses,
  plays as before, found signed out. When MixMusic sees your account has
  Premium, it offers the switch once, and never turns it on by itself.
  It's kept as `audioAsAccount` in settings.json (#471).
