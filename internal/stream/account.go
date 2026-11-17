package stream

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/tools"
)

// Asking as the account: YouTube serves YouTube Music Premium's audio of a
// higher bitrate, Opus of format 774 and AAC-LC of format 141, only to a
// request made as a Premium account. yt-dlp asks as one with the cookies
// of its session (--cookies), with the clients it picks for a signed-in
// session, and YouTube Music's own web client too for a music.youtube.com
// address. Those need a challenge solved, which takes a JavaScript
// runtime.

// SessionFilePrefix begins the names of the files that hand the session of
// the user's YouTube Music account to yt-dlp, one a run: the app removes
// those a MixMusic that stopped meanwhile left, as it does the copies of
// browsers' cookies, which are named alike.
const SessionFilePrefix = "mixmusic-cookies-"

// accountTimeout bounds yt-dlp's run as the account, which leaves the rest
// of resolveTimeout to the ways that ask signed out. Tests shorten it.
var accountTimeout = 30 * time.Second

// maxRefusedAsAccount bounds how many videos the resolver remembers of
// those whose stream found as the account YouTube refused.
const maxRefusedAsAccount = 500

// refusalMark is how long a video is found signed out once YouTube
// refused its stream found as the account (refused): YouTube may refuse
// it for a while, or only on the network it was found on, before a move
// to another or to a VPN, after which the account finds one that plays.
const refusalMark = 10 * time.Minute

// SetAsAccount has the resolver ask for streams as the user's YouTube Music
// account from now on, while Session gives a session, or signed out:
// streams found before are found again, and the videos YouTube refused
// the stream of found as the account are asked for as it again.
func (r *Resolver) SetAsAccount(on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.asAccount != on {
		r.asAccount = on
		r.calls = map[string]*resolution{}
		r.refusedAsAccount = nil
	}
}

// accountStream has yt-dlp find the stream of a video as the user's YouTube
// Music account, for Premium's higher bitrate, when accountSession gives
// a session to ask as. It reports false when it didn't ask, or found no
// stream the engine plays: the stream is found signed out then, as
// without the switch, so a track never fails for it. again asks once
// more, after the signed-out ways (againAsAccount). The log tells how it
// went, never the session nor the stream's URL.
func (r *Resolver) accountStream(ctx context.Context, videoID string, quality Quality, again bool) (Stream, bool) {
	cookies := r.accountSession(videoID, quality, again)
	if cookies == nil {
		return Stream{}, false
	}
	more, then := "", "it's found signed out"
	if again {
		more, then = " once more", "nor is it signed out, where YouTube asks to sign in"
	}
	start := time.Now()
	actx, cancel := context.WithTimeout(ctx, accountTimeout)
	s, err := r.ytdlpAsAccount(actx, videoID, quality, cookies)
	cancel()
	if err == nil {
		err = takenAsAccount(s)
	}
	took := time.Since(start).Round(10 * time.Millisecond)
	if err != nil {
		log.Printf("mixmusic: yt-dlp found no stream of %s as your YouTube Music account%s in %s (%s); %s",
			videoID, more, took, strings.TrimPrefix(err.Error(), "stream: "), then)
		return Stream{}, false
	}
	s.AsAccount = true
	if s.Surround != nil {
		s.Surround.AsAccount = true
	}
	log.Printf("mixmusic: found the stream of %s as your YouTube Music account%s in %s with yt-dlp: %s at %.0f kbit/s",
		videoID, more, took, s.Codec, s.Bitrate)
	return s, true
}

// againAsAccount has yt-dlp find the stream of a video as the account once
// more, after err, why the signed-out ways found none, which is YouTube's
// sign-in wall (signInWall), when YouTube refused the stream of the video
// found as the account less than refusalMark ago: the account may pass
// the wall with a stream that plays, as after a move to another network.
// It asks once a resolve, so a track whose stream YouTube refuses again
// fails rather than asks on.
func (r *Resolver) againAsAccount(ctx context.Context, videoID string, quality Quality, err error) (Stream, bool) {
	if ctx.Err() != nil || !signInWall(err) {
		return Stream{}, false
	}
	return r.accountStream(ctx, videoID, quality, true)
}

