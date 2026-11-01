package app

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/volume"
)

// hereLoad is how long the page of MixMusic's own Spotify device may take
// to load.
const hereLoad = 20 * time.Second

// spotifyPlaysHere tells whether MixMusic plays Spotify itself: where its
// webview, WebView2, decrypts Spotify's audio.
const spotifyPlaysHere = runtime.GOOS == "windows"

// spotifyHere is MixMusic's own Spotify device for the native interface:
// Spotify's Web Playback SDK, which plays Spotify's protected audio, in the
// page of a hidden window, public/spotify.html, as the web interface played
// it in its own page. Tracks play on it through the Web API, as on Spotify
// Connect, and the player follows it by asking the page. The window opens
// at the first Spotify track that plays in MixMusic, and closes when the
// sign-in or the settings change: a WebView2 takes memory.
type spotifyHere struct {
	a *App

	mu sync.Mutex
	// win is the window, and loaded is closed once its page is ready.
	win    *mygo.Window
	loaded chan struct{}
}

// page returns the page of the device, opening its window if need be.
func (h *spotifyHere) page(ctx context.Context) (*mygo.Page, error) {
	if err := needWebView("playing Spotify in MixMusic"); err != nil {
		return nil, err
	}
	h.mu.Lock()
	if h.win == nil {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:       "MixMusic's Spotify player",
			Width:       320,
			Height:      240,
			Hidden:      true,
			SkipTaskbar: true,
			URL:         "/spotify.html",
		})
		loaded := make(chan struct{})
		h.win, h.loaded = win, loaded
		win.OnClosed(func() {
			h.mu.Lock()
			if h.win == win {
				h.win, h.loaded = nil, nil
			}
			h.mu.Unlock()
		})
		go waitLoaded(win, loaded)
	}
	win, loaded := h.win, h.loaded
	h.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, hereLoad)
	defer cancel()
	select {
	case <-loaded:
		return win.Page(), nil
	case <-ctx.Done():
		return nil, errors.New("MixMusic's Spotify player didn't load")
	}
}

// waitLoaded closes loaded once the page of win defines its device.
func waitLoaded(win *mygo.Window, loaded chan struct{}) {
	for deadline := time.Now().Add(hereLoad); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if ok, _ := mygo.EvalAs[bool](win.Page(), "return typeof mm === 'object'"); ok {
			close(loaded)
			return
		}
	}
}

// open returns the page of the device, or nil while it isn't open: a
// device that never played is paused.
func (h *spotifyHere) open() *mygo.Page {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.win == nil {
		return nil
	}
	select {
	case <-h.loaded:
		return h.win.Page()
	default:
		return nil
	}
}

// close closes the window: MixMusic leaves the user's Spotify devices.
func (h *spotifyHere) close() {
	h.mu.Lock()
	win := h.win
	h.win, h.loaded = nil, nil
	h.mu.Unlock()
	if win != nil {
		win.Close()
	}
}

// eval runs code in the page and decodes what it returns; what it throws
// is the page's own error, as it words it.
func eval[T any](ctx context.Context, page *mygo.Page, code string) (T, error) {
	var out T
	v, err := page.EvalContext(ctx, code)
	if err != nil {
		var evalErr *mygo.EvalError
		if errors.As(err, &evalErr) {
			return out, errors.New(strings.TrimPrefix(evalErr.Message, "Error: "))
		}
		return out, err
	}
	data, err := json.Marshal(v)
	if err == nil {
		err = json.Unmarshal(data, &out)
	}
	return out, err
}

// SpotifyPage is what the page of MixMusic's own Spotify device calls.
type SpotifyPage struct{ a *App }

// Token returns the access token of the user's Spotify sign-in, with which
// Spotify's player plays.
func (s *SpotifyPage) Token(ctx context.Context) (string, error) { return s.a.spotify.AccessToken(ctx) }

