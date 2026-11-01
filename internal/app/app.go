// Package app wires MixMusic together: the services its windows call, the
// player, and its place on the desktop: windows, the tray, media keys and
// notifications.
package app

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/artwork"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/online"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/taskbar"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/lyrics"
)

// artworkLimit bounds the artwork kept on disk, in bytes.
const artworkLimit = 256 << 20

// streamScheme is the scheme of the addresses of the audio PrepareStream
// finds, which the audio engine opens.
const streamScheme = "stream"

// Options configure the app.
type Options struct {
	// DataDir keeps the user's settings, library and add-ons.
	DataDir string
	// CacheDir keeps what can be fetched again: audio, yt-dlp's cache.
	CacheDir string
	// Builtin holds the add-ons MixMusic ships with.
	Builtin fs.FS
	// TrayIcon is the PNG of the tray icon.
	TrayIcon []byte
	// Version is the app's version, for LRCLIB's user agent.
	Version string
	// LogsDir holds the log and the crash reports.
	LogsDir string
	// LastCrash is the crash report of the run before, when it crashed.
	LastCrash string
	// Interface opens the main window, hidden when asked, and Mini the
	// mini player: windows that MyGo draws, of package native.
	Interface func(a *App, hidden bool) *mygo.Window
	Mini      func(a *App) *mygo.Window
}

// App is MixMusic.
type App struct {
	opts Options
	// lastCrash is the crash report of the run before, until the user
	// answers about it.
	lastCrash string

	settings     *store.File[settings.Settings]
	library      *library.Library
	youtube      *youtube.Client
	auth         *spotify.Auth
	spotify      *spotify.Client
	tokens       tokenStore
	lastfmStore  lastfmStore
	youtubeStore *youtubeStore
	tools        *tools.Manager
	resolver     *stream.Resolver
	proxy        *stream.Proxy
	online       *online.Watch
	cache        *stream.Cache
	loudness     *loudnessBook
	matcher      *match.Matcher
	lyrics       *lyrics.Client
	addons       *addons.Manager
	storage      *addons.Storage
	lyricsMemo   *memo[lyrics.Lyrics]
	related      *memo[[]media.Shelf]
	artwork      *artwork.Client
	artStore     *artwork.Store
	colors       *memo[string]

	presence presence
	// native is MixMusic's own audio engine, made at its first use.
	native     *audio.Engine
	nativeOnce sync.Once
	// controls shows the engine's track in the system's media controls.
	controls controls
	// goPlayer is the player, once the main window opened, and queue its
	// queue as kept between runs.
	goPlayer atomic.Pointer[player.Player]
	queue    *store.File[player.Saved]
	// listen hears what the player plays outside the engine, with the
	// player.
	listen atomic.Pointer[listener]
	// here is MixMusic's own Spotify device, which the player plays
	// Spotify tracks on where it can.
	here spotifyHere
	// onNavigate shows the pages of the main window.
	onNavigate *func(target string)
	// updates keeps the user's choices of updates.
	updates *store.File[updatePrefs]
	// checkUpdates has MixMusic look for a new version at once.
	checkUpdates chan struct{}
	// readingProfiles is set while the accounts' profiles are read in the
	// background.
	readingProfiles atomic.Bool
	// updating is held while an update installs.
	updating sync.Mutex
	// quitting is set once MixMusic starts to quit: its windows close then,
	// even with close to the tray on.
	quitting atomic.Bool
	// trayHide hides the main window in the tray, out of full screen first
	// on macOS (#84).
	trayHide trayHide

	mu         sync.Mutex
	installed  string // the version installed to run at the next start
	lastfm     *scrobbles
	youtubeMe  *youtube.Account
	spotifyMe  *spotify.Profile
	nowPlaying NowPlaying
	main, mini *mygo.Window
	// retitling is set while the main window's title waits to change.
	retitling bool
	tray      *mygo.Tray
	// taskbar is the buttons on the thumbnail of the main window's
	// taskbar button, on Windows.
	taskbar   *taskbar.Bar
	mediaKeys bool
	// offer is the new version the main window offers, until the user
	// answers; putOff the one they put off until putOffUntil.
	offer       *Release
	putOff      string
	putOffUntil time.Time
	// profileErrs is why the accounts' profiles could not be read, until
	// they are, or the user signs in or out.
	profileErrs profileErrors
	// flushers save what the windows hold back, as MixMusic quits.
	flushers []*func()
}

