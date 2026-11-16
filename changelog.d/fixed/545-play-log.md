- Stats, Your year and the Plays sort count every play, rather than your
  last 1,000: All time said "You played 1,000 tracks" and never more, and
  a year's recap held only its last months. MixMusic keeps each play in
  `plays.jsonl` in your data folder, about 1 MB a year at 10 plays a
  day, and fills it once from the last 1,000 plays of the history. Clear
  history empties it too (#545).
