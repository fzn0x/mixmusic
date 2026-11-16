package app

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// TestRoomGuestTriesForAQuarterHour drops a guest off a room whose host
// went, on a clock a thousand times faster: it tries to come back for
// about 15 minutes, as long as the host keeps its place, then the room
// ended for it.
func TestRoomGuestTriesForAQuarterHour(t *testing.T) {
	was, wasMost := roomRetry, roomRetryMost
	roomRetry, roomRetryMost = time.Millisecond, 32*time.Millisecond
	t.Cleanup(func() { roomRetry, roomRetryMost = was, wasMost })
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	// The host goes, as a computer that sleeps: no one answers where it
	// was.
	desk.ln.Close()
	desk.ln.kill()
	untilGuest(t, lap.App, "away", func(v GuestView) bool { return v.Step == RoomAway })
	start := time.Now()
	for deadline := start.Add(20 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		if v, _ := lap.Rooms().Guest(); v.Step == RoomEnded {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("still coming back")
		}
	}
	// 1, 2, 4, 8 and 16, then 32 for each try left: 15 minutes, a
	// thousand times faster, and the tries' own time.
	if d := time.Since(start); d < 850*time.Millisecond {
		t.Fatalf("the guest gave up after %v: %v at the speed of the room", d, d*1000)
	}
}
