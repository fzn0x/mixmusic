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
	client  *discord.Client
	appID   string
	trackID string
	playing bool
	start   time.Time // when the track would have started, at its position
	warned  bool
	busy    bool
}

// update tells Discord what plays now, when it changed: another track,
// play or pause, or a seek. A paused or stopped player clears the presence.
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
		p.client, p.appID, p.trackID, p.playing = discord.New(appID), appID, "", false
	}
	playing := np.Track != nil && np.Playing
	start := time.Now().Add(-time.Duration(np.PositionMs) * time.Millisecond)
	trackID := ""
	if np.Track != nil {
		trackID = np.Track.ID
	}
	drift := start.Sub(p.start)
	if trackID == p.trackID && playing == p.playing && (!playing || (drift < presenceSkew && drift > -presenceSkew)) {
		return
	}
	if p.busy {
		return // the next report tries again
	}
	p.trackID, p.playing, p.start, p.busy = trackID, playing, start, true
	client := p.client
	var activity *discord.Activity
	if playing {
		activity = activityOf(*np.Track, start, np.DurationMs)
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

// activityOf is the presence of a track that started at start.
func activityOf(t media.Track, start time.Time, durationMs int) *discord.Activity {
	a := &discord.Activity{
		Type:    discord.Listening,
		Details: t.Title,
		State:   t.ArtistNames(),
		Start:   start,
		// The member list says "Listening to MixMusic", whether the track
		// is Spotify's or YouTube Music's.
		StatusDisplay: discord.ShowName,
	}
	if durationMs == 0 {
		durationMs = t.DurationMs
	}
	if durationMs > 0 {
		a.End = start.Add(time.Duration(durationMs) * time.Millisecond)
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