// SpotifyHereCheck tells, for development, whether MixMusic's own
// Spotify device could play: its page loads, its webview decrypts
// Spotify's audio, and it reaches the app for a token, which it gets once
// the user signed in.
func (a *App) SpotifyHereCheck(ctx context.Context) (string, error) {
	page, err := a.here.page(ctx)
	if err != nil {
		return "", err
	}
	defer a.here.close()
	return eval[string](ctx, page, `
		const supported = await mm.supported();
		const token = await mygo.call("SpotifyPage.Token").then(() => "a token", (e) => "no token: " + e.message);
		return "decrypts Spotify's audio: " + supported + ", " + token;`)
}

// hereDevice is the player's view of MixMusic's own Spotify device.
type hereDevice struct{ h *spotifyHere }

func (d hereDevice) Play(ctx context.Context, t media.Track, at time.Duration) error {
	switch a := d.h.a; {
	case !a.spotify.SignedIn():
		return errors.New("sign in to Spotify in Settings › Accounts to play it")
	case !a.spotify.Granted("streaming"):
		return errors.New("sign in to Spotify again in Settings › Accounts, for MixMusic to play it itself")
	}
	page, err := d.h.page(ctx)
	if err != nil {
		return err
	}
	// The device plays at the player's volume, which the player sends it
	// only while it plays: from its start, and again once it plays, as
	// what the user set while the track loaded didn't reach it.
	if _, err := page.EvalContext(ctx, "await mm.setVolume("+d.level()+")"); err != nil {
		return err
	}
	id, err := eval[string](ctx, page, "return await mm.device()")
	if err != nil {
		return err
	}
	if err := (&Player{d.h.a}).PlayHere(ctx, id, t, int(at/time.Millisecond)); err != nil {
		return err
	}
	_, err = page.EvalContext(ctx, "await mm.setVolume("+d.level()+")")
	return err
}

// level is the gain of the player's volume, 0 when muted, for the page.
func (d hereDevice) level() string {
	v := volume.Of(0.8)
	if p := d.h.a.goPlayer.Load(); p != nil {
		st := p.State()
		v = st.Volume
		if st.Muted {
			v = 0
		}
	}
	return gain(v)
}

// gain is the gain of the volume v for the page, the gain MixMusic's own
// player plays it at, so that both sound alike.
func gain(v float64) string { return strconv.FormatFloat(volume.Gain(v), 'f', -1, 64) }

// do runs a command of the device in its page, if it's open.
func (d hereDevice) do(ctx context.Context, code string) error {
	page := d.h.open()
	if page == nil {
		return nil
	}
	_, err := page.EvalContext(ctx, code)
	return err
}

func (d hereDevice) Pause(ctx context.Context) error  { return d.do(ctx, "await mm.pause()") }
func (d hereDevice) Resume(ctx context.Context) error { return d.do(ctx, "await mm.resume()") }

func (d hereDevice) Seek(ctx context.Context, at time.Duration) error {
	return d.do(ctx, "await mm.seek("+strconv.FormatInt(at.Milliseconds(), 10)+")")
}

func (d hereDevice) SetVolume(ctx context.Context, v float64) error {
	return d.do(ctx, "await mm.setVolume("+gain(v)+")")
}

// hereState is what the page tells the device plays.
type hereState struct {
	Playing    bool         `json:"playing"`
	ProgressMs int          `json:"progressMs"`
	Track      *media.Track `json:"track"`
}

func (d hereDevice) State(ctx context.Context) (*player.DeviceState, error) {
	page := d.h.open()
	if page == nil {
		return nil, nil
	}
	st, err := eval[*hereState](ctx, page, "return await mm.state()")
	if err != nil || st == nil {
		return nil, err
	}
	if st.Track != nil {
		st.Track.Source = media.SourceSpotify
	}
	return &player.DeviceState{Playing: st.Playing, Progress: time.Duration(st.ProgressMs) * time.Millisecond, Track: st.Track}, nil
}

// AskEvery is how often the player asks the device what it plays: its
// page answers at once.
func (d hereDevice) AskEvery() time.Duration { return time.Second }
