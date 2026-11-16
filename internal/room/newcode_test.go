package room

import (
	"context"
	"errors"
	"testing"
)

// TestNewCodeShutsOutTheOldOne gives the room a new code: the old code and
// link let no one in, a computer blocked that came back under a new key
// included; the new ones do; the guests stay.
func TestNewCodeShutsOutTheOldOne(t *testing.T) {
	r := newTestRoom(t)
	stays := r.join(newGuest(t, "Stays"))
	oldSecret := r.server.Secret
	secret, _ := NewSecret()
	r.server.SetCode("XYZ789", secret)
	// A blocked computer, with a key it made anew, has only the old code.
	again := newGuest(t, "Again")
	if _, err := again.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("the old code: %v", err)
	}
	if _, err := again.Knock(context.Background(), r.addr, &r.key, oldSecret, true); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("the old link: %v", err)
	}
	s, err := again.Knock(context.Background(), r.addr, nil, []byte("XYZ789"), false)
	if err != nil {
		t.Fatalf("the new code: %v", err)
	}
	s.Close()
	s, err = again.Knock(context.Background(), r.addr, &r.key, secret, true)
	if err != nil {
		t.Fatalf("the new link: %v", err)
	}
	s.Close()
	select {
	case <-stays.Done():
		t.Fatalf("a guest left with the new code: %v", stays.Err())
	default:
	}
	if m := r.server.Members(); len(m) != 1 || m[0].Name != "Stays" || !m[0].Here {
		t.Fatalf("members %+v", m)
	}
}

// TestNewCodeEndsTheWaitToJoin proves the old code, and waits to join:
// a new code ends the wait, and Join is refused.
func TestNewCodeEndsTheWaitToJoin(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Early")
	s, err := g.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	secret, _ := NewSecret()
	r.server.SetCode("XYZ789", secret)
	if err := s.Join(context.Background()); !errors.Is(err, ErrNewCode) && !errors.Is(err, ErrLost) {
		t.Fatalf("joining after a new code: %v", err)
	}
	<-s.Done()
	if !errors.Is(s.Err(), ErrNewCode) {
		t.Fatalf("the wait ended with %v", s.Err())
	}
	if len(r.server.Members()) != 0 {
		t.Fatal("it joined with the old code")
	}
}
