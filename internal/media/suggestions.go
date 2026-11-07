package media

// Suggestions is what a service suggests for what the user typed so far:
// searches to run, and songs, albums, artists and playlists to play or
// open at once.
type Suggestions struct {
	Searches []string `json:"searches"`
	Items    []Item   `json:"items,omitempty"`
}
