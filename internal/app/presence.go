package app

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/discord"
	"github.com/fzn0x/mixmusic/internal/media"
)

// presenceSkew is how far the position may drift from what Discord was
// told before MixMusic tells it again, as after a seek.
const presenceSkew = 3 * time.Second

// presence shows what plays on the user's Discord profile.
type presence struct {
	mu      sync.Mutex
	client  presenceClient
	appID   string
	trackID string
	playing bool
	start   time.Time // when the track would have started, at its position
	speed   float64
	warned  bool
	busy    bool
}

// presenceClient is what presence tells Discord through: a
// discord.Client.
type presenceClient interface {
	SetActivity(ctx context.Context, a discord.Activity) error
	Clear(ctx context.Context) error
	Close() error
}

// newPresenceClient connects to Discord as the app appID.
var newPresenceClient = func(appID string) presenceClient { return discord.New(appID) }

// update tells Discord what plays now, when it changed: another track,
// play or pause, or a seek. A paused or stopped player clears the presence;
// a track that loads plays (NowPlaying.plays), so that the presence goes
// from one track to the next without clearing between them.
func (p *presence) update(on bool, appID string, np NowPlaying) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !on || appID == "" {
		if p.client != nil {
			p.client.Close() // Discord clears the presence of a closed connection
			p.client, p.trackID, p.playing = nil, "", false
		}
		return
	}
	if p.client == nil || p.appID != appID {
		if p.client != nil {
			p.client.Close()
		}
		p.client, p.appID, p.trackID, p.playing = newPresenceClient(appID), appID, "", false
	}
	playing := np.plays()
	// Discord's times are the clock's: at 2×, a track 1 minute in started
	// half a minute ago, and ends in half the time left.
	speed := np.speed()
	start := time.Now().Add(-time.Duration(float64(np.PositionMs) * float64(time.Millisecond) / speed))
	trackID := ""
	if np.Track != nil {
		trackID = np.Track.ID
	}
	drift := start.Sub(p.start)
	if trackID == p.trackID && playing == p.playing && (!playing || (speed == p.speed && drift < presenceSkew && drift > -presenceSkew)) {
		return
	}
	if p.busy {
		return // the next report tries again
	}
	p.trackID, p.playing, p.start, p.speed, p.busy = trackID, playing, start, speed, true
	client := p.client
	var activity *discord.Activity
	if playing {
		activity = activityOf(*np.Track, start, np.DurationMs, speed, np.Live)
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		if activity != nil {
			err = client.SetActivity(ctx, *activity)
		} else {
			err = client.Clear(ctx)
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		p.busy = false
		if err != nil {
			// Try again with the next report.
			p.trackID, p.playing = "", false
			if !errors.Is(err, discord.ErrNotRunning) && !p.warned {
				p.warned = true
				log.Printf("mixmusic: Discord presence: %v", err)
			}
		}
	}()
}

// activityOf is the presence of a track that started at start, of
// durationMs, or the catalog's length when 0, which plays at speed: it
// ends after its duration over the speed. A live stream has none: Discord
// tells how long it played.
func activityOf(t media.Track, start time.Time, durationMs int, speed float64, live bool) *discord.Activity {
	a := &discord.Activity{
		Type:    discord.Listening,
		Details: t.Title,
		State:   t.ArtistNames(),
		Start:   start,
		// The member list says "Listening to MixMusic", whether the track
		// is Spotify's or YouTube Music's.
		StatusDisplay: discord.ShowName,
	}
	switch {
	case live:
		durationMs = 0
	case durationMs == 0:
		durationMs = t.DurationMs
	}
	if durationMs > 0 {
		a.End = start.Add(time.Duration(float64(durationMs) * float64(time.Millisecond) / speed))
	}
	if t.Album != nil {
		a.LargeText = t.Album.Name
	}
	a.LargeImage = largeArtwork(t.Artwork)
	return a
}

// largeArtwork picks the artwork Discord and the system's media controls
// show: an https image of about 512 pixels.
func largeArtwork(images []media.Image) string {
	best := ""
	bestWidth := 0
	for _, img := range images {
		if !strings.HasPrefix(img.URL, "https://") {
			continue
		}
		if best == "" || (bestWidth < 512 && img.Width > bestWidth) || (img.Width >= 512 && img.Width < bestWidth) {
			best, bestWidth = img.URL, img.Width
		}
	}
	return best
}
