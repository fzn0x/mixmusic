package player

import (
	"errors"
	"iter"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// The tracks that couldn't play (#351): the player keeps what it tried of
// each, and why each try failed, as it heard, for the details of the
// track's notice and of Settings › About, until MixMusic closes.

// keptFailures is how many failures the player keeps: the latest.
const keptFailures = 10

// A Failure is a track that couldn't play.
type Failure struct {
	// ID tells it apart from the others: the notice of it names it.
	ID int
	// At is when it failed.
	At time.Time
	// Track is the track that couldn't play, and Recording the one that
	// was to play in its place, when another: a Spotify track's recording
	// on YouTube Music, once it was found. Its ID is "" otherwise.
	Track, Recording media.Track
	// Steps are what the player tried, in turn: the last one failed it.
	Steps []Step
	// Stopped is set when the player stopped after it, as maxFailures
	// tracks in a row couldn't play.
	Stopped bool
}

// A Step is a way the player tried to play a track, and why it failed.
type Step struct {
	// What names the way, as "the stream", "yt-dlp", "the file" or
	// "Spotify Connect".
	What string
	// Why is what went wrong, in the words of its error.
	Why string
	// Times is how many times in a row it failed so: 1 but for the tries
	// of a stream again.
	Times int
}

// stepped is an error that tells the steps it came of, in turn: what each
// tried, and why it failed, the last its own. The resolver's error is one
// when yt-dlp failed after YouTube's player API.
type stepped interface {
	error
	Steps() iter.Seq2[string, string]
}

// failing is what the player keeps of the tracks that couldn't play.
type failing struct {
	// tries are what the player tried of the current entry since it
	// loaded, and awaited the error of the work whose answer the player
	// takes in, while it does (await): they belong to its goroutine.
	tries   tries
	awaited error

	mu   sync.Mutex
	kept []Failure // the latest, oldest first
	last int       // the ID of the latest
}

// tries are the steps tried of an entry, as it loaded with a token: told
// is the error given last, which the steps may tell in parts.
type tries struct {
	token int
	uid   string
	steps []Step
	told  string
}

// Failures returns the tracks that couldn't play since the player was
// made, the latest first: keptFailures of them at most.
func (p *Player) Failures() []Failure {
	f := &p.failed
	f.mu.Lock()
	defer f.mu.Unlock()
	out := slices.Clone(f.kept)
	slices.Reverse(out)
	return out
}

// FailureOf returns the failure of the ID a notice names, while the
// player keeps it.
func (p *Player) FailureOf(id int) (Failure, bool) {
	f := &p.failed
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range f.kept {
		if k.ID == id {
			return k, true
		}
	}
	return Failure{}, false
}

// tried keeps a step the player tried of the current entry: what, and why
// it failed. An error the player awaited that tells its own steps, as the
// resolver's, gives them in its place, and yt-dlp's error names it.
func (p *Player) tried(what, why string) {
	cur, _ := p.s.Current()
	t := &p.failed.tries
	if t.token != p.loadToken || t.uid != cur.UID {
		*t = tries{token: p.loadToken, uid: cur.UID}
	}
	t.told = why
	var st stepped
	if e := p.failed.awaited; e != nil && e.Error() == why && errors.As(e, &st) {
		for what, why := range st.Steps() {
			t.add(what, why)
		}
		return
	}
	if rest, ok := strings.CutPrefix(why, "yt-dlp: "); ok {
		what, why = "yt-dlp", rest
	}
	t.add(what, why)
}

// add keeps a step tried: as one more time of the last, when it is the
// same.
func (t *tries) add(what, why string) {
	if n := len(t.steps); n > 0 && t.steps[n-1].What == what && t.steps[n-1].Why == why {
		t.steps[n-1].Times++
		return
	}
	t.steps = append(t.steps, Step{What: what, Why: why, Times: 1})
}

// keepFailure keeps the current entry as a track that couldn't play, for
// message, and returns the ID its notice names: what it tried, with
// message as the last step's error unless that step told it already.
func (p *Player) keepFailure(message string, stopped bool) int {
	cur, ok := p.s.Current()
	if !ok {
		return 0
	}
	if t := p.failed.tries; t.token != p.loadToken || t.uid != cur.UID || t.told != message {
		p.tried(p.way(cur), message)
	}
	k := Failure{At: time.Now(), Track: cur.Track, Steps: slices.Clone(p.failed.tries.steps), Stopped: stopped}
	if src := p.s.Source; src != nil && src.Track.ID != "" && src.Track.ID != cur.Track.ID {
		k.Recording = src.Track
	}
	f := &p.failed
	f.mu.Lock()
	defer f.mu.Unlock()
	f.last++
	k.ID = f.last
	f.kept = append(f.kept, k)
	if len(f.kept) > keptFailures {
		f.kept = slices.Delete(f.kept, 0, len(f.kept)-keptFailures)
	}
	return k.ID
}

// way names how the current entry was to play: the step whose error
// failed it, when no other step named itself.
func (p *Player) way(cur Item) string {
	src := p.s.Source
	switch {
	case cur.Track.Source == media.SourceFile:
		return "the file"
	case src == nil && cur.Track.Source == media.SourceSpotify:
		return "finding it on YouTube Music"
	case src == nil:
		return "finding its stream"
	case src.Mode == ModeConnect:
		return "Spotify Connect"
	case src.Mode == ModeSpotify:
		return "Spotify in MixMusic"
	case src.Live:
		return "the live stream"
	case p.playsOffline():
		return "the download, or the offline cache"
	}
	return "the stream"
}

// spotifyWay names the way the settings play Spotify tracks, which failed
// as the player fell back to YouTube Music.
func (p *Player) spotifyWay() string {
	if p.spotifyMode == settings.SpotifyPlaybackConnect {
		return "Spotify Connect"
	}
	return "Spotify in MixMusic"
}

// again names a way tried once more, after n tries that failed.
func again(what string, n int) string {
	if n == 0 {
		return what
	}
	return what + ", tried again"
}
