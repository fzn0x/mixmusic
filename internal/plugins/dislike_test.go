package plugins

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/player"
)

// TestDislike has a plugin dislike the track that plays, which the player
// moves on from at once, and take the dislike back with a like.
func TestDislike(t *testing.T) {
	h := newTestHost(t)
	h.install("thumbs", manifest("thumbs"), map[string]string{"index.js": `
const strobe = { id: "yt:1", source: "youtube", title: "Strobe", artists: [{ name: "deadmau5" }], durationMs: 1000, artwork: [] };
const teardrop = { id: "yt:2", source: "youtube", title: "Teardrop", artists: [{ name: "Massive Attack" }], durationMs: 1000, artwork: [] };
export default function activate(api) {
  api.commands.register({ id: "play", title: "Play", run: () => api.player.playTracks([strobe, teardrop], 0) });
  api.commands.register({ id: "dislike", title: "Dislike", run: async () => {
    await api.library.setDisliked(strobe, true);
    api.ui.toast("disliked " + api.library.isDisliked("yt:1"));
  } });
  api.commands.register({ id: "like", title: "Like", run: async () => {
    await api.library.setLiked(strobe, true);
    api.ui.toast("liked " + api.library.isLiked("yt:1") + ", disliked " + api.library.isDisliked("yt:1"));
  } });
}`})
	h.enable("thumbs")
	h.wait("the commands", func() bool { return len(h.Commands()) == 3 })
	run := func(id string) {
		t.Helper()
		for _, c := range h.Commands() {
			if c.ID == "thumbs."+id {
				c.Run()
				return
			}
		}
		t.Fatalf("no command %s", id)
	}
	run("play")
	h.wait("Strobe playing", func() bool {
		cur, ok := h.p.State().Current()
		return ok && cur.Track.ID == "yt:1" && h.p.State().Playing
	})
	run("dislike")
	h.wait("the dislike", func() bool { return h.ui.said("disliked true") })
	h.wait("Teardrop playing", func() bool { return h.p.State().Index == 1 })
	if q := h.p.State().Queue; q[0].Skip != player.SkipDisliked {
		t.Errorf("the queue doesn't pass over Strobe: %q", q[0].Skip)
	}
	run("like")
	h.wait("the like", func() bool { return h.ui.said("liked true, disliked false") })
	h.wait("Strobe played again", func() bool { return h.p.State().Queue[0].Skip == "" })
}
