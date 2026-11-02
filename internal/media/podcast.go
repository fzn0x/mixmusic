package media

// ItemPodcast is the kind of an Item that holds a podcast.
const ItemPodcast ItemKind = "podcast"

// Episode is what a podcast's episode has besides a track's. The track's
// artists name its show, so that what tells what plays names it too.
type Episode struct {
	// Show is the podcast the episode is of, with its ID when the service
	// tells it.
	Show Ref `json:"show"`
	// Released is the day the episode came out, as 2026-10-30, or only
	// its month or its year, as 2026-10 or 2026, when that is all the
	// service tells.
	Released string `json:"released,omitempty"`
	// ResumeMs is where the user left the episode on its service: only
	// Spotify tells, to a sign-in that lets MixMusic read it.
	ResumeMs int `json:"resumeMs,omitempty"`
	// Played is set once the user heard the episode to its end, as its
	// service tells.
	Played bool `json:"played,omitempty"`
}

// IsEpisode reports whether the track is a podcast's episode.
func (t Track) IsEpisode() bool { return t.Episode != nil }

// Podcast is a show of episodes. Lists of podcasts leave Episodes empty.
type Podcast struct {
	ID     string `json:"id"`
	Source Source `json:"source"`
	Title  string `json:"title"`
	// Authors are who makes the show, as YouTube Music links its channel:
	// Spotify has told apps no publisher since February 2026.
	Authors     []Ref   `json:"authors"`
	Description string  `json:"description,omitempty"`
	Artwork     []Image `json:"artwork"`
	// EpisodeCount is how many episodes the show has, when its service
	// says.
	EpisodeCount int `json:"episodeCount,omitempty"`
	// Episodes are the show's newest episodes, a page of them, the newest
	// first.
	Episodes []Track `json:"episodes"`
	// More reads the page of episodes after Episodes, "" after the last.
	More string `json:"more,omitempty"`
	URL  string `json:"url,omitempty"`
	// PlaylistID is YouTube Music's playlist of the show's episodes, which
	// saving the show saves.
	PlaylistID string `json:"playlistId,omitempty"`
	// Saved is set when the show is in the user's library of its service;
	// only a podcast page says so.
	Saved bool `json:"saved,omitempty"`
}

// Summary returns the podcast as lists keep it: without its episodes.
func (p Podcast) Summary() Podcast {
	p.Episodes, p.More, p.Saved = []Track{}, "", false
	return p
}

// EpisodePage is a page of a podcast's episodes, and what reads the page
// after it, "" after the last.
type EpisodePage struct {
	Episodes []Track `json:"episodes"`
	More     string  `json:"more,omitempty"`
}

// PodcastResults holds the podcasts and the episodes a search found.
type PodcastResults struct {
	Podcasts []Podcast `json:"podcasts"`
	Episodes []Track   `json:"episodes"`
}

// EmptyPodcastResults returns results that found nothing, whose lists
// encode as empty, not null.
func EmptyPodcastResults() PodcastResults {
	return PodcastResults{Podcasts: []Podcast{}, Episodes: []Track{}}
}
