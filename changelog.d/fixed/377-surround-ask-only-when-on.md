- YouTube Music tracks no longer start up to half a second late while
  "Play 5.1 surround when a track has it" is off, as it is by default:
  MixMusic asks YouTube whether a track has a 5.1 mix only while it's
  on, rather than for every track, and waited for that answer on a slow
  connection. With it off, the 5.1 button shows only for a track whose
  5.1 mix MixMusic knows of already, as one in the offline cache (#377).
