- The volume slider has even steps of loudness: each tenth of it is 6 dB,
  and one press of up or down is about 3 dB at every level down to -40 dB,
  where it used to jump 6 dB or more near the quiet end. Your volume sounds
  as before; the slider shows it further right. Spotify played in MixMusic
  follows the same curve, and Spotify Connect devices get the same
  percentage as before for the same loudness. A plugin that sets a fixed
  volume with `api.player.setVolume()` gets that position's new loudness
  (#82).