// signInWalls are parts of yt-dlp's errors, as plainErrors has them, of a
// video YouTube plays only to signed-in viewers: its "Please sign in", its
// check of age, and a channel's members only. Its check of bots is not
// one: it is of the network, not the viewer.
var signInWalls = []string{"please sign in", "confirm your age", "members-only"}

// signInWall reports whether err, why the signed-out ways found no stream,
// is YouTube's wall before a video it plays only to signed-in viewers, in
// the plain words of one of signInWalls.
func signInWall(err error) bool {
	var f failure
	if !errors.As(err, &f) {
		return false
	}
	for _, match := range signInWalls {
		if plain, _ := plainError(match); plain == f.msg {
			return true
		}
	}
	return false
}

// accountSession returns the session to ask for the stream of a video as,
// or nil to ask signed out: while the user asks for it (SetAsAccount) and
// Session gives one yt-dlp takes for a signed-in one, of the best quality,
// which alone gains from Premium, with a JavaScript runtime, unless
// YouTube refused the stream of the video found as the account less than
// refusalMark ago, as the same session. again asks for such a video
// alone: once more, after the signed-out ways (againAsAccount).
func (r *Resolver) accountSession(videoID string, quality Quality, again bool) []*http.Cookie {
	r.mu.Lock()
	on := r.asAccount
	r.mu.Unlock()
	// The clients yt-dlp asks as for an account need a challenge solved,
	// which takes a runtime, as the 5.1 audio does.
	if !on || r.Session == nil || quality != QualityBest || !r.findsSurround() {
		return nil
	}
	cookies := r.Session()
	r.mu.Lock()
	refused := r.marked(videoID, sessionKey(cookies))
	r.mu.Unlock()
	if refused != again || len(cookies) == 0 {
		return nil
	}
	if !signedInFor(cookies) {
		r.noLogin.Do(func() {
			log.Print("mixmusic: the YouTube Music session lacks its LOGIN_INFO or SAPISID cookie, without which yt-dlp asks signed out: " +
				"streams are found signed out; signing in to YouTube Music again may bring them")
		})
		return nil
	}
	return cookies
}

// signedInFor reports whether yt-dlp takes cookies for a signed-in session,
// which it asks as: those holding LOGIN_INFO, beside a cookie that signs
// requests, SAPISID or one of its secure twins.
func signedInFor(cookies []*http.Cookie) bool {
	login, signs := false, false
	for _, c := range cookies {
		if c == nil || c.Value == "" {
			continue
		}
		switch c.Name {
		case "LOGIN_INFO":
			login = true
		case "SAPISID", "__Secure-1PAPISID", "__Secure-3PAPISID":
			signs = true
		}
	}
	return login && signs
}

// sessionKey tells sessions apart by the cookies that sign the user in,
// LOGIN_INFO and SAPISID's, which YouTube sets as they sign in and keeps
// while the session lasts, unlike those it refreshes as it goes: a hash
// of them, so that the resolver keeps no copy of the session, or "" for
// none.
func sessionKey(cookies []*http.Cookie) string {
	h := sha256.New()
	signs := false
	for _, c := range cookies {
		if c == nil || c.Value == "" {
			continue
		}
		switch c.Name {
		case "LOGIN_INFO", "SAPISID", "__Secure-1PAPISID", "__Secure-3PAPISID":
			fmt.Fprintf(h, "%s=%s\n", c.Name, c.Value)
			signs = true
		}
	}
	if !signs {
		return ""
	}
	return string(h.Sum(nil))
}

