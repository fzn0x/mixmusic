package media

// Privacy tells who may open a playlist on its service.
type Privacy string

const (
	// Public is anyone: the playlist shows on its owner's page, their
	// channel on YouTube Music or their profile on Spotify.
	Public Privacy = "public"
	// Unlisted is anyone with the playlist's link, on YouTube Music: it
	// shows on no page of its owner's.
	Unlisted Privacy = "unlisted"
	// Private is its owner alone on YouTube Music. On Spotify, it is a
	// playlist kept off its owner's profile, which anyone with its link
	// may still open.
	Private Privacy = "private"
)
