package settings

import "testing"

// TestNotchPlayer has new installs and the settings of earlier versions
// show the player around the notch, until the user turns it off.
func TestNotchPlayer(t *testing.T) {
	for file, want := range map[string]bool{`{}`: true, `{"notchPlayer":true}`: true, `{"notchPlayer":false}`: false} {
		s, err := Decode([]byte(file))
		if err != nil {
			t.Fatal(err)
		}
		if s.NotchPlayer != want {
			t.Errorf("%s: NotchPlayer %v, not %v", file, s.NotchPlayer, want)
		}
	}
}
