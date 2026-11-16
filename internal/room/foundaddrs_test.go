package room

import (
	"reflect"
	"testing"

	"github.com/fzn0x/mixmusic/internal/dnssd"
)

// TestFoundKeepsEveryAddress keeps, of a room whose host told its address
// on this computer's network and on a VPN's, both, each with the room's
// port.
func TestFoundKeepsEveryAddress(t *testing.T) {
	got := foundOf([]dnssd.Instance{{Name: "a", Host: "192.168.1.12", Port: 50958, Others: []string{"100.127.209.93", "fd7a:115c:a1e0::1"},
		Text: map[string]string{"id": "abcdef", "name": "Guesty", "v": "1"}}})
	want := Found{ID: "abcdef", Name: "Guesty", Addr: "192.168.1.12:50958", Others: []string{"100.127.209.93:50958", "[fd7a:115c:a1e0::1]:50958"}}
	if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Fatalf("found %+v", got)
	}
}
