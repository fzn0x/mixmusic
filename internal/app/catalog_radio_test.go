package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// fakeMusic is YouTube Music as the radio tests need it. search answers
// a search of its songs, and radio the radio of a video, with a recorded
// answer, or nil for HTTP 500. It notes the searches and the radios asked.
type fakeMusic struct {
	search func(r *http.Request, query string) []byte
	radio  func(videoID string) []byte

	mu       sync.Mutex
	searches []string
	radios   []string
}

// newFakeMusic answers as YouTube Music did, signed out: a search that
// names Instant Crush finds its song first, and other searches nothing;
// the radio of its song is of songs, and every other radio is the radio
// of its music video, of videos.
func newFakeMusic(t *testing.T) *fakeMusic {
	songs := recorded(t, "search-songs-of-video")
	ofSong, ofVideo := recorded(t, "next-song"), recorded(t, "next-video")
	return &fakeMusic{
		search: func(_ *http.Request, query string) []byte {
			if strings.Contains(query, "Instant Crush") {
				return songs
			}
			return nothing
		},
		radio: func(videoID string) []byte {
			if videoID == "khnokW3Mw24" {
				return ofSong
			}
			return ofVideo
		},
	}
}

var nothing = []byte(`{}`)

// recorded returns an answer recorded in ../youtube/testdata.
func recorded(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "youtube", "testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// catalog returns a catalog that asks m.
func (m *fakeMusic) catalog(t *testing.T) *Catalog {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			VideoID string `json:"videoId"`
			Query   string `json:"query"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		var answer []byte
		switch r.URL.Path {
		case "/youtubei/v1/search":
			m.note(&m.searches, body.Query)
			answer = m.search(r, body.Query)
		case "/youtubei/v1/next":
			m.note(&m.radios, body.VideoID)
			answer = m.radio(body.VideoID)
		default:
			http.NotFound(w, r)
			return
		}
		if answer == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(answer)
	}))
	t.Cleanup(srv.Close)
	yt := youtube.New()
	yt.BaseURL = srv.URL
	return &Catalog{&App{youtube: yt, matcher: match.New(yt, nil)}}
}

func (m *fakeMusic) note(asked *[]string, what string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	*asked = append(*asked, what)
}

// asked returns the searches and the radios asked so far.
func (m *fakeMusic) asked() (searches, radios []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.searches), slices.Clone(m.radios)
}

func trackIDs(tracks []media.Track) []string {
	out := make([]string, len(tracks))
	for i, t := range tracks {
		out[i] = t.ID
	}
	return out
}

var (
	instantCrushVideo = media.Track{ID: "yt:a5uQMwRMHcs", Source: media.SourceYouTube, Title: "Instant Crush (feat. Julian Casablancas)", Artists: []media.Ref{{Name: "Daft Punk"}}, Video: true}
	instantCrushSong  = media.Track{ID: "yt:khnokW3Mw24", Source: media.SourceYouTube, Title: "Instant Crush (feat. Julian Casablancas)", Artists: []media.Ref{{Name: "Daft Punk"}, {Name: "Julian Casablancas"}}}
	// driveThru is an upload that has no song on YouTube Music.
	driveThru = media.Track{ID: "yt:GPZ5fnYFI4Q", Source: media.SourceYouTube, Title: "My Drive Thru - Santogold, Julian Casablancas, N.E.R.D produced by Pharrell", Artists: []media.Ref{{Name: "MyDriveThru"}}, Video: true}

	// The radio of the song of Instant Crush, but the song, and of its
	// video, but the video.
	songRadio  = []string{"yt:Rgrt_8mXrK8", "yt:RmYCOm4ehKs", "yt:nujn6wbr-e8", "yt:DkKB4KNKr10"}
	videoRadio = []string{"yt:qFLhGq0060w", "yt:H5v3kku4y6Q", "yt:1G4isv_Fylg", "yt:GPZ5fnYFI4Q"}
)

func TestRadioOfASong(t *testing.T) {
	m := newFakeMusic(t)
	found, err := m.catalog(t).Radio(t.Context(), instantCrushSong)
	searches, radios := m.asked()
	if err != nil || !slices.Equal(trackIDs(found), songRadio) {
		t.Errorf("found %v, %v", trackIDs(found), err)
	}
	if len(searches) != 0 || !slices.Equal(radios, []string{"khnokW3Mw24"}) {
		t.Errorf("searched %q, asked the radios of %v", searches, radios)
	}
}

// TestRadioOfAVideoIsOfItsSong: the radio of a music video is the radio
// of its song, without the song, which plays as the video.
func TestRadioOfAVideoIsOfItsSong(t *testing.T) {
	m := newFakeMusic(t)
	found, err := m.catalog(t).Radio(t.Context(), instantCrushVideo)
	searches, radios := m.asked()
	if err != nil || !slices.Equal(trackIDs(found), songRadio) {
		t.Errorf("found %v, %v", trackIDs(found), err)
	}
	if !slices.Equal(searches, []string{"Instant Crush Daft Punk"}) || !slices.Equal(radios, []string{"khnokW3Mw24"}) {
		t.Errorf("searched %q, asked the radios of %v", searches, radios)
	}
}

func TestRadioOfAVideoWhoseSongHasNone(t *testing.T) {
	m := newFakeMusic(t)
	radio := m.radio
	m.radio = func(videoID string) []byte {
		if videoID == "khnokW3Mw24" {
			return nothing
		}
		return radio(videoID)
	}
	found, err := m.catalog(t).Radio(t.Context(), instantCrushVideo)
	_, radios := m.asked()
	// The video's own radio, as before, rather than nothing.
	if err != nil || !slices.Equal(trackIDs(found), videoRadio) {
		t.Errorf("found %v, %v", trackIDs(found), err)
	}
	if !slices.Equal(radios, []string{"khnokW3Mw24", "a5uQMwRMHcs"}) {
		t.Errorf("asked the radios of %v", radios)
	}
}

func TestRadioOfAVideoWhoseSongsRadioFails(t *testing.T) {
	m := newFakeMusic(t)
	m.radio = func(string) []byte { return nil }
	_, err := m.catalog(t).Radio(t.Context(), instantCrushVideo)
	_, radios := m.asked()
	if err == nil || !slices.Equal(radios, []string{"khnokW3Mw24"}) {
		t.Errorf("asked the radios of %v: %v", radios, err)
	}
}

// TestRadioOfAVideoWhoseSongIsSlow: a search that doesn't answer in time
// doesn't hold the radio up for long, and the video's own radio plays.
func TestRadioOfAVideoWhoseSongIsSlow(t *testing.T) {
	defer func(d time.Duration) { songWait = d }(songWait)
	songWait = 50 * time.Millisecond
	m := newFakeMusic(t)
	m.search = func(r *http.Request, _ string) []byte {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		return nothing
	}
	start := time.Now()
	found, err := m.catalog(t).Radio(t.Context(), instantCrushVideo)
	took := time.Since(start)
	searches, radios := m.asked()
	if err != nil || !slices.Equal(trackIDs(found), videoRadio) {
		t.Errorf("found %v, %v", trackIDs(found), err)
	}
	if len(searches) != 1 || !slices.Equal(radios, []string{"a5uQMwRMHcs"}) || took > 3*time.Second {
		t.Errorf("searched %q, asked the radios of %v, in %v", searches, radios, took)
	}
}

// TestRadioOfAnUploadCarriesOnFromASong: an upload with no song carries
// on from the song of the first video its radio suggests that has one.
func TestRadioOfAnUploadCarriesOnFromASong(t *testing.T) {
	m := newFakeMusic(t)
	found, err := m.catalog(t).Radio(t.Context(), driveThru)
	searches, radios := m.asked()
	if err != nil || !slices.Equal(trackIDs(found), append([]string{instantCrushSong.ID}, songRadio...)) {
		t.Errorf("found %v, %v", trackIDs(found), err)
	}
	if len(searches) != 2 || !slices.Equal(radios, []string{"GPZ5fnYFI4Q", "khnokW3Mw24"}) {
		t.Errorf("searched %q, asked the radios of %v", searches, radios)
	}
}

// TestRadioOfVideosWithoutSongs: when none of the first videos has a song
// either, the upload's radio of videos stands.
func TestRadioOfVideosWithoutSongs(t *testing.T) {
	m := newFakeMusic(t)
	m.search = func(*http.Request, string) []byte { return nothing }
	found, err := m.catalog(t).Radio(t.Context(), driveThru)
	searches, radios := m.asked()
	want := []string{"yt:a5uQMwRMHcs", "yt:qFLhGq0060w", "yt:H5v3kku4y6Q", "yt:1G4isv_Fylg"}
	if err != nil || !slices.Equal(trackIDs(found), want) {
		t.Errorf("found %v, %v", trackIDs(found), err)
	}
	// The upload's song, then those of three of its radio's videos.
	if len(searches) != 1+relayTries || !slices.Equal(radios, []string{"GPZ5fnYFI4Q"}) {
		t.Errorf("searched %q, asked the radios of %v", searches, radios)
	}
}

// TestRadioOfAnUploadWhenSearchesFail: once YouTube Music fails a search,
// the upload's radio stands, without more searches.
func TestRadioOfAnUploadWhenSearchesFail(t *testing.T) {
	m := newFakeMusic(t)
	m.search = func(_ *http.Request, query string) []byte {
		if strings.HasPrefix(query, "My Drive Thru") {
			return nothing
		}
		return nil
	}
	found, err := m.catalog(t).Radio(t.Context(), driveThru)
	searches, radios := m.asked()
	if err != nil || len(found) != 4 {
		t.Errorf("found %v, %v", trackIDs(found), err)
	}
	if len(searches) != 2 || !slices.Equal(radios, []string{"GPZ5fnYFI4Q"}) {
		t.Errorf("searched %q, asked the radios of %v", searches, radios)
	}
}

// withASong returns radio, a recorded radio of videos, with the first
// track of songRadio, a song, put in fourth.
func withASong(t *testing.T, radio, songRadio []byte) []byte {
	t.Helper()
	var r, s any
	if err := json.Unmarshal(radio, &r); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(songRadio, &s); err != nil {
		t.Fatal(err)
	}
	panel := func(v any) map[string]any {
		for _, k := range []any{"contents", "singleColumnMusicWatchNextResultsRenderer", "tabbedRenderer", "watchNextTabbedResultsRenderer", "tabs", 0, "tabRenderer", "content", "musicQueueRenderer", "content", "playlistPanelRenderer"} {
			if i, ok := k.(int); ok {
				v = v.([]any)[i]
			} else {
				v = v.(map[string]any)[k.(string)]
			}
		}
		return v.(map[string]any)
	}
	items := panel(r)["contents"].([]any)
	panel(r)["contents"] = slices.Insert(slices.Clone(items), 3, panel(s)["contents"].([]any)[0])
	out, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// uploadWithASong is newFakeMusic, where the radio of driveThru suggests
// the song of Instant Crush among its videos.
func uploadWithASong(t *testing.T) *fakeMusic {
	m := newFakeMusic(t)
	upload := withASong(t, recorded(t, "next-video"), recorded(t, "next-song"))
	radio := m.radio
	m.radio = func(videoID string) []byte {
		if videoID == "GPZ5fnYFI4Q" {
			return upload
		}
		return radio(videoID)
	}
	return m
}

// TestRadioOfAnUploadWithASong: an upload whose radio suggests a song
// among its videos carries on from the radio of that song, with no search
// beyond its own, and Start radio queues that radio rather than one song.
func TestRadioOfAnUploadWithASong(t *testing.T) {
	m := uploadWithASong(t)
	found, err := m.catalog(t).Radio(t.Context(), driveThru)
	searches, radios := m.asked()
	if err != nil || !slices.Equal(trackIDs(found), append([]string{instantCrushSong.ID}, songRadio...)) {
		t.Errorf("found %v, %v", trackIDs(found), err)
	}
	if len(searches) != 1 || !slices.Equal(radios, []string{"GPZ5fnYFI4Q", "khnokW3Mw24"}) {
		t.Errorf("searched %q, asked the radios of %v", searches, radios)
	}
	if got := trackIDs(player.RadioTracks(found, []media.Track{driveThru})); !slices.Equal(got, append([]string{instantCrushSong.ID}, songRadio[:3]...)) {
		t.Errorf("Start radio queues %v", got)
	}
}

// TestRadioOfAnUploadWithASongWhoseRadioFails: when the radio of that song
// fails or is empty, the upload's own radio stands.
func TestRadioOfAnUploadWithASongWhoseRadioFails(t *testing.T) {
	for _, answer := range [][]byte{nil, nothing} {
		m := uploadWithASong(t)
		radio := m.radio
		m.radio = func(videoID string) []byte {
			if videoID == "khnokW3Mw24" {
				return answer
			}
			return radio(videoID)
		}
		found, err := m.catalog(t).Radio(t.Context(), driveThru)
		_, radios := m.asked()
		want := []string{"yt:a5uQMwRMHcs", "yt:qFLhGq0060w", "yt:H5v3kku4y6Q", "yt:khnokW3Mw24", "yt:1G4isv_Fylg"}
		if err != nil || !slices.Equal(trackIDs(found), want) || !slices.Equal(radios, []string{"GPZ5fnYFI4Q", "khnokW3Mw24"}) {
			t.Errorf("song's radio %q: found %v, %v, asked the radios of %v", answer, trackIDs(found), err, radios)
		}
	}
}

// TestRadioQueuesSongs is what Autoplay and Start radio queue of those
// radios: songs, without the upload the song's radio suggests beside
// them, or the videos of a radio that has nothing else.
func TestRadioQueuesSongs(t *testing.T) {
	for _, c := range []struct {
		name    string
		seed    media.Track
		noSongs bool
		want    []string
	}{
		{"a music video", instantCrushVideo, false, songRadio[:3]},
		{"an upload", driveThru, false, append([]string{instantCrushSong.ID}, songRadio[:3]...)},
		{"an upload whose radio has no songs", driveThru, true, []string{"yt:a5uQMwRMHcs", "yt:qFLhGq0060w", "yt:H5v3kku4y6Q", "yt:1G4isv_Fylg"}},
	} {
		m := newFakeMusic(t)
		if c.noSongs {
			m.search = func(*http.Request, string) []byte { return nothing }
		}
		found, err := m.catalog(t).Radio(t.Context(), c.seed)
		if got := trackIDs(player.RadioTracks(found, []media.Track{c.seed})); err != nil || !slices.Equal(got, c.want) {
			t.Errorf("after %s, queued %v, %v", c.name, got, err)
		}
	}
}
