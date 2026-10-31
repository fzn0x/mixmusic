package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// PlaybackMode is how a track plays.
type PlaybackMode string

const (
	// PlaybackStream plays the track in MixMusic's audio engine, from Url.
	PlaybackStream PlaybackMode = "stream"
	// PlaybackConnect plays the track on a Spotify Connect device.
	PlaybackConnect PlaybackMode = "connect"
	// PlaybackSpotify plays the track in MixMusic, as a Spotify device of
	// its own, with Spotify's Web Playback SDK.
	PlaybackSpotify PlaybackMode = "spotify"
)

// PlaybackSource tells the player how to play a track.
type PlaybackSource struct {
	Mode PlaybackMode `json:"mode"`
	// URL is the audio of a stream.
	URL string `json:"url,omitempty"`
	// Track is the track that plays: for a Spotify track played from
	// YouTube Music, its recording there.
	Track media.Track `json:"track"`
	// Cached is set when the audio plays from the cache, without the
	// network.
	Cached bool `json:"cached,omitempty"`
}

// Player prepares tracks for the player of the main window, controls
// Spotify Connect devices, and relays the player's state and commands
// between windows.
type Player struct{ a *App }

// Prepare finds how to play a track: the URL of its audio, found ahead so
// that errors such as a missing yt-dlp show before playback starts, a
// Spotify Connect device, or Spotify in MixMusic.
func (p *Player) Prepare(ctx context.Context, track media.Track) (PlaybackSource, error) {
	if track.Source == media.SourceSpotify {
		switch p.a.settings.Get().SpotifyPlayback {
		case settings.SpotifyPlaybackConnect:
			return PlaybackSource{Mode: PlaybackConnect, Track: track}, nil
		case settings.SpotifyPlaybackHere:
			return PlaybackSource{Mode: PlaybackSpotify, Track: track}, nil
		}
	}
	return p.PrepareStream(ctx, track)
}

// PrepareStream finds the audio of a track to play in the engine, whatever
// the settings say of Spotify tracks: those play from their recording on
// YouTube Music. The player falls back to it when no Spotify device can
// play.
func (p *Player) PrepareStream(ctx context.Context, track media.Track) (PlaybackSource, error) {
	yt, err := p.a.matcher.Match(ctx, track)
	if err != nil {
		return PlaybackSource{}, err
	}
	_, videoID, err := media.ParseID(yt.ID)
	if err != nil {
		return PlaybackSource{}, err
	}
	src := PlaybackSource{Mode: PlaybackStream, URL: schemeURL(streamScheme) + strings.TrimPrefix(stream.Path(videoID), "/"), Track: yt}
	if p.a.cache.Has(videoID) {
		src.Cached = true
		return src, nil
	}
	if _, err := p.a.resolver.Resolve(ctx, videoID); err != nil {
		return PlaybackSource{}, err
	}
	return src, nil
}

// StreamFailure tells why the audio of a track, the recording PrepareStream
// returned, failed to load: in plain words, or "" when its audio arrived,
// and the engine could not play it.
func (p *Player) StreamFailure(track media.Track) string {
	_, videoID, err := media.ParseID(track.ID)
	if err != nil {
		return ""
	}
	return p.a.proxy.Failure(videoID)
}

// ForgetMatch drops the YouTube Music recording found for a Spotify track,
// so that the next play searches again.
func (p *Player) ForgetMatch(trackID string) error {
	return p.a.matcher.Forget(trackID)
}

// Report tells the app what the player plays, for the tray,
// notifications, the system's media controls, Discord and Last.fm.
func (p *Player) Report(np NowPlaying) {
	p.a.mu.Lock()
	prev := p.a.nowPlaying
	p.a.nowPlaying = np
	tray := p.a.tray
	main := p.a.main
	bar := p.a.taskbar
	p.a.mu.Unlock()
	s := p.a.settings.Get()
	p.a.scrobble(np)
	p.a.presence.update(s.DiscordPresence, s.DiscordApp(), np)
	p.a.reportControls(np)
	if bar != nil {
		bar.Show(np.Playing, np.Track != nil)
	}

	changed := np.Track != nil && (prev.Track == nil || prev.Track.ID != np.Track.ID)
	if tray != nil && (changed || prev.Playing != np.Playing) {
		tray.SetToolTip(trayTip(np))
	}
	if changed && np.Playing && p.a.settings.Get().Notifications && (main == nil || !main.IsFocused()) {
		go p.a.notifyTrack(*np.Track)
	}
}

