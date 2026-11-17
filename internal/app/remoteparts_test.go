package app

import (
	"reflect"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
)

// TestRemoteStateParts tells the computer that controls this one the parts
// of the track that plays which the player passes over, and its highlight,
// as the seek bars here mark them: none of a track without.
func TestRemoteStateParts(t *testing.T) {
	st := player.State{Queue: []player.Item{{Track: media.Track{ID: "a", Title: "A"}}}, Index: 0, Duration: 211 * time.Second,
		Parts:     []player.Part{{Kind: sponsorblock.Intro, From: 0, To: 12 * time.Second}, {Kind: sponsorblock.Sponsor, From: 20 * time.Second, To: time.Minute}},
		Highlight: 150 * time.Second}
	want := []remote.Part{{Kind: sponsorblock.Intro, FromMs: 0, ToMs: 12_000}, {Kind: sponsorblock.Sponsor, FromMs: 20_000, ToMs: 60_000},
		{Kind: sponsorblock.Highlight, FromMs: 150_000}}
	if got := remoteStateOf(st).Parts; !reflect.DeepEqual(got, want) {
		t.Errorf("parts %+v, want %+v", got, want)
	}
	st.Parts, st.Highlight = nil, 0
	if got := remoteStateOf(st).Parts; got != nil {
		t.Errorf("a track without parts tells %+v", got)
	}
}
