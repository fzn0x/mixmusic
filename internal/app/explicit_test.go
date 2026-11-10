package app

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// explicitBadge is YouTube Music's explicit badge of a row, as its answers
// give it.
const explicitBadge = `"badges": [{"musicInlineBadgeRenderer": {"icon": {"iconType": "MUSIC_EXPLICIT_BADGE"}}}], `

// TestRadioWithoutExplicit has the radio leave out the songs YouTube Music
// badges explicit while the settings hide explicit content, and keep them
// while they don't. The radio is next.json, recorded, whose third row the
// test badges, as no recorded radio holds an explicit song.
func TestRadioWithoutExplicit(t *testing.T) {
	recorded, err := os.ReadFile(filepath.Join("..", "youtube", "testdata", "next.json"))
	if err != nil {
		t.Fatal(err)
	}
	const row = `"playlistPanelVideoRenderer": {`
	parts := bytes.SplitN(recorded, []byte(row), 4)
	if len(parts) < 4 {
		t.Fatalf("next.json has %d rows", len(parts)-1)
	}
	answer := bytes.Join([][]byte{parts[0], parts[1], parts[2], append([]byte(explicitBadge), parts[3]...)}, []byte(row))
	a := dislikeApp(t, func(w http.ResponseWriter, r *http.Request) { w.Write(answer) },
		func(w http.ResponseWriter, r *http.Request) {})
	c := &Catalog{a}
	ctx := context.Background()
	seed := media.Track{ID: "yt:seed", Source: media.SourceYouTube, Title: "Seed"}
	all, err := c.Radio(ctx, seed)
	if err != nil {
		t.Fatal(err)
	}
	explicit := slices.IndexFunc(all, func(tr media.Track) bool { return tr.Explicit })
	if explicit < 0 || len(all) < 3 {
		t.Fatalf("the radio %v holds no explicit song", trackIDs(all))
	}
	st := a.settings.Get()
	st.HideExplicit = true
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	got, err := c.Radio(ctx, seed)
	want := slices.Delete(slices.Clone(trackIDs(all)), explicit, explicit+1)
	if err != nil || !slices.Equal(trackIDs(got), want) {
		t.Errorf("hiding explicit content, the radio is %v, not %v, %v", trackIDs(got), want, err)
	}
}
