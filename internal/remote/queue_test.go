package remote

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// controlled pairs a computer that controls with one controlled, served on
// 127.0.0.1, and starts a session.
func controlled(t *testing.T) (*fakeHost, *Server, *Dialer, *Session) {
	t.Helper()
	h := newFakeHost()
	s, addr := serve(t, h, fast)
	d := newDialer(t, "Laptop", fast)
	peer, err := pair(t, d, addr, h, same)
	if err != nil {
		t.Fatal(err)
	}
	<-h.ended
	return h, s, d, connect(t, d, addr, peer.Key)
}

// TestQueueCommands names the entries of the queue in the states, and
// sends the commands on them, cleaned, to a computer that takes them.
func TestQueueCommands(t *testing.T) {
	h, _, _, sess := controlled(t)
	h.setState(State{Track: &Track{Title: "Low Tide", Entry: "e0"},
		Next: []Track{{Title: "Paper Boats", Entry: "e1"}, {Title: "Glass Harbour", Entry: "e 2"}, {Title: "Salt", Entry: "e3"}}})
	state := func() any { st, _, _ := sess.State(); return st }
	waitFor(t, "the entries", func() bool {
		st, _, _ := sess.State()
		return len(st.Next) == 3 && st.Next[0].Entry == "e1"
	}, state)
	st, _, _ := sess.State()
	if st.Track.Entry != "e0" || st.Next[1].Entry != "" || st.Next[2].Entry != "e3" {
		t.Errorf("entries %+v, %+v", st.Track, st.Next)
	}
	if !sess.Can(FeatureQueue) || !sess.Can(FeatureHandOver) || sess.Can("teleport") {
		t.Errorf("the computer controlled can %q", sess.features)
	}
	sent := []Command{{Op: OpPlayEntry, Entry: "e3"}, {Op: OpRemove, Entry: "e1"}, {Op: OpMove, Entry: "e3", To: "e1", Volume: 9},
		{Op: OpClear, Entry: "e1"}}
	for _, c := range sent {
		if err := sess.Do(c); err != nil {
			t.Fatal(err)
		}
	}
	want := []Command{{Op: OpPlayEntry, Entry: "e3"}, {Op: OpRemove, Entry: "e1"}, {Op: OpMove, Entry: "e3", To: "e1"}, {Op: OpClear}}
	waitFor(t, "the commands", func() bool { return reflect.DeepEqual(h.got(), want) }, func() any { return h.got() })
	for _, c := range []Command{{Op: OpRemove}, {Op: OpPlayEntry, Entry: "e 1"}, {Op: OpMove, Entry: "e1", To: "e1"},
		{Op: OpMove, Entry: "e1"}, {Op: OpRemove, Entry: strings.Repeat("e", 65)}} {
		if err := sess.Do(c); err == nil {
			t.Errorf("sent %+v", c)
		}
	}
}

