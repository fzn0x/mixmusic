package app

import (
	"context"
	"errors"
	"net/url"
	"runtime"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/artwork"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/audio/output"
	"github.com/fzn0x/mixmusic/internal/opus"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/tools"
)

// Info describes the app.
type Info struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
	Dev         bool   `json:"dev"`
	DataDir     string `json:"dataDir"`
	RedirectURI string `json:"redirectUri"`
	// NativeAudio is set where MixMusic's own audio engine can play: on
	// 64-bit Windows, macOS and Linux, with its audio library installed.
	NativeAudio bool `json:"nativeAudio"`
}

// System is what the windows need of the desktop and the app.
type System struct{ a *App }

// Info describes the app.
func (s *System) Info() Info {
	return Info{
		Name:        mygo.App.Name(),
		Version:     s.a.version(),
		Platform:    runtime.GOOS,
		Arch:        runtime.GOARCH,
		Dev:         mygo.IsDev(),
		DataDir:     s.a.opts.DataDir,
		RedirectURI: spotify.RedirectURI,
		NativeAudio: settings.NativeAudio && output.Available() == nil,
	}
}

// version is the version of MixMusic that runs.
func (a *App) version() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return a.opts.Version
}

// Tools describes the tools MixMusic runs: yt-dlp and a JavaScript
// runtime.
func (s *System) Tools(ctx context.Context) tools.Status { return s.a.tools.Status(ctx) }

// ArtworkColor returns the accent color of a piece of artwork, as
// "#rrggbb", for themes that follow what plays.
func (s *System) ArtworkColor(ctx context.Context, url string) (string, error) {
	if c, ok := s.a.colors.get(url); ok {
		return c, nil
	}
	c, err := s.a.artwork.Color(ctx, url)
	if err != nil {
		return "", err
	}
	s.a.colors.put(url, c)
	return c, nil
}

// OpenExternal opens a web page in the default browser.
func (s *System) OpenExternal(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("only web pages open in the browser")
	}
	return mygo.Shell.OpenExternal(u.String())
}

// Notice is the license of code MixMusic includes from another project.
type Notice struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// Notices returns the licenses of the code MixMusic includes from other
// projects, which its copies must reproduce.
func (s *System) Notices() []Notice {
	return []Notice{
		{Name: "libopus, which decodes the audio", Text: opus.License},
		{Name: "Chromium's Web Audio, which shapes the sound", Text: audio.License},
	}
}

// CacheUsage is what the audio cache holds.
type CacheUsage struct {
	Bytes  int64 `json:"bytes"`
	Tracks int   `json:"tracks"`
}

// CacheUsage returns what the audio cache holds.
func (s *System) CacheUsage() CacheUsage {
	b, n := s.a.cache.Usage()
	return CacheUsage{Bytes: b, Tracks: n}
}

// ClearCache empties the audio cache.
func (s *System) ClearCache() error { return s.a.cache.Clear() }

// Artwork downloads the artwork the windows show, and keeps it on disk.
func (s *System) Artwork() *artwork.Store { return s.a.artStore }

// ArtworkUsage is what the artwork kept on disk holds.
type ArtworkUsage struct {
	Bytes  int64 `json:"bytes"`
	Images int   `json:"images"`
}

// ArtworkUsage returns what the artwork kept on disk holds.
func (s *System) ArtworkUsage() ArtworkUsage {
	b, n := s.a.artStore.Usage()
	return ArtworkUsage{Bytes: b, Images: n}
}

// ClearArtwork removes the artwork kept on disk.
func (s *System) ClearArtwork() error { return s.a.artStore.Clear() }

// ShowMiniPlayer shows the mini player, a small window over the others.
func (s *System) ShowMiniPlayer() { s.a.showMini() }

// ShowMainWindow shows the main window and closes the mini player.
func (s *System) ShowMainWindow() {
	s.a.showMain()
	s.a.mu.Lock()
	mini := s.a.mini
	s.a.mu.Unlock()
	if mini != nil {
		mini.Close()
	}
}

// Notify shows a desktop notification, for plugins.
func (s *System) Notify(title, body string) error {
	return mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body}).Show()
}

// Quit quits MixMusic.
func (s *System) Quit() { mygo.App.Quit() }
