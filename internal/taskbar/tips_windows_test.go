//go:build windows && (amd64 || arm64)

package taskbar

import (
	"slices"
	"syscall"
	"testing"
)

// TestTips shows the tooltips the app gives, in its language, and the
// English of those it gives none of.
func TestTips(t *testing.T) {
	b := &bar{}
	shown := func(st state) (tips []string) {
		for _, btn := range b.buttons(st) {
			tips = append(tips, syscall.UTF16ToString(btn.tip[:]))
		}
		return tips
	}
	fr := Tips{Play: "Lire", Pause: "Pause", Previous: "Précédent", Next: "Suivant"}
	if got := shown(state{tips: fr}); !slices.Equal(got, []string{"Précédent", "Lire", "Suivant"}) {
		t.Errorf("paused, the tooltips are %q", got)
	}
	if got := shown(state{playing: true, track: true, tips: Tips{Next: "Suivant"}}); !slices.Equal(got, []string{"Previous", "Pause", "Suivant"}) {
		t.Errorf("playing, the tooltips are %q", got)
	}
	// A change of the tips alone is a change to show.
	var changed bool
	b.mu.Lock()
	was := b.want
	b.want.tips = fr
	changed = b.want != was
	b.mu.Unlock()
	if !changed {
		t.Error("new tooltips aren't a change")
	}
}