// New opens what the app keeps in opts.DataDir and opts.CacheDir.
func New(opts Options) (*App, error) {
	a := &App{opts: opts, lastCrash: opts.LastCrash}
	a.here.a = a
	var err error
	if a.settings, err = store.Open(filepath.Join(opts.DataDir, "settings.json"), settings.Default()); err != nil {
		return nil, fmt.Errorf("opening settings: %w", err)
	}
	if a.library, err = library.Open(filepath.Join(opts.DataDir, "library.json")); err != nil {
		return nil, fmt.Errorf("opening the library: %w", err)
	}
	if a.updates, err = store.Open(filepath.Join(opts.DataDir, "updater.json"), updatePrefs{}); err != nil {
		return nil, fmt.Errorf("opening the update preferences: %w", err)
	}
	a.checkUpdates = make(chan struct{}, 1)
	matches, err := store.Open(filepath.Join(opts.CacheDir, "matches.json"), map[string]media.Track{})
	if err != nil {
		return nil, fmt.Errorf("opening matches: %w", err)
	}
	loudness, err := store.Open(filepath.Join(opts.CacheDir, "loudness.json"), map[string]float64{})
	if err != nil {
		return nil, fmt.Errorf("opening loudness: %w", err)
	}
	s := a.settings.Get().Clean()

	a.youtube = youtube.New()
	a.auth = spotify.NewAuth(s.SpotifyClientID)
	a.spotify = spotify.NewClient(a.auth)
	a.tokens = tokenStore{newSecretStore(opts.DataDir, tokenSecret, "spotify-token.json", "spotify-token.dpapi")}
	a.spotify.OnToken = func(t spotify.Token) {
		if err := a.tokens.Save(t); err != nil {
			log.Printf("mixmusic: keeping the Spotify token: %v", err)
		}
	}
	if t, err := a.tokens.Load(); err == nil {
		a.spotify.SetToken(t)
	}
	a.lastfmStore = lastfmStore{newSecretStore(opts.DataDir, lastfmSecret, "lastfm.json", "lastfm.dpapi")}
	a.openLastfm()
	a.tools = tools.New(filepath.Join(opts.DataDir, "bin"))
	a.resolver = stream.NewResolver(a.tools, filepath.Join(opts.CacheDir, "yt-dlp"))
	// MIXMUSIC_YTDLP_ONLY has yt-dlp find every stream, as before MixMusic
	// asked YouTube's player API: to time one against the other, or to
	// tell which fails. The API still tells how loud tracks are.
	direct := stream.NewDirect()
	if os.Getenv("MIXMUSIC_YTDLP_ONLY") == "" {
		a.resolver.Direct = direct
	} else {
		log.Print("mixmusic: yt-dlp alone finds the streams (MIXMUSIC_YTDLP_ONLY)")
	}
	a.loudness = newLoudnessBook(direct.Loudness, loudness)
	a.cache = stream.NewCache(filepath.Join(opts.CacheDir, "audio"), 0)
	a.proxy = stream.NewProxy(a.resolver)
	// The player asks whether the network answers, which audio coming
	// tells too.
	a.online = online.New(online.Probe)
	a.proxy.Seen = a.online.Seen
	a.youtubeStore = newYouTubeStore(opts.DataDir)
	a.openYouTube()
	a.proxy.Cache = a.cache
	a.matcher = match.New(a.youtube, matches)
	a.lyrics = lyrics.New("MixMusic/" + opts.Version + " (https://github.com/fzn0x/mixmusic)")
	a.lyricsMemo = newMemo[lyrics.Lyrics](200)
	a.related = newMemo[[]media.Shelf](100)
	a.artStore = artwork.NewStore(filepath.Join(opts.CacheDir, "artwork"), artworkLimit)
	a.artwork = artwork.New()
	a.artwork.Store = a.artStore
	a.colors = newMemo[string](500)
	a.addons = addons.NewManager(filepath.Join(opts.DataDir, "addons"), opts.Builtin)
	a.storage = addons.NewStorage(filepath.Join(opts.DataDir, "addons", ".storage"))
	a.apply(s)
	return a, nil
}