// command has the player do what the tray, the media keys, the system's
// media controls or a deep link ask.
func (a *App) command(cmd PlayerCommand) {
	if p := a.goPlayer.Load(); p != nil {
		a.commandGoPlayer(p, cmd)
	}
}

// trayTip is the tray's tooltip for what plays.
func trayTip(np NowPlaying) string {
	if np.Track == nil {
		return "MixMusic"
	}
	tip := np.Track.Title + " — " + np.Track.ArtistNames()
	if !np.Playing {
		tip = "Paused: " + tip
	}
	// Windows cuts tooltips at 127 characters.
	if r := []rune(tip); len(r) > 120 {
		tip = string(r[:119]) + "…"
	}
	return tip
}

// notifyTrack shows a notification of the track that started.
func (a *App) notifyTrack(t media.Track) {
	body := t.ArtistNames()
	if t.Album != nil && t.Album.Name != "" {
		body += " • " + t.Album.Name
	}
	n := mygo.NewNotification(mygo.NotificationOptions{ID: "now-playing", Title: t.Title, Body: body, Silent: true})
	n.OnClick(a.showMain)
	if err := n.Show(); err != nil && !errors.Is(err, mygo.ErrNotificationsDenied) {
		log.Printf("mixmusic: notification: %v", err)
	}
}

// Devices returns the user's Spotify Connect devices.
func (p *Player) Devices(ctx context.Context) ([]spotify.Device, error) {
	return p.a.spotify.Devices(ctx)
}

// ConnectPlay plays a Spotify track on the Spotify Connect device of the
// settings, or the active one, from positionMs.
func (p *Player) ConnectPlay(ctx context.Context, track media.Track, positionMs int) error {
	_, id, err := media.ParseID(track.ID)
	if err != nil || track.Source != media.SourceSpotify {
		return errors.New("only Spotify tracks play on Spotify Connect")
	}
	device := p.a.settings.Get().SpotifyDevice
	err = p.a.spotify.Play(ctx, device, []string{spotify.URI("track", id)}, positionMs)
	var apiErr *spotify.APIError
	if errors.As(err, &apiErr) && apiErr.Status == 404 && device == "" {
		// No device is active: wake up the first one that takes commands.
		devices, derr := p.a.spotify.Devices(ctx)
		if derr != nil {
			return err
		}
		for _, d := range devices {
			if !d.Restricted {
				return p.a.spotify.Play(ctx, d.ID, []string{spotify.URI("track", id)}, positionMs)
			}
		}
	}
	return err
}

// playHereRetry is how long PlayHere waits before asking Spotify again.
var playHereRetry = time.Second

// PlayHere plays a Spotify track from positionMs on the device of the Web
// Playback SDK, MixMusic's own, by its ID. Spotify may not know a device
// that just connected for a moment, so it asks again a few times.
func (p *Player) PlayHere(ctx context.Context, deviceID string, track media.Track, positionMs int) error {
	_, id, err := media.ParseID(track.ID)
	if err != nil || track.Source != media.SourceSpotify {
		return errors.New("only Spotify tracks play with Spotify")
	}
	for try := 0; ; try++ {
		err = p.a.spotify.Play(ctx, deviceID, []string{spotify.URI("track", id)}, positionMs)
		var apiErr *spotify.APIError
		if try == 4 || !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
			return err
		}
		select {
		case <-time.After(playHereRetry):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// ConnectPause pauses the Spotify Connect device.
func (p *Player) ConnectPause(ctx context.Context) error {
	return p.a.spotify.Pause(ctx, p.a.settings.Get().SpotifyDevice)
}

// ConnectResume resumes the Spotify Connect device.
func (p *Player) ConnectResume(ctx context.Context) error {
	return p.a.spotify.Resume(ctx, p.a.settings.Get().SpotifyDevice)
}

// ConnectSeek moves the Spotify Connect device to positionMs.
func (p *Player) ConnectSeek(ctx context.Context, positionMs int) error {
	return p.a.spotify.Seek(ctx, p.a.settings.Get().SpotifyDevice, positionMs)
}

// ConnectVolume sets the volume of the Spotify Connect device, from 0 to 1.
func (p *Player) ConnectVolume(ctx context.Context, volume float64) error {
	return p.a.spotify.SetVolume(ctx, p.a.settings.Get().SpotifyDevice, int(volume*100+0.5))
}

// ConnectState returns what the Spotify Connect device plays, or nil when
// no device is active.
func (p *Player) ConnectState(ctx context.Context) (*spotify.PlaybackState, error) {
	return p.a.spotify.State(ctx)
}
