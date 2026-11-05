- On macOS, the yt-dlp MixMusic installs starts in 0.2 seconds instead of
  8: MixMusic installs yt-dlp's unpacked build, which doesn't unpack Python
  again at every start. A track that only yt-dlp can play starts that much
  sooner, and so does Settings › Playback › Tools. The yt-dlp installed
  before is replaced a minute after MixMusic starts, at its update check,
  whatever its version. It takes about 120 MB on disk instead of 37 (#245).
