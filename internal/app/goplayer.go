package app

// The player: the queue played in Go (internal/player), through
// MixMusic's own audio engine and Spotify's devices, with the services of
// Player.

import (
	"context"
	"log"
	"path/filepath"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/tools"
)

// GoPlayer returns the native interface's player, or nil while the web
// interface plays.
func (a *App) GoPlayer() *player.Player { return a.goPlayer.Load() }

// startGoPlayer makes the native interface's player, of the queue kept
// from the last run, which plays nothing until asked.
func (a *App) startGoPlayer() (*player.Player, error) {
	if p := a.goPlayer.Load(); p != nil {
		return p, nil
	}
	e := a.nativeEngine()
	if e == nil {
		return nil, errNoNative
	}
	queue, err := store.Open(filepath.Join(a.opts.DataDir, "queue.json"), player.Saved{})
	if err != nil {
		return nil, err
	}
	a.queue = queue
	p := player.New(playerServices{a}, engineStream{a, e}, connectDevice{a}, queue.Get())
	if spotifyPlaysHere {
		p.SpotifyHere(hereDevice{&a.here})
	}
	l := newListener()
	a.listen.Store(l)
	p.Subscribe(func(player.Event) { l.follow(p.State()) })
	a.goPlayer.Store(p)
	a.applyEngine(a.settings.Get().Clean())
	return p, nil
}

// applyEngine gives the engine the sound of the settings: the equalizer,
// the levelling and Even loudness.
func (a *App) applyEngine(s settings.Settings) {
	e := a.nativeEngine()
	if e == nil {
		return
	}
	var gains [len(audio.Bands)]float64
	copy(gains[:], s.Equalizer.Gains)
	e.SetEqualizer(s.Equalizer.Enabled, gains, s.Equalizer.Preamp)
	e.SetNormalize(s.Normalize)
	e.SetLoudness(s.Loudness)
}

// commandGoPlayer has the native interface's player do what the tray, the
// media keys, the mini player or a deep link asks.
func (a *App) commandGoPlayer(p *player.Player, cmd PlayerCommand) {
	switch cmd.Command {
	case CommandToggle:
		p.ToggleOnce()
	case CommandPlay:
		p.Play()
	case CommandPause:
		p.Pause()
	case CommandNext:
		p.Next()
	case CommandPrevious:
		p.Previous()
	case CommandSeek:
		p.Seek(fromMs(cmd.Value))
	case CommandVolume:
		p.SetVolume(cmd.Value)
	case CommandShuffle:
		p.ToggleShuffle()
	case CommandRepeat:
		p.CycleRepeat()
	case CommandLike:
		if cur, ok := p.State().Current(); ok {
			liked := a.library.IsLiked(cur.Track.ID)
			go func() {
				if err := (&Collection{a}).SetLiked(context.Background(), cur.Track, !liked); err != nil {
					log.Printf("mixmusic: liking %s: %v", cur.Track.ID, err)
				}
			}()
		}
	case CommandPlayTrack:
		p.PlayID(cmd.TrackID)
	}
}

// playerServices is the app as the services of the native interface's
// player.
type playerServices struct{ a *App }

func playerSource(src PlaybackSource) player.Source {
	return player.Source{Mode: player.Mode(src.Mode), URL: src.URL, Track: src.Track}
}

func (s playerServices) Prepare(ctx context.Context, t media.Track) (player.Source, error) {
	src, err := (&Player{s.a}).Prepare(ctx, t)
	return playerSource(src), err
}

func (s playerServices) PrepareStream(ctx context.Context, t media.Track) (player.Source, error) {
	src, err := (&Player{s.a}).PrepareStream(ctx, t)
	return playerSource(src), err
}

func (s playerServices) StreamFailure(t media.Track) string { return (&Player{s.a}).StreamFailure(t) }

func (s playerServices) RecordPlay(t media.Track) {
	if err := (&Collection{s.a}).RecordPlay(t); err != nil {
		log.Printf("mixmusic: counting a play of %s: %v", t.ID, err)
	}
}

func (s playerServices) Radio(ctx context.Context, t media.Track) ([]media.Track, error) {
	tracks, err := (&Catalog{s.a}).Radio(ctx, t)
	return tracks, s.a.offlineErr(ctx, err)
}

var _ player.Network = playerServices{}

// Online, WaitOnline and KickOnline are the network, as the player waits
// for it.
func (s playerServices) Online(ctx context.Context) bool      { return s.a.online.Check(ctx) }
func (s playerServices) WaitOnline(ctx context.Context) error { return s.a.online.Wait(ctx) }
func (s playerServices) KickOnline()                          { s.a.online.Kick() }

func (s playerServices) Settings() settings.Settings { return s.a.settings.Get().Clean() }

func (s playerServices) SaveVolume(v float64) {
	st := s.a.settings.Get()
	st.Volume = v
	if _, err := (&Preferences{s.a}).Set(st); err != nil {
		log.Printf("mixmusic: keeping the volume: %v", err)
	}
}

