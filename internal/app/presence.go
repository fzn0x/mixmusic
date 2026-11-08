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
	"github.com/fzn0x/mixmusic/internal/settings"
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
	look    presenceLook
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

// update tells Discord what plays now, as d chooses, when it changed:
// another track, play or pause, a seek, or what d chooses. A paused or
// stopped player clears the presence, unless d keeps a paused track; a
// track that loads plays (NowPlaying.plays), so that the presence goes
// from one track to the next without clearing between them.
func (p *presence) update(d settings.Desktop, np NowPlaying) {
	p.mu.Lock()
	defer p.mu.Unlock()
	appID := d.DiscordApp()
	if !d.DiscordPresence || appID == "" {
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
	now := time.Now()
	playing, speed, start, look := np.plays(), np.speed(), presenceStart(np, now), presenceLookOf(d)
	trackID := ""
	if np.Track != nil {
		trackID = np.Track.ID
	}
	drift := start.Sub(p.start)
	if trackID == p.trackID && playing == p.playing && look == p.look &&
		(!playing || (speed == p.speed && drift < presenceSkew && drift > -presenceSkew)) {
		return
	}
	if p.busy {
		return // the next report tries again
	}
	p.trackID, p.playing, p.start, p.speed, p.look, p.busy = trackID, playing, start, speed, look, true
	client := p.client
	activity := DiscordActivity(d, np, now)
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

// presenceLook is what Settings › Desktop chooses that the presence shows,
// which, once it changes, shows what plays again.
type presenceLook struct {
	first, second                              string
	cover, badge, timeLeft, listen, keptPaused bool
}

// presenceLookOf is what d chooses that the presence shows.
func presenceLookOf(d settings.Desktop) presenceLook {
	return presenceLook{d.DiscordFirstLine, d.DiscordSecondLine,
		d.DiscordCover, d.DiscordServiceBadge, d.DiscordTimeLeft, d.DiscordListenButton, d.DiscordKeepWhilePaused}
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
