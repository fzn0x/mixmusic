package player

import (
	"slices"
	"testing"
)

// TestPlayListInOrder plays a list's first part and adds the rest as it
// comes, after the list's last entry: before what the user queued since
// and before the radio's entries. Once another queue replaced the list,
// it adds nothing.
func TestPlayListInOrder(t *testing.T) {
	h := newHarness(t, Saved{})
	l := h.p.PlayList(tracks("a", "b"), false)
	h.wait("a playing", playing("a"))
	if !l.Add(tracks("c")) {
		t.Fatal("the list isn't in the queue")
	}
	h.p.Enqueue(tracks("x"))
	h.on(func(p *Player) {
		p.s.Queue = append(slices.Clone(p.s.Queue), Item{UID: "r1", Track: track("r"), Radio: true})
	})
	if !l.Add(tracks("d", "e")) || !l.Add(nil) {
		t.Fatal("the list isn't in the queue")
	}
	s := h.p.State()
	if got := queueIDs(s); !slices.Equal(got, []string{"a", "b", "c", "d", "e", "x", "r*"}) || current(s) != "a" {
		t.Fatalf("queue %v, %s current", got, current(s))
	}

	// Played past the list, the rest follows the current entry.
	h.p.PlayNextAt(5)
	h.on(func(*Player) {})
	h.p.Next()
	h.wait("x playing", playing("x"))
	h.on(func(p *Player) { p.s.Queue = slices.Delete(slices.Clone(p.s.Queue), 2, 6) })
	if !l.Add(tracks("f")) {
		t.Fatal("the list isn't in the queue")
	}
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a", "x", "f", "r*"}) {
		t.Fatalf("queue %v", got)
	}

	h.p.PlayTracks(tracks("y", "z"), 0)
	if l.Add(tracks("g")) {
		t.Error("a list another queue replaced took more")
	}
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"y", "z"}) {
		t.Errorf("queue %v", got)
	}
}

// TestPlayListShuffled scatters each part after the first among the
// entries that haven't played, after the current one and before the
// radio's, and keeps the list's order for shuffle off.
func TestPlayListShuffled(t *testing.T) {
	h := newHarness(t, Saved{})
	l := h.p.PlayList(tracks("a", "b", "c"), true)
	h.wait("a song playing", func(p *Player) bool { return p.s.Index == 0 && p.s.Playing })
	h.on(func(p *Player) {
		p.s.Queue = append(slices.Clone(p.s.Queue), Item{UID: "r1", Track: track("r"), Radio: true})
	})
	first := current(h.p.State())
	more := tracks("d", "e", "f", "g", "h", "i", "j", "k")
	for i := 0; i < len(more); i += 2 {
		if !l.Add(more[i : i+2]) {
			t.Fatal("the list isn't in the queue")
		}
	}
	s := h.p.State()
	got := queueIDs(s)
	if len(got) != 12 || got[0] != first || current(s) != first || got[11] != "r*" {
		t.Fatalf("queue %v, %s current", got, current(s))
	}
	if sorted := slices.Sorted(slices.Values(got[:11])); !slices.Equal(sorted, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}) {
		t.Errorf("the list's songs are %v", sorted)
	}

	h.p.SetShuffle(false)
	h.on(func(*Player) {})
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "r*"}) {
		t.Errorf("unshuffled, the queue is %v", got)
	}
}

// TestScatter keeps the entries before from and from to on in place, and
// those between in their order, with the new ones among them.
func TestScatter(t *testing.T) {
	q := []Item{{UID: "p"}, {UID: "c"}, {UID: "1"}, {UID: "2"}, {UID: "3"}, {UID: "r"}}
	add := []Item{{UID: "x"}, {UID: "y"}}
	seen := map[int]bool{}
	for range 200 {
		out := scatter(q, 2, 5, add)
		var uids, old []string
		for i, it := range out {
			uids = append(uids, it.UID)
			if it.UID == "x" {
				seen[i] = true
			}
			if it.UID >= "1" && it.UID <= "3" {
				old = append(old, it.UID)
			}
		}
		if len(out) != 8 || uids[0] != "p" || uids[1] != "c" || uids[7] != "r" || !slices.Equal(old, []string{"1", "2", "3"}) {
			t.Fatalf("scattered: %v", uids)
		}
	}
	if len(seen) != 5 {
		t.Errorf("x went only to %v", seen)
	}
}
