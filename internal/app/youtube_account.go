package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// youtubeKeySecret is the key, in the system's credential store, of the
// key that encrypts the YouTube session: its cookies are more than the
// credential store keeps.
const youtubeKeySecret = "youtube-session-key"

// youtubeSignInURL signs in to Google and comes back to YouTube Music.
const youtubeSignInURL = "https://accounts.google.com/ServiceLogin?ltmpl=music&service=youtube&passive=true&continue=" +
	"https%3A%2F%2Fmusic.youtube.com%2F"

// youtubeCookie is a cookie of the YouTube session, as kept on disk.
type youtubeCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitzero"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"httpOnly"`
}

// youtubeStore keeps the cookies of the user's YouTube session, encrypted
// with a key of the system's credential store, or readable by the user
// alone where there is no credential store.
type youtubeStore struct{ dir string }

func (s youtubeStore) sealed() string { return filepath.Join(s.dir, "youtube-session.bin") }
func (s youtubeStore) plain() string  { return filepath.Join(s.dir, "youtube-session.json") }

func (s youtubeStore) Load() ([]*http.Cookie, error) {
	var data []byte
	key, err := mygo.Secrets.Get(youtubeKeySecret)
	switch {
	case err == nil:
		sealed, err := os.ReadFile(s.sealed())
		if err != nil {
			return nil, err
		}
		if data, err = open(key, sealed); err != nil {
			return nil, err
		}
	case errors.Is(err, errors.ErrUnsupported):
		if data, err = os.ReadFile(s.plain()); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	var kept []youtubeCookie
	if err := json.Unmarshal(data, &kept); err != nil {
		return nil, err
	}
	cookies := make([]*http.Cookie, len(kept))
	for i, k := range kept {
		cookies[i] = &http.Cookie{Name: k.Name, Value: k.Value, Domain: k.Domain, Path: k.Path, Expires: k.Expires, Secure: k.Secure, HttpOnly: k.HTTPOnly}
	}
	return cookies, nil
}

func (s youtubeStore) Save(cookies []*http.Cookie) error {
	kept := make([]youtubeCookie, len(cookies))
	for i, c := range cookies {
		kept[i] = youtubeCookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Expires: c.Expires, Secure: c.Secure, HTTPOnly: c.HttpOnly}
	}
	data, err := json.Marshal(kept)
	if err != nil {
		return err
	}
	key, err := mygo.Secrets.Get(youtubeKeySecret)
	if errors.Is(err, mygo.ErrSecretNotFound) {
		key = make([]byte, 32)
		rand.Read(key)
		err = mygo.Secrets.Set(youtubeKeySecret, key)
	}
	if errors.Is(err, errors.ErrUnsupported) {
		if err := store.WriteFile(s.plain(), data); err != nil {
			return err
		}
		return os.Chmod(s.plain(), 0o600)
	}
	if err != nil {
		return err
	}
	sealed, err := seal(key, data)
	if err != nil {
		return err
	}
	return store.WriteFile(s.sealed(), sealed)
}

func (s youtubeStore) Clear() error {
	err := mygo.Secrets.Delete(youtubeKeySecret)
	if errors.Is(err, errors.ErrUnsupported) || errors.Is(err, mygo.ErrSecretNotFound) {
		err = nil
	}
	for _, f := range []string{s.sealed(), s.plain()} {
		if rmErr := os.Remove(f); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
	}
	return err
}

// seal encrypts data with AES-GCM, the nonce first.
func seal(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	return gcm.Seal(nonce, nonce, data, nil), nil
}

// open decrypts what seal encrypted.
func open(key, sealed []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("the YouTube session file is damaged")
	}
	return gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
}

// YouTubeAccount is the state of the YouTube Music sign-in.
type YouTubeAccount struct {
	SignedIn bool             `json:"signedIn"`
	Account  *youtube.Account `json:"account"`
	// Error tells why the account could not be read, if it could not.
	Error string `json:"error,omitempty"`
}

// openYouTube signs the YouTube client in with the session kept, if any.
func (a *App) openYouTube() {
	// Version 0.2.0 gave yt-dlp the session in this file. Streams are
	// found signed out now, so the copy goes.
	os.Remove(filepath.Join(a.opts.CacheDir, "youtube-cookies.txt"))
	a.youtube.OnCookies = func(cookies []*http.Cookie) {
		if err := a.youtubeStore.Save(cookies); err != nil {
			log.Printf("mixmusic: keeping the YouTube session: %v", err)
		}
	}
	// YouTube ended the session: the accounts say so, and ask to sign in
	// again.
	a.youtube.OnRejected = func() {
		a.mu.Lock()
		a.youtubeMe = nil
		a.mu.Unlock()
		notify()
	}
	cookies, err := a.youtubeStore.Load()
	if err != nil || len(cookies) == 0 {
		return
	}
	a.youtube.SetCookies(cookies)
	a.youtube.SetChannel(a.settings.Get().YouTubeChannel)
}