// TestHandOver hands what plays over: the computer controlled gets its
// queue cleaned, a file without its path, and answers how it took it,
// each hand-over its own answer. A computer removed meanwhile hands
// nothing over.
func TestHandOver(t *testing.T) {
	h, _, d, sess := controlled(t)
	ho := HandOver{Queue: []QueueTrack{
		{Source: "youtube", ID: "yt:aaaaaaaaaaa", Title: "Paper Boats", Artists: []Ref{{ID: "yt:UCx", Name: "Harbour Lights"}},
			Artwork: []Image{{URL: "http://example.com/a.jpg"}, {URL: "https://example.com/a.jpg", Width: 544, Height: 544}}},
		{Source: "file", ID: "file:L1VzZXJzL21lL011c2ljL3NhbHQuZmxhYw", Title: "Salt", Artists: []Ref{{ID: "file:artist:eA", Name: "Kelp"}},
			DurationMs: 200000},
		{Source: "spotify", ID: "sp:4uLU6hMCjMI75M1A2tKUQC", Title: "Glass Harbour", Album: &Ref{Name: "Tides"}},
	}, Index: 1, PositionMs: 61000, Playing: true, Shuffle: true, Repeat: "one"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	taken, err := sess.HandOver(ctx, ho)
	if err != nil {
		t.Fatal(err)
	}
	if taken != (Taken{ID: 1, Kept: 2, Left: 1, CurrentLeft: true}) {
		t.Errorf("taken %+v", taken)
	}
	h.mu.Lock()
	got := h.handed[0]
	h.mu.Unlock()
	if got.Queue[1].ID != "" || got.Queue[1].Title != "Salt" || got.Queue[1].DurationMs != 200000 {
		t.Errorf("the file handed over %+v", got.Queue[1])
	}
	if a := got.Queue[0].Artwork; len(a) != 1 || a[0].URL != "https://example.com/a.jpg" || a[0].Width != 544 {
		t.Errorf("covers %+v", a)
	}
	if got.Index != 1 || got.PositionMs != 61000 || !got.Playing || !got.Shuffle || got.Repeat != "one" || got.Queue[2].ID != ho.Queue[2].ID {
		t.Errorf("handed over %+v", got)
	}
	if taken, err := sess.HandOver(ctx, HandOver{Queue: ho.Queue[:1]}); err != nil || taken.ID != 2 || taken.Kept != 1 {
		t.Errorf("the second hand-over: %+v, %v", taken, err)
	}

	// Removed, it hands nothing over: its session ends.
	h.revoke(d.Identity.Fingerprint())
	if _, err := sess.HandOver(ctx, ho); err == nil {
		t.Error("a computer removed handed over")
	}
	select {
	case <-sess.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the session of a computer removed went on")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.handed) != 2 {
		t.Errorf("%d hand-overs taken", len(h.handed))
	}
}

// TestUnknownIgnored skips a message of a type it doesn't know, and a
// command it doesn't know, and goes on: as an older MixMusic does with a
// newer one's.
func TestUnknownIgnored(t *testing.T) {
	h, s, _, sess := controlled(t)
	for _, m := range []message{{Type: "teleport"}, {Type: "command", Command: &Command{Op: "teleport"}}, {Type: "handover"}} {
		if err := sess.w.send(m, time.Second); err != nil {
			t.Fatal(err)
		}
	}
	if err := sess.Do(Command{Op: OpPause}); err != nil {
		t.Fatal(err)
	}
	want := []Command{{Op: OpPause}}
	waitFor(t, "the pause", func() bool { return reflect.DeepEqual(h.got(), want) }, func() any { return h.got() })
	if s.Sessions() != 1 || sess.Err() != nil {
		t.Errorf("the session ended: %v", sess.Err())
	}
}

// older is a computer controlled whose MixMusic is older: it speaks
// version 1, but its welcome tells no features. It keeps what it gets.
type older struct {
	mu  sync.Mutex
	got []string
}

func (o *older) seen() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.got)
}

// serveOlder runs an older computer controlled on a port of 127.0.0.1,
// for one session, and returns its key and its address.
func serveOlder(t *testing.T) (*older, Fingerprint, string) {
	t.Helper()
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	o := &older{}
	done := make(chan struct{})
	t.Cleanup(func() {
		ln.Close()
		<-done
	})
	go func() {
		defer close(done)
		raw, err := ln.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		tc := tls.Server(raw, (&Server{Identity: id}).tlsConfig())
		if tc.Handshake() != nil {
			return
		}
		w := newWire(tc)
		if _, err := w.expect("hello", 5*time.Second); err != nil {
			return
		}
		w.send(message{Type: "welcome", Name: "Old desktop"}, time.Second)
		w.send(message{Type: "state", State: &State{Track: &Track{Title: "Low Tide"}, Next: []Track{{Title: "Salt"}}}}, time.Second)
		for {
			m, err := w.receive(5 * time.Second)
			if err != nil {
				return
			}
			if m.Type == "ping" {
				continue
			}
			seen := m.Type
			if m.Command != nil {
				seen += " " + m.Command.Op
			}
			o.mu.Lock()
			o.got = append(o.got, seen)
			o.mu.Unlock()
		}
	}()
	return o, id.Fingerprint(), ln.Addr().String()
}

