- Installing or updating yt-dlp tries again when GitHub fails the
  download for a while, with a 502, a 504 or a download broken off: a
  second later, then twice as long after each failure, for up to two
  minutes, rather than failing at once. The progress starts again from
  zero with each try (#538).
