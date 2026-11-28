package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/lyrics"
)

func TestNewProvidersLookupAndFallback(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/lrcred/s/GBAYE0000267.ttml", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<tt xmlns="http://www.w3.org/ns/ttml" xml:lang="en"><body><div><p begin="1.0" end="2.0"><span begin="1.0" end="2.0">LrcRed</span></p><p begin="3.0" end="4.0"><span begin="3.0" end="4.0">Word</span></p></div></body></tt>`))
	})
	mux.HandleFunc("/spotify/color-lyrics/v2/track/sp1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("app-platform") != "WebPlayer" || r.Header.Get("Authorization") != "Bearer sp-tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"lyrics":{"syncType":"LINE_SYNCED","language":"en","lines":[{"startTimeMs":"1000","words":"From Spotify"},{"startTimeMs":"4000","words":"Second line"}]}}`))
	})
	mux.HandleFunc("/qqmusic/soso/fcgi-bin/client_search_cp", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"song":{"list":[{"songmid":"qq1","songname":"Song","albumname":"Album","interval":200,"singer":[{"name":"Artist"}]}]}}}`))
	})
	mux.HandleFunc("/qqmusic/lyric/fcgi-bin/fcg_query_lyric_new.fcg", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"retcode":0,"lyric":"[00:01.00]From QQ Music\n[00:04.00]Second"}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ls := newLyricsSources(t)
	a := ls.c.a
	a.lrcred = lyrics.NewLrcRed("test")
	a.lrcred.BaseURL = srv.URL + "/lrcred"
	a.qqmusic = lyrics.NewQQMusic("test")
	a.qqmusic.BaseURL = srv.URL + "/qqmusic"
	a.spotify = spotify.NewClient(nil)
	a.spotify.LyricsURL = srv.URL + "/spotify"
	a.spotify.SetToken(spotify.Token{
		AccessToken: "sp-tok",
		SPDC:        "cookie",
		Expiry:      time.Now().Add(time.Hour),
	})

	spTrack := media.Track{
		ID: "sp:sp1", Source: media.SourceSpotify, Title: "Song", ISRC: "GBAYE0000267",
		Artists: []media.Ref{{Name: "Artist"}}, Album: &media.Ref{Name: "Album"}, DurationMs: 200000,
	}

	only := func(key string) []settings.LyricsSource {
		var srcs []settings.LyricsSource
		for _, k := range settings.LyricsSourceKeys {
			srcs = append(srcs, settings.LyricsSource{Key: k, Off: k != key})
		}
		return srcs
	}

	// 1. lrc.red -> word-synced by ISRC.
	withSources(t, ls, false, only(settings.LyricsLrcRed)...)
	l, err := ls.c.Lyrics(context.Background(), spTrack)
	if err != nil || l.Source != "lrc.red" || !l.WordTimed() {
		t.Fatalf("lrc.red = %+v, %v", l, err)
	}

	// 2. Spotify -> synced by Spotify track ID using sp_dc web session.
	withSources(t, ls, false, only(settings.LyricsSpotify)...)
	gotSp, err := ls.c.Lyrics(context.Background(), spTrack)
	if err != nil || gotSp.Source != "Spotify" || !gotSp.Synced || gotSp.Lines[0].Text != "From Spotify" {
		t.Fatalf("Spotify = %+v, %v", gotSp, err)
	}

	// 3. QQ Music -> synced by title/artist.
	withSources(t, ls, false, only(settings.LyricsQQMusic)...)
	gotQQ, err := ls.c.Lyrics(context.Background(), aSong)
	if err != nil || gotQQ.Source != "QQ Music" || gotQQ.Lines[0].Text != "From QQ Music" {
		t.Fatalf("QQ Music = %+v, %v", gotQQ, err)
	}
}

