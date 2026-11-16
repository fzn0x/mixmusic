package room

import (
	"context"
	"errors"
	"testing"
)

// TestNoJoinsAsTheRoomMoves refuses a computer that joins, or one that
// comes back, while the room moves, and once it moved, as their places
// wouldn't move with it.
func TestNoJoinsAsTheRoomMoves(t *testing.T) {
	r := newTestRoom(t)
	ana, bo, cy := newGuest(t, "Ana"), newGuest(t, "Bo"), newGuest(t, "Cy")
	sa := r.join(ana)
	sb := r.join(bo)
	release := make(chan struct{})
	hostWhenAsked(t, ana, sa, release)
	reachWhenAsked(t, bo, sb, release)
	done := make(chan error, 1)
	if err := r.server.MakeHost(ana.Identity.Fingerprint(), func(_ Move, err error) { done <- err }); err != nil {
		t.Fatal(err)
	}
	if _, err := cy.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false); !errors.Is(err, ErrRoomMoves) {
		t.Fatalf("a knock as the room moves: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := cy.Knock(context.Background(), r.addr, &r.key, r.server.Secret, true); !errors.Is(err, ErrRoomMoves) {
		t.Fatalf("a knock once the room moved: %v", err)
	}
	if _, err := bo.Rejoin(context.Background(), r.addr, r.key); !errors.Is(err, ErrRoomMoves) {
		t.Fatalf("coming back once the room moved: %v", err)
	}
}
