package settings

// The panels of the mini player that it keeps open from one start to the
// next (#371). The panel of a track that couldn't play isn't one of them:
// the player forgets those tracks as MixMusic closes.
const (
	MiniQueue  = "queue"
	MiniLyrics = "lyrics"
	MiniOutput = "output"
)

// cleanMiniPanel is p, when it names a panel the mini player keeps, else
// "", which is none.
func cleanMiniPanel(p string) string {
	switch p {
	case MiniQueue, MiniLyrics, MiniOutput:
		return p
	}
	return ""
}
