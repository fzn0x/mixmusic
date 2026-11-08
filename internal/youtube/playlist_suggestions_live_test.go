package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLivePlaylistSuggestions reads the songs YouTube Music suggests for a
// playlist of a real account, and five more by the refresh, which changes
// nothing: the playlist MIXMUSIC_YT_PLAYLIST names, or the account's first
// playlist of its own. MIXMUSIC_YT_RECORD names a folder to keep the
// answers in, for testdata:
//
//	MIXMUSIC_LIVE=1 MIXMUSIC_YT_COOKIES='SAPISID=…; …' MIXMUSIC_YT_RECORD=dir go test ./internal/youtube -run LivePlaylistSuggestions -v
func TestLivePlaylistSuggestions(t *testing.T) {
	header := os.Getenv("MIXMUSIC_YT_COOKIES")
	if os.Getenv("MIXMUSIC_LIVE") == "" || header == "" {
		t.Skip("set MIXMUSIC_LIVE=1 and MIXMUSIC_YT_COOKIES to read the suggestions of a signed-in account's playlist")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New()
	cookies, err := http.ParseCookie(header)
	if err != nil {
		t.Fatal(err)
	}
	c.SetCookies(cookies)
	id := os.Getenv("MIXMUSIC_YT_PLAYLIST")
	if id == "" {
		playlists, err := c.LibraryPlaylists(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range playlists {
			if _, native, _ := media.ParseID(p.ID); p.Editable && native != "LM" {
				id = native
				break
			}
		}
		if id == "" {
			t.Skip("the account has no playlist of its own")
		}
	}
	record := func(name string, n node) {
		dir := os.Getenv("MIXMUSIC_YT_RECORD")
		if dir == "" {
			return
		}
		data, err := json.MarshalIndent(n.v, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	page, err := c.browse(ctx, "VL"+id)
	if err != nil {
		t.Fatal(err)
	}
	record("playlist-own", page)
	p, _, err := parsePlaylist(page, id)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: editable %v, %d tracks, suggest %q", p.Title, p.Editable, len(p.Tracks), p.Suggest)
	if !p.Editable || p.Suggest == "" {
		t.Fatalf("the playlist of the account's own gives no token of suggestions")
	}
	token := p.Suggest
	for _, name := range []string{"playlist-suggestions", "playlist-suggestions-refresh"} {
		n, err := c.call(ctx, "browse", map[string]any{"continuation": token}, nil)
		if err != nil {
			t.Fatal(err)
		}
		record(name, n)
		tracks, refresh := parsePlaylistSuggestions(n)
		for _, tr := range tracks {
			t.Logf("  %s: %s, %d ms", tr.ID, tr.Title, tr.DurationMs)
		}
		t.Logf("%s: %d songs, refresh %q", name, len(tracks), refresh)
		if len(tracks) == 0 || refresh == "" {
			t.Errorf("%s: %d songs, refresh %q; the answer's keys: %v", name, len(tracks), refresh, answerKeys(n.get("continuationContents")))
			return
		}
		token = refresh
	}
}

// answerKeys returns the keys of an object of an answer, to tell its shape.
func answerKeys(n node) []string {
	m, _ := n.v.(map[string]any)
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