func (a *App) youtubeAccount(ctx context.Context) YouTubeAccount {
	if !a.youtube.SignedIn() {
		return YouTubeAccount{}
	}
	a.mu.Lock()
	me := a.youtubeMe
	a.mu.Unlock()
	if me == nil {
		acct, err := a.youtube.Account(ctx)
		if err != nil {
			return YouTubeAccount{SignedIn: true, Error: err.Error()}
		}
		me = &acct
		a.mu.Lock()
		a.youtubeMe = me
		a.mu.Unlock()
	}
	return YouTubeAccount{SignedIn: true, Account: me}
}

// YouTubeLogin opens a window where the user signs in to Google, and signs
// MixMusic in to YouTube Music with that session once Google comes back to
// YouTube Music. Nothing the user types passes through MixMusic: it reads
// the cookies of the session the window ends with.
func (s *Accounts) YouTubeLogin(ctx context.Context) (AccountsStatus, error) {
	if err := needWebView("signing in to YouTube Music"); err != nil {
		return AccountsStatus{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	done := make(chan []*http.Cookie, 1)
	closed := make(chan struct{})
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:  "Sign in to YouTube Music",
		Width:  480,
		Height: 720,
		URL:    youtubeSignInURL,
	})
	check := func(url string) {
		if !strings.HasPrefix(url, "https://music.youtube.com") {
			return
		}
		cookies, err := youtubeSession()
		if err == nil && len(cookies) > 0 {
			select {
			case done <- cookies:
			default:
			}
		}
	}
	// Navigation events run on the UI thread, inside the webview's own
	// handler, where a cookie call waits for a webview that cannot answer
	// until the handler returns: the windows freeze, this one blank.
	// Google sends a session it knows straight back to YouTube Music, so
	// the first navigation already gets here.
	win.Page().OnDidNavigate(func(url string) { go check(url) })
	win.OnClosed(func() { close(closed) })
	defer win.Close()

	var cookies []*http.Cookie
	select {
	case cookies = <-done:
	case <-closed:
		return AccountsStatus{}, errors.New("the sign-in window closed before signing in")
	case <-ctx.Done():
		return AccountsStatus{}, errors.New("signing in to YouTube Music took too long; try again")
	}
	s.a.youtube.SetCookies(cookies)
	s.a.mu.Lock()
	s.a.youtubeMe = nil
	s.a.mu.Unlock()
	if _, err := s.a.youtube.Account(ctx); err != nil {
		s.a.youtube.SetCookies(nil)
		return AccountsStatus{}, fmt.Errorf("YouTube Music did not accept the sign-in: %w", err)
	}
	if err := s.a.youtubeStore.Save(cookies); err != nil {
		return AccountsStatus{}, err
	}
	status := s.a.accountsStatus(ctx)
	notify()
	return status, nil
}

// youtubeSession returns the cookies a request to YouTube Music sends, when
// they hold a signed-in session.
func youtubeSession() ([]*http.Cookie, error) {
	list, err := mygo.Cookies.List(mygo.CookieFilter{URL: "https://music.youtube.com/"})
	if err != nil {
		return nil, err
	}
	var cookies []*http.Cookie
	signed := false
	for _, c := range list {
		cookies = append(cookies, &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Expires: c.Expires, Secure: c.Secure, HttpOnly: c.HTTPOnly})
		if c.Name == "SAPISID" || c.Name == "__Secure-3PAPISID" {
			signed = true
		}
	}
	if !signed {
		return nil, nil
	}
	return cookies, nil
}

// YouTubeLogout signs MixMusic out of YouTube Music, and forgets the
// Google session of the sign-in window.
func (s *Accounts) YouTubeLogout(ctx context.Context) error {
	s.a.youtube.SetCookies(nil)
	s.a.mu.Lock()
	s.a.youtubeMe = nil
	s.a.mu.Unlock()
	err := s.a.youtubeStore.Clear()
	if set := s.a.settings.Get(); set.YouTubeChannel != "" {
		set.YouTubeChannel = ""
		if serr := s.a.settings.Set(set); serr == nil {
			notify()
		}
	}
	for _, domain := range []string{"youtube.com", "google.com"} {
		list, lerr := mygo.Cookies.List(mygo.CookieFilter{Domain: domain})
		if lerr != nil {
			continue
		}
		for _, c := range list {
			if derr := mygo.Cookies.Delete(c); derr != nil {
				err = errors.Join(err, derr)
			}
		}
	}
	notify()
	return err
}