// TestOlderHost neither sends the commands on the queue nor hands over to
// a computer whose MixMusic is older, which its welcome tells: those fail
// here, with ErrOlder, and the rest goes on as before.
func TestOlderHost(t *testing.T) {
	o, key, addr := serveOlder(t)
	d := newDialer(t, "Laptop", fast)
	sess := connect(t, d, addr, key)
	waitFor(t, "the state", func() bool { _, _, ok := sess.State(); return ok }, func() any { return sess.Err() })
	if sess.Can(FeatureQueue) || sess.Can(FeatureHandOver) {
		t.Errorf("an older computer can %q", sess.features)
	}
	if err := sess.Do(Command{Op: OpRemove, Entry: "e1"}); !errors.Is(err, ErrOlder) {
		t.Errorf("a command on the queue: %v", err)
	}
	if _, err := sess.HandOver(context.Background(), HandOver{Queue: []QueueTrack{{Source: "youtube", ID: "yt:a", Title: "Salt"}}}); !errors.Is(err, ErrOlder) {
		t.Errorf("a hand-over: %v", err)
	}
	if err := sess.Do(Command{Op: OpNext}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the next", func() bool { return slices.Equal(o.seen(), []string{"command next"}) }, func() any { return o.seen() })
}

// TestCleanHandOver keeps a hand-over in range: its index in its queue,
// its words as cleanText makes them, IDs that can be some, covers from
// https alone, and the tracks around the one that plays.
func TestCleanHandOver(t *testing.T) {
	artists := make([]Ref, 12)
	for i := range artists {
		artists[i] = Ref{Name: fmt.Sprint("Artist ", i)}
	}
	h := cleanHandOver(HandOver{Queue: []QueueTrack{
		{Source: "youtube", ID: "yt:a b", Title: "Low\x00 Tide\n", Artists: artists, DurationMs: -5, VideoType: "MUSIC_VIDEO_TYPE_OMV"},
		{Source: "file", ID: "file:abc", Title: "Salt", Album: &Ref{ID: "file:album:x", Name: "  "}, Show: &Ref{Name: "Show"}},
	}, Index: 7, PositionMs: -1, Repeat: "sometimes"})
	if h.Index != 1 || h.PositionMs != 0 || h.Repeat != "" {
		t.Errorf("cleaned %+v", h)
	}
	a, b := h.Queue[0], h.Queue[1]
	if a.ID != "" || a.Title != "Low Tide" || len(a.Artists) != 10 || a.DurationMs != 0 || a.VideoType != "MUSIC_VIDEO_TYPE_OMV" {
		t.Errorf("the first %+v", a)
	}
	if b.ID != "" || b.Album != nil || b.Show == nil || b.Show.Name != "Show" {
		t.Errorf("the file %+v", b)
	}
	if h := cleanHandOver(HandOver{Index: -3}); h.Index != 0 || len(h.Queue) != 0 {
		t.Errorf("nothing handed over: %+v", h)
	}

	long := make([]QueueTrack, 1500)
	for i := range long {
		long[i] = QueueTrack{Source: "youtube", ID: fmt.Sprintf("yt:%011d", i), Title: fmt.Sprint(i)}
	}
	h = cleanHandOver(HandOver{Queue: long, Index: 1200})
	if len(h.Queue) != 400 || h.Index != 100 || h.Queue[h.Index].Title != "1200" {
		t.Errorf("a long queue: %d tracks, the one that plays at %d, %q", len(h.Queue), h.Index, h.Queue[h.Index].Title)
	}
	h = cleanHandOver(HandOver{Queue: long, Index: 3})
	if len(h.Queue) != MaxHandOver || h.Index != 3 || h.Queue[MaxHandOver-1].Title != "999" {
		t.Errorf("a long queue from its start: %d tracks, the one that plays at %d", len(h.Queue), h.Index)
	}
}

// TestFitHandOver cuts a hand-over whose message would be longer than the
// protocol takes, around the track that plays.
func TestFitHandOver(t *testing.T) {
	artists := make([]Ref, 10)
	for i := range artists {
		artists[i] = Ref{ID: "yt:" + strings.Repeat("a", 120), Name: strings.Repeat("Name ", 60)}
	}
	q := make([]QueueTrack, MaxHandOver)
	for i := range q {
		q[i] = QueueTrack{Source: "youtube", ID: fmt.Sprintf("yt:%011d", i), Title: fmt.Sprint(i, strings.Repeat(" title", 50)), Artists: artists}
	}
	h := fitHandOver(cleanHandOver(HandOver{Queue: q, Index: 50}))
	data, err := json.Marshal(message{Type: "handover", HandOver: &h})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > maxMessage || len(h.Queue) >= MaxHandOver || !strings.HasPrefix(h.Queue[h.Index].Title, "50 ") {
		t.Errorf("%d bytes, %d tracks, the one that plays %q", len(data), len(h.Queue), h.Queue[h.Index].Title[:8])
	}
	if small := fitHandOver(HandOver{Queue: q[:3], Index: 2}); len(small.Queue) != 3 || small.Index != 2 {
		t.Errorf("a short queue cut: %+v", small)
	}
}