// takenAsAccount tells why s, the stream yt-dlp found as the account, isn't
// taken, or nil: one of a codec the engine doesn't play, which the stream
// found signed out may have, or a live one, whose audio is no better as
// the account.
func takenAsAccount(s Stream) error {
	switch {
	case s.Live:
		return errors.New("a live video, whose audio is the same signed out")
	case !strings.EqualFold(s.Codec, "opus") && s.Codec != "mp4a.40.2":
		return fmt.Errorf("its audio is %q, which MixMusic doesn't play", s.Codec)
	}
	return nil
}

// ytdlpAsAccount asks yt-dlp for the stream of a video the full way, as
// ytdlp does, with the cookies of the session in a file of their own,
// which goes once yt-dlp ran.
func (r *Resolver) ytdlpAsAccount(ctx context.Context, videoID string, quality Quality, cookies []*http.Cookie) (Stream, error) {
	file, err := writeSession(cookies, time.Now())
	if err != nil {
		return Stream{}, err
	}
	defer os.Remove(file)
	args := []string{"-J", "--no-playlist", "-f", formats[quality], "--cookies", file}
	if r.Tools != nil {
		args = append(args, r.Tools.YTDLPArgs(r.CacheDir)...)
	}
	args = append(args, "--", "https://music.youtube.com/watch?v="+videoID)
	out, err := r.Run(ctx, args...)
	if err != nil {
		return Stream{}, err
	}
	s, err := parseInfo(out)
	if err != nil {
		return Stream{}, err
	}
	return s.found(RouteAccount, args), nil
}

// writeSession writes cookies, of a YouTube Music session, into a new file
// of the Netscape format, which yt-dlp reads with --cookies, and writes
// again as it ends: readable by the user alone, as os.CreateTemp makes it,
// in the system's temporary folder. The caller removes it once yt-dlp ran.
// Each cookie goes to every host of youtube.com, as MixMusic's own
// requests to music.youtube.com send them all, and yt-dlp asks
// www.youtube.com too. Those that expired, a second of a name, and those
// whose name or value would break a line are left out.
func writeSession(cookies []*http.Cookie, now time.Time) (string, error) {
	var b strings.Builder
	b.WriteString(tools.NetscapeHeader)
	seen := map[string]bool{}
	for _, c := range cookies {
		if c == nil || c.Name == "" || seen[c.Name] || strings.ContainsAny(c.Name+c.Value, "\t\r\n") ||
			!c.Expires.IsZero() && !c.Expires.After(now) {
			continue
		}
		seen[c.Name] = true
		// 0 is a cookie that ends with the session.
		var expires int64
		if !c.Expires.IsZero() {
			expires = c.Expires.Unix()
		}
		fmt.Fprintf(&b, ".youtube.com\tTRUE\t/\tTRUE\t%d\t%s\t%s\n", expires, c.Name, c.Value)
	}
	f, err := os.CreateTemp("", SessionFilePrefix+"*.txt")
	if err != nil {
		return "", err
	}
	_, err = f.WriteString(b.String())
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// refused remembers that YouTube refused the stream of a video found as
// the account, when res, which the resolver drops, holds one: it is found
// signed out for refusalMark from then on. It runs under the resolver's
// mutex.
func (r *Resolver) refused(videoID string, res *resolution) {
	if res.err != nil || !res.stream.AsAccount {
		return
	}
	if r.refusedAsAccount == nil || len(r.refusedAsAccount) >= maxRefusedAsAccount {
		r.refusedAsAccount = map[string]time.Time{}
	}
	r.refusedAsAccount[videoID] = time.Now()
}

// marked reports whether YouTube refused the stream of a video found as
// the account less than refusalMark ago, forgetting a mark older than
// that, as the session of key (sessionKey): every mark goes once Session
// gives another, as signing in or out, YouTube ending the session, or
// another Google account of it do. It runs under the resolver's mutex.
func (r *Resolver) marked(videoID, key string) bool {
	if key != r.markedSession {
		r.refusedAsAccount, r.markedSession = nil, key
		return false
	}
	at, ok := r.refusedAsAccount[videoID]
	if ok && time.Since(at) >= refusalMark {
		delete(r.refusedAsAccount, videoID)
		return false
	}
	return ok
}
