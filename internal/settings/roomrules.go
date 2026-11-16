package settings

// What becomes of the songs the guests of the user's room add (#734).
const (
	// RoomSongsAdd adds them to the queue at once.
	RoomSongsAdd = "add"
	// RoomSongsAsk has the user accept each first, as at first.
	RoomSongsAsk = "ask"
	// RoomSongsOff takes none.
	RoomSongsOff = "off"
)

// cleanRoomSongs is what becomes of guests' songs, as s tells it: asking
// first for anything else.
func cleanRoomSongs(s string) string {
	switch s {
	case RoomSongsAdd, RoomSongsOff:
		return s
	}
	return RoomSongsAsk
}
