package remote

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// patient is fast, but waits long for a silent computer: what ends sooner
// was ended for another reason.
var patient = func() Timing {
	t := fast
	t.Silence = 20 * time.Second
	return t
}()

// dialUnpaired connects to addr over TLS as a computer of a key of its own,
// not paired, as anyone on the network may.
func dialUnpaired(t *testing.T, addr string) *tls.Conn {
	t.Helper()
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	c := tls.Client(raw, id.ClientTLS(alpn, nil, nil))
	t.Cleanup(func() { c.Close() })
	c.SetDeadline(time.Now().Add(10 * time.Second))
	if err := c.Handshake(); err != nil {
		t.Fatal(err)
	}
	c.SetDeadline(time.Time{})
	return c
}

// endedWithin reports whether the other end of c ended it within wait: a
// read fails, but not as its deadline passed.
func endedWithin(c net.Conn, wait time.Duration) bool {
	c.SetReadDeadline(time.Now().Add(wait))
	var b [512]byte
	for {
		if _, err := c.Read(b[:]); err != nil {
			var ne net.Error
			return !errors.As(err, &ne) || !ne.Timeout()
		}
	}
}

// TestUnpairedLongMessageEnds ends the connection of a computer not paired
// that tells a message longer than a pairing's, from its length alone,
// rather than wait for a megabyte (#736).
func TestUnpairedLongMessageEnds(t *testing.T) {
	_, addr := serve(t, newFakeHost(), patient)
	for _, n := range []uint32{MaxMessage, unpairedMessage + 1} {
		c := dialUnpaired(t, addr)
		var head [4]byte
		binary.BigEndian.PutUint32(head[:], n)
		if _, err := c.Write(head[:]); err != nil {
			t.Fatal(err)
		}
		if !endedWithin(c, 5*time.Second) {
			t.Errorf("a computer not paired that told a message of %d bytes was waited for", n)
		}
	}
}

// TestPairedLongMessages has a paired computer's session carry messages
// longer than a pairing's, both ways: a long state, and a long hand-over.
func TestPairedLongMessages(t *testing.T) {
	h, _, _, sess := controlled(t)
	long := func(i int) string { return fmt.Sprint(i, strings.Repeat(" long title", 25)) }
	st := State{Track: &Track{Title: long(0)}, Volume: 1}
	for i := range MaxNext {
		st.Next = append(st.Next, Track{Title: long(i + 1), Artists: []string{strings.Repeat("Artist ", 20)}})
	}
	h.setState(st)
	waitFor(t, "the long state", func() bool {
		got, _, _ := sess.State()
		return len(got.Next) == MaxNext && got.Next[MaxNext-1].Title == long(MaxNext)
	}, func() any { got, _, _ := sess.State(); return len(got.Next) })

	ho := HandOver{Playing: true}
	for i := range 200 {
		ho.Queue = append(ho.Queue, QueueTrack{Source: "youtube", ID: fmt.Sprintf("yt:%011d", i), Title: long(i)})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if taken, err := sess.HandOver(ctx, ho); err != nil || taken.Kept != len(ho.Queue) {
		t.Fatalf("a hand-over of %d tracks: %+v, %v", len(ho.Queue), taken, err)
	}
}