func (s playerServices) Report(now player.Now) {
	np := NowPlaying{
		Track:      now.Track,
		Playing:    now.Playing,
		Loading:    now.Loading,
		PositionMs: int(now.Position / time.Millisecond),
		DurationMs: int(now.Duration / time.Millisecond),
		Volume:     now.Volume,
		Shuffle:    now.Shuffle,
		Repeat:     Repeat(now.Repeat),
		Speed:      now.Speed,
	}
	if now.Track != nil {
		np.Liked = s.a.library.IsLiked(now.Track.ID)
	}
	(&Player{s.a}).Report(np)
}

func (s playerServices) SaveQueue(saved player.Saved) {
	if err := s.a.queue.Update(func(v *player.Saved) error {
		*v = saved
		return nil
	}); err != nil {
		log.Printf("mixmusic: keeping the queue: %v", err)
	}
}

// engineStream is the engine as the player's stream. It tells the
// system's media controls whether the engine has the player's track.
type engineStream struct {
	a *App
	e *audio.Engine
}

func (s engineStream) Load(url string, start time.Duration, autoplay bool, speed float64) error {
	if err := s.e.Load(url, start, autoplay, speed); err != nil {
		return err
	}
	s.a.setNative(true)
	return nil
}

func (s engineStream) Crossfade(url string, d time.Duration, speed float64) error {
	if err := s.e.Crossfade(url, d, speed); err != nil {
		return err
	}
	s.a.setNative(true)
	return nil
}

func (s engineStream) Stop() {
	s.e.Stop()
	s.a.setNative(false)
}

// Unload keeps the media controls: the engine gets the player's next
// track.
func (s engineStream) Unload() { s.e.Unload() }

func (s engineStream) Preload(url string)     { s.e.Preload(url) }
func (s engineStream) Play() error            { return s.e.Play() }
func (s engineStream) Pause()                 { s.e.Pause() }
func (s engineStream) Seek(t time.Duration)   { s.e.Seek(t) }
func (s engineStream) SetVolume(v float64)    { s.e.SetVolume(v) }
func (s engineStream) SetSpeed(speed float64) { s.e.SetSpeed(speed) }

// connectDevice is the Spotify Connect device of the settings, or the
// active one, as the device of the native interface's player.
type connectDevice struct{ a *App }

func (d connectDevice) Play(ctx context.Context, t media.Track, at time.Duration) error {
	return (&Player{d.a}).ConnectPlay(ctx, t, int(at/time.Millisecond))
}

func (d connectDevice) Pause(ctx context.Context) error  { return (&Player{d.a}).ConnectPause(ctx) }
func (d connectDevice) Resume(ctx context.Context) error { return (&Player{d.a}).ConnectResume(ctx) }

func (d connectDevice) Seek(ctx context.Context, at time.Duration) error {
	return (&Player{d.a}).ConnectSeek(ctx, int(at/time.Millisecond))
}

func (d connectDevice) SetVolume(ctx context.Context, v float64) error {
	return (&Player{d.a}).ConnectVolume(ctx, v)
}

func (d connectDevice) State(ctx context.Context) (*player.DeviceState, error) {
	st, err := (&Player{d.a}).ConnectState(ctx)
	if err != nil || st == nil {
		return nil, err
	}
	return &player.DeviceState{
		Playing:  st.Playing,
		Progress: time.Duration(st.ProgressMs) * time.Millisecond,
		Track:    st.Track,
	}, nil
}

// Services are what the windows call.
type Services struct {
	Catalog     *Catalog
	Player      *Player
	Collection  *Collection
	Accounts    *Accounts
	Preferences *Preferences
	Addons      *Addons
	System      *System
}

// Services returns the app's services.
func (a *App) Services() Services {
	return Services{
		Catalog:     &Catalog{a},
		Player:      &Player{a},
		Collection:  &Collection{a},
		Accounts:    &Accounts{a},
		Preferences: &Preferences{a},
		Addons:      &Addons{a},
		System:      &System{a},
	}
}

// IsLiked reports whether a track is among the liked songs.
func (a *App) IsLiked(id string) bool { return a.library.IsLiked(id) }

// InstallYTDLP downloads the latest yt-dlp into the app's data, telling
// progress how much came, for the native interface.
func (a *App) InstallYTDLP(ctx context.Context, progress func(done, total int64)) (tools.Status, error) {
	if err := a.tools.InstallYTDLP(ctx, progress); err != nil {
		return tools.Status{}, err
	}
	return a.tools.Status(ctx), nil
}

// InstallUpdate downloads the newest version, checks its signature and
// puts it in place of this one, telling progress how far its download
// went. It returns the version installed, which runs once MixMusic
// restarts.
func (a *App) InstallUpdate(ctx context.Context, progress func(done, total int64)) (string, error) {
	return a.installUpdate(ctx, progress)
}
