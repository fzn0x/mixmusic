package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// TestAssistantToastWords says what an AI assistant did in en-XA: each
// toast is one message of the catalog, with what it played named inside.
func TestAssistantToastWords(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	song := media.Track{ID: "yt:a", Title: "Teardrop", Artists: []media.Ref{{Name: "Massive Attack"}}}
	for _, f := range []assistantFound{{Kind: kindSong, Track: song}, {Kind: kindAlbum, Title: "Mezzanine"},
		{Kind: kindArtist, Title: "Massive Attack"}, {Kind: kindPlaylist, Title: "Trip-hop"}} {
		toast := locales.AssistantsToastPlayed(foundSaid("pl:1", f))
		if !strings.HasPrefix(toast, "[") || strings.Contains(toast, "] [") {
			t.Errorf("%s: %q, which isn't one message of the catalog", f.Kind, toast)
		}
		if name := f.Title + song.Title; !strings.Contains(toast, f.Title) || f.Kind == kindSong && !strings.Contains(toast, song.Title) {
			t.Errorf("%s: %q doesn't name %q", f.Kind, toast, name)
		}
	}
	if got := foundSaid(likedID, assistantFound{Kind: kindPlaylist}); got != locales.AssistantsNameLiked() {
		t.Errorf("the liked songs: %q", got)
	}
	if got := namedSaid(media.Track{Title: "Untitled"}); got != locales.AssistantsNameTitle("Untitled") {
		t.Errorf("a track of no artists: %q", got)
	}
}
