package settings

// Home is what the home page keeps of what it told the user.
type Home struct {
	// SkippedYTDLP is the release of yt-dlp the home page told of, which
	// the user put off: it doesn't tell of it again.
	SkippedYTDLP string `json:"skippedYtdlp"`
}
