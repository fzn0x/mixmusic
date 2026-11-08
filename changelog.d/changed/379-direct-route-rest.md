- When YouTube's player API refuses three tracks in a row, MixMusic asks
  for the third once more with a new visitor ID, then leaves it alone for
  2 minutes rather than 30, and twice as long each time it's refused
  again, up to 30 minutes: tracks found by the slower yt-dlp meanwhile
  were a delay that lasted half an hour. A track the player API doesn't
  answer for in time counts as a refusal, and the next waits 2 seconds
  for it rather than 4. The log says how long it rests (#379).
