package room

import "testing"

// TestHouseholdKeepsItsPreviews has five computers behind one address
// open the link they were sent, and look before they join: none takes
// another's place.
func TestHouseholdKeepsItsPreviews(t *testing.T) {
	r := newTestRoom(t)
	var all []*Session
	for range 5 {
		all = append(all, knockAndWait(t, r))
	}
	if n := previewsHeld(r); n != 5 {
		t.Fatalf("%d previews held of 5", n)
	}
	for i, s := range all {
		select {
		case <-s.Done():
			t.Fatalf("preview %d ended: %v", i, s.Err())
		default:
		}
	}
}
