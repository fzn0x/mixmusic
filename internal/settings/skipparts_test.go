package settings

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/sponsorblock"
)

// TestSkipParts has new installs, and the settings of earlier versions,
// skip no parts, with Non-music parts, Sponsor and Self-promotion ticked
// and the highlight shown once the user turns it on; it keeps what the
// user ticked, of the kinds it knows, in their order.
func TestSkipParts(t *testing.T) {
	first := []sponsorblock.Kind{sponsorblock.NonMusic, sponsorblock.Sponsor, sponsorblock.SelfPromo}
	for file, want := range map[string]SkipParts{
		`{}`:                                   {Kinds: first, Highlight: true},
		`{"skipParts":{"on":true}}`:            {On: true, Kinds: first, Highlight: true},
		`{"skipParts":{"on":true,"kinds":[]}}`: {On: true, Kinds: []sponsorblock.Kind{}, Highlight: true},
		`{"skipParts":{"kinds":["filler","intro","chapter","intro"],"highlight":false}}`: {Kinds: []sponsorblock.Kind{sponsorblock.Intro, sponsorblock.Filler}},
		`{"skipParts":{"server":"https://sponsor.example/"}}`:                            {Kinds: first, Highlight: true, Server: "https://sponsor.example"},
		`{"skipParts":{"server":"http://sponsor.example"}}`:                              {Kinds: first, Highlight: true},
	} {
		s := Default()
		if err := json.Unmarshal([]byte(file), &s); err != nil {
			t.Fatal(err)
		}
		got := s.Clean().SkipParts
		if got.On != want.On || !slices.Equal(got.Kinds, want.Kinds) || got.Highlight != want.Highlight || got.Server != want.Server {
			t.Errorf("%s: %+v, want %+v", file, got, want)
		}
	}
	if s := Default(); s.SkipParts.Skips(sponsorblock.Filler) || !s.SkipParts.Skips(sponsorblock.Sponsor) {
		t.Errorf("skips %v at first", s.SkipParts.Kinds)
	}
}
