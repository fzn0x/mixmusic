package room

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/dnssd"
)

// TestFoundOfKeepsRoomsOfThisVersion keeps the rooms of the instances that
// tell an ID and this version, their names cleaned.
func TestFoundOfKeepsRoomsOfThisVersion(t *testing.T) {
	insts := []dnssd.Instance{
		{Name: "a", Host: "192.168.1.2", Port: 4000, Text: map[string]string{"id": "ABCDEF", "name": " Desk\x07 ", "v": "1"}},
		{Name: "b", Host: "192.168.1.3", Port: 4001, Text: map[string]string{"id": "older", "name": "Old", "v": "0"}},
		{Name: "c", Host: "192.168.1.4", Port: 4002, Text: map[string]string{"name": "No ID", "v": "1"}},
		{Name: "d", Host: "192.168.1.5", Port: 4003, Text: map[string]string{"id": "has space", "v": "1"}},
	}
	got := foundOf(insts)
	if len(got) != 1 || got[0] != (Found{ID: "abcdef", Name: "Desk", Addr: "192.168.1.2:4000"}) {
		t.Fatalf("found %+v", got)
	}
}
