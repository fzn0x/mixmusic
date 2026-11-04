package youtube

import (
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// readProgress keeps on an episode where YouTube Music says the user left
// it, from the musicPlaybackProgressRenderer of its row. Signed in, the
// renderer holds how far they got, as playbackProgressPercentage from 0 to
// 100 of the episode's length, with what the row shows in
// playbackProgressText: its length, or "Played" once they heard it to its
// end, as the percentage turns 100. Its playedText is there either way,
// for the row to show once it's played. Signed out, the percentage is 0
// and nothing else tells: that is no place, and keeps none.
func readProgress(t *media.Track, progress node) {
	if t.Episode == nil || !progress.exists() {
		return
	}
	pct := progress.get("playbackProgressPercentage").number()
	shown := progress.get("playbackProgressText")
	if !shown.exists() && pct <= 0 {
		return
	}
	switch {
	case pct >= 100 || saysPlayed(shown):
		t.Episode.ResumeMs, t.Episode.Played = 0, true
	case pct > 0 && t.DurationMs <= 0:
		return // a share of a length not known
	case pct > 0:
		t.Episode.ResumeMs, t.Episode.Played = int(int64(t.DurationMs)*int64(pct)/100), false
	default:
		t.Episode.ResumeMs, t.Episode.Played = 0, false
	}
	t.Episode.ToldAt = today()
}

// saysPlayed reports whether a row's text tells its episode was played.
func saysPlayed(text node) bool {
	for _, r := range text.runs() {
		if strings.EqualFold(strings.TrimSpace(r.get("text").str()), "played") {
			return true
		}
	}
	return false
}