func TestSearchMultiCandidatesPreviewAndCandidateIDPick(t *testing.T) {
	var qqLazyCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/lrclib/search", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[
			{"id":101,"trackName":"Yellow","artistName":"Coldplay","albumName":"Parachutes","duration":267,"syncedLyrics":"[00:01.00]Version A\n[00:04.00]Two"},
			{"id":102,"trackName":"Yellow (Live)","artistName":"Coldplay","albumName":"Live 2012","duration":280,"syncedLyrics":"[00:01.00]Version B\n[00:04.00]Two"}
		]`))
	})
	mux.HandleFunc("/lrclib/get/102", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":102,"trackName":"Yellow (Live)","artistName":"Coldplay","albumName":"Live 2012","duration":280,"syncedLyrics":"[00:01.00]Version B\n[00:04.00]Two"}`))
	})
	mux.HandleFunc("/qqmusic/soso/fcgi-bin/client_search_cp", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"song":{"list":[
			{"songmid":"qq1","songname":"Yellow","albumname":"Parachutes","interval":267,"singer":[{"name":"Coldplay"}]},
			{"songmid":"qq2","songname":"Yellow (Remix)","albumname":"Remix","interval":270,"singer":[{"name":"Coldplay"}]},
			{"songmid":"qq3","songname":"Yellow (Radio)","albumname":"Radio","interval":250,"singer":[{"name":"Coldplay"}]},
			{"songmid":"qq4","songname":"Yellow Acoustic","albumname":"Acoustic","interval":240,"singer":[{"name":"Coldplay"}]}
		]}}}`))
	})
	mux.HandleFunc("/qqmusic/lyric/fcgi-bin/fcg_query_lyric_new.fcg", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("songmid") {
		case "qq1", "qq2", "qq3":
			w.Write([]byte(`{"retcode":0,"lyric":"[00:01.00]QQ Top\n[00:04.00]Line"}`))
		case "qq4":
			if qqLazyCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.Write([]byte(`{"retcode":0,"lyric":"[00:01.00]LazyQQ\n[00:04.00]Line"}`))
		default:
			w.Write([]byte(`{"retcode":-1901,"lyric":""}`))
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ls := newLyricsSources(t)
	withPicks(t, ls)
	ls.c.a.lyrics.BaseURL = srv.URL + "/lrclib"
	ls.c.a.qqmusic = lyrics.NewQQMusic("test")
	ls.c.a.qqmusic.BaseURL = srv.URL + "/qqmusic"

	withSources(t, ls, false,
		settings.LyricsSource{Key: settings.LyricsLRCLIB},
		settings.LyricsSource{Key: settings.LyricsQQMusic},
		settings.LyricsSource{Key: settings.LyricsBetterLyrics, Off: true},
		settings.LyricsSource{Key: settings.LyricsUnison, Off: true},
		settings.LyricsSource{Key: settings.LyricsLrcRed, Off: true},
		settings.LyricsSource{Key: settings.LyricsSpotify, Off: true},
		settings.LyricsSource{Key: settings.LyricsYouTubeMusic, Off: true},
		settings.LyricsSource{Key: settings.LyricsCaptions, Off: true},
		settings.LyricsSource{Key: settings.LyricsYouTubePage, Off: true},
	)

	track := aSong
	track.DurationMs = 267000
	found := ls.c.SearchLyrics(context.Background(), track, "Yellow", "Coldplay")
	// 2 LRCLIB + 4 QQ Music (top 3 preloaded, #4 lazy) = 6.
	if len(found) != 6 {
		t.Fatalf("len(found) = %d, want 6: %+v", len(found), found)
	}
	if !found[0].Loaded || !found[1].Loaded || found[1].Pick.CandidateID != "102" {
		t.Errorf("LRCLIB candidates = %+v, %+v", found[0], found[1])
	}
	if !found[2].Loaded || found[5].Loaded || found[5].Pick.CandidateID != "qq4" {
		t.Errorf("QQ Music candidates (top 3 loaded, #4 lazy) = %+v, %+v", found[2], found[5])
	}

	// First preview fails with 502; second preview retries and succeeds without caching the error.
	if _, err := ls.c.PreviewLyricsCandidate(context.Background(), track, found[5].Pick); err == nil {
		t.Fatal("expected first lazy preview to fail with 502")
	}
	prev, err := ls.c.PreviewLyricsCandidate(context.Background(), track, found[5].Pick)
	if err != nil || !strings.Contains(prev.Lines[0].Text, "LazyQQ") {
		t.Fatalf("Retry Preview = %+v, err=%v", prev, err)
	}
	beforePickCalls := qqLazyCalls.Load()
	if err := ls.c.PickLyrics(track, found[5].Pick, &prev); err != nil {
		t.Fatal(err)
	}
	got, err := ls.c.Lyrics(context.Background(), track)
	if err != nil || got.Lines[0].Text != "LazyQQ" || qqLazyCalls.Load() != beforePickCalls {
		t.Errorf("picked cached = %+v, err=%v, calls=%d (want %d)", got, err, qqLazyCalls.Load(), beforePickCalls)
	}

	// Pick LRCLIB candidate 102 and verify persistence across restart fetches /get/102.
	dir := filepath.Join(t.TempDir(), "picks")
	ls.c.a.lyricsPicks = newLyricsPicks(dir)
	if err := ls.c.PickLyrics(track, found[1].Pick, nil); err != nil {
		t.Fatal(err)
	}
	ls.c.a.lyricsAsked = newMemo[*lyricsAsked](8)
	ls.c.a.lyricsPicks = newLyricsPicks(dir)
	got2, err := ls.c.Lyrics(context.Background(), track)
	if err != nil || got2.Lines[0].Text != "Version B" {
		t.Errorf("re-fetched CandidateID 102 = %+v, err=%v", got2, err)
	}
}
