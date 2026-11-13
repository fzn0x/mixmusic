package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/netease"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// neteaseSecret is the key of the NetEase Cloud Music session in the
// system's credential store.
const neteaseSecret = "netease-session"

// neteaseLevel is the quality NetEase is asked for: 320 kbit/s MP3.
// shortcut: one quality, whatever Settings › Quality says; map it to
// "lossless" (FLAC) once members ask for it.
const neteaseLevel = "exhigh"

// NetEaseAccount is the state of the NetEase Cloud Music account.
type NetEaseAccount struct {
	SignedIn bool             `json:"signedIn"`
	Profile  *netease.Profile `json:"profile"`
	Error    string           `json:"error,omitempty"`
}

// openNetEase signs the client in with the session kept, if any.
func (a *App) openNetEase() {
	if u, err := a.neteaseStore.Load(); err == nil && u != "" {
		a.netease.SetSession(u)
	}
	a.resolver.NetEase = a.neteaseStream
}

// neteaseAccount is the state of the account, reading the profile once.
func (a *App) neteaseAccount(ctx context.Context) NetEaseAccount {
	acc := NetEaseAccount{SignedIn: a.netease.SignedIn()}
	if !acc.SignedIn {
		return acc
	}
	me, err := a.neteaseProfile(ctx)
	if err != nil {
		acc.Error = err.Error()
	} else {
		acc.Profile = &me
	}
	return acc
}

// neteaseProfile returns the signed-in user, asking NetEase once. A
// session NetEase no longer takes is dropped.
func (a *App) neteaseProfile(ctx context.Context) (netease.Profile, error) {
	a.mu.Lock()
	me := a.neteaseMe
	a.mu.Unlock()
	if me != nil {
		return *me, nil
	}
	p, err := a.netease.Me(ctx)
	if errors.Is(err, netease.ErrSignedOut) {
		a.signOutNetEase()
	}
	if err != nil {
		return netease.Profile{}, err
	}
	a.mu.Lock()
	a.neteaseMe = &p
	a.mu.Unlock()
	return p, nil
}

// neteaseUserID returns the signed-in user's ID, or "".
func (a *App) neteaseUserID(ctx context.Context) string {
	if !a.netease.SignedIn() {
		return ""
	}
	me, err := a.neteaseProfile(ctx)
	if err != nil {
		return ""
	}
	return me.ID
}

// NetEaseLogin opens NetEase Cloud Music's sign-in page in a window, where
// the user scans its QR code with the NetEase Cloud Music app, and keeps
// the session once the page has one.
func (s *Accounts) NetEaseLogin(ctx context.Context) (AccountsStatus, error) {
	if err := needWebView("signing in to NetEase Cloud Music"); err != nil {
		return AccountsStatus{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	closed := make(chan struct{})
	win := mygo.NewWindow(mygo.WindowOptions{Title: "登录网易云音乐", Width: 960, Height: 720, URL: netease.SignInURL})
	win.OnClosed(func() { close(closed) })
	defer win.Close()
	// The page signs in without navigating: its cookies tell.
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-closed:
			return AccountsStatus{}, errors.New("the sign-in window closed before signing in")
		case <-ctx.Done():
			return AccountsStatus{}, errors.New("signing in to NetEase Cloud Music took too long; try again")
		case <-tick.C:
		}
		list, err := mygo.Cookies.List(mygo.CookieFilter{URL: "https://music.163.com/"})
		if err != nil {
			continue
		}
		for _, c := range list {
			if c.Name == netease.CookieName && c.Value != "" {
				return s.a.neteaseSignIn(ctx, c.Value)
			}
		}
	}
}

// neteaseSignIn keeps a session once NetEase shows it takes it.
func (a *App) neteaseSignIn(ctx context.Context, musicU string) (AccountsStatus, error) {
	a.netease.SetSession(musicU)
	a.mu.Lock()
	a.neteaseMe = nil
	a.mu.Unlock()
	if _, err := a.neteaseProfile(ctx); err != nil {
		a.netease.SetSession("")
		return AccountsStatus{}, fmt.Errorf("NetEase Cloud Music did not accept the sign-in: %w", err)
	}
	if err := a.neteaseStore.Save(musicU); err != nil {
		return AccountsStatus{}, err
	}
	notify()
	return a.accountsStatus(ctx), nil
}

// NetEaseLogout signs out of NetEase Cloud Music, in the sign-in window's
// cookies too, so that signing in again shows the QR code.
func (s *Accounts) NetEaseLogout(ctx context.Context) error {
	err := s.a.signOutNetEase()
	if list, lerr := mygo.Cookies.List(mygo.CookieFilter{Domain: "music.163.com"}); lerr == nil {
		for _, c := range list {
			err = errors.Join(err, mygo.Cookies.Delete(c))
		}
	}
	return err
}

func (a *App) signOutNetEase() error {
	a.netease.SetSession("")
	a.mu.Lock()
	a.neteaseMe = nil
	a.mu.Unlock()
	err := a.neteaseStore.Clear()
	notify()
	return err
}

// neteaseStream finds the audio of a song, as the resolver asks.
func (a *App) neteaseStream(ctx context.Context, songID string) (stream.Stream, error) {
	au, err := a.netease.SongAudio(ctx, songID, neteaseLevel)
	if err != nil {
		return stream.Stream{}, err
	}
	mime := "audio/mpeg"
	if au.Type == "flac" {
		mime = "audio/flac"
	}
	return stream.Stream{URL: au.URL, MimeType: mime, Size: au.Size, Expires: au.Expires}, nil
}

// neteaseSearcher is NetEase as the matcher searches it, in YouTube Music's
// place: songs alone, it has no videos.
type neteaseSearcher struct{ c *netease.Client }

func (s neteaseSearcher) Search(ctx context.Context, query string, filter youtube.Filter) ([]media.Item, error) {
	if filter != youtube.FilterSongs {
		return nil, nil
	}
	tracks, err := s.c.SearchSongs(ctx, query)
	if err != nil {
		log.Printf("mixmusic: searching NetEase Cloud Music: %v", err)
		return nil, err
	}
	items := make([]media.Item, len(tracks))
	for i := range tracks {
		items[i] = media.Item{Kind: media.ItemTrack, Track: &tracks[i]}
	}
	return items, nil
}

// streamKey returns the key the proxy knows a track's audio by: a
// video's ID, or a NetEase song's key.
func streamKey(t media.Track) (string, error) {
	source, native, err := media.ParseID(t.ID)
	if err != nil {
		return "", err
	}
	if source == media.SourceNetEase {
		return stream.NetEaseKey(native), nil
	}
	return native, nil
}

// NetEasePlaylists returns the playlists the user made and saved on NetEase
// Cloud Music.
func (c *Collection) NetEasePlaylists(ctx context.Context) ([]media.Playlist, error) {
	uid := c.a.neteaseUserID(ctx)
	if uid == "" {
		return nil, netease.ErrSignedOut
	}
	return c.a.netease.UserPlaylists(ctx, uid)
}

// NetEaseLiked returns the user's liked songs on NetEase Cloud Music: the
// first playlist they made, which NetEase keeps them in.
func (c *Collection) NetEaseLiked(ctx context.Context) (media.Playlist, error) {
	list, err := c.NetEasePlaylists(ctx)
	if err != nil {
		return media.Playlist{}, err
	}
	for _, p := range list {
		if p.Mine {
			return c.a.wholePlaylist(ctx, p.ID)
		}
	}
	return media.Playlist{Source: media.SourceNetEase, Tracks: []media.Track{}}, nil
}