// apply makes settings take effect in the parts of the app they configure.
func (a *App) apply(s settings.Settings) {
	a.tools.Configure(s.YTDLPPath, s.JSRuntime)
	a.resolver.SetQuality(stream.Quality(s.Quality))
	a.cache.SetLimit(int64(s.CacheSizeMB) << 20)
	if a.auth.ClientID() != s.SpotifyClientID {
		a.auth.SetClientID(s.SpotifyClientID)
	}
	if mygo.App.IsReady() {
		a.applyMediaKeys(s.MediaKeys)
	}
	if !s.DiscordPresence {
		a.presence.update(false, "", NowPlaying{})
	}
	if p := a.goPlayer.Load(); p != nil {
		p.SettingsChanged(s)
		a.applyEngine(s)
	}
	if s.SpotifyPlayback != settings.SpotifyPlaybackHere {
		a.here.close()
	}
	if a.youtube.SignedIn() && a.youtube.Channel() != s.YouTubeChannel {
		a.youtube.SetChannel(s.YouTubeChannel)
	}
	a.retitle()
}

// Bind registers what the page of MixMusic's own Spotify device calls. It
// must run before mygo.App.Run.
func (a *App) Bind() { mygo.Bind(&SpotifyPage{a}) }

// needWebView tells why what, which takes a window showing a web page,
// can't be done, or nil when it can: MixMusic's own windows need no
// webview, so it runs on systems without one, where opening such a window
// would panic.
func needWebView(what string) error {
	if webViewAvailable() {
		return nil
	}
	switch runtime.GOOS {
	case "windows":
		return fmt.Errorf("%s needs Microsoft's WebView2 Runtime, which this computer doesn't have: get it from https://go.microsoft.com/fwlink/p/?LinkId=2124703", what)
	case "linux":
		return fmt.Errorf("%s needs WebKitGTK, which this computer doesn't have: install libwebkit2gtk-4.1-0 on Debian and Ubuntu, webkit2gtk4.1 on Fedora", what)
	}
	return fmt.Errorf("%s needs the system's webview, which this computer doesn't have", what)
}

// webViewAvailable is mygo.WebViewAvailable, asked on the main thread. On
// Linux the first call loads WebKitGTK and keeps its cookies in a file,
// which creates WebKit's default web context on the calling thread. Created
// on another thread than GTK's, the context makes the web view of the next
// window abort as GTK's thread creates it (#51).
func webViewAvailable() bool {
	var available bool
	mygo.RunOnMain(func() { available = mygo.WebViewAvailable() })
	return available
}

// schemeURL returns the base URL of a custom scheme, as WebView2 serves
// custom schemes, at http://<scheme>.localhost: the stream addresses
// PrepareStream gives keep that form.
func schemeURL(scheme string) string {
	if runtime.GOOS == "windows" {
		return "http://" + scheme + ".localhost/"
	}
	return scheme + "://localhost/"
}

// memo remembers the results of lookups, up to a number of keys.
type memo[T any] struct {
	mu    sync.Mutex
	max   int
	order []string
	items map[string]T
}

func newMemo[T any](max int) *memo[T] {
	return &memo[T]{max: max, items: map[string]T{}}
}

func (m *memo[T]) get(key string) (T, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.items[key]
	return v, ok
}

func (m *memo[T]) put(key string, v T) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.items[key]; !ok {
		m.order = append(m.order, key)
		if len(m.order) > m.max {
			delete(m.items, m.order[0])
			m.order = m.order[1:]
		}
	}
	m.items[key] = v
}
