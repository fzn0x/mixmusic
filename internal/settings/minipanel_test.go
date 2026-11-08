package settings

import "testing"

// TestMiniPanel opens the mini player without a panel in new installs and
// in the settings of earlier versions, which don't have the key, keeps the
// panels it has, and none for a value it doesn't know (#371).
func TestMiniPanel(t *testing.T) {
	for file, want := range map[string]string{
		`{}`:                        "",
		`{"keepOnTop":true}`:        "",
		`{"miniPanel":"queue"}`:     MiniQueue,
		`{"miniPanel":"lyrics"}`:    MiniLyrics,
		`{"miniPanel":"output"}`:    MiniOutput,
		`{"miniPanel":"error"}`:     "",
		`{"miniPanel":"Queue"}`:     "",
		`{"miniPanel":"something"}`: "",
	} {
		s, err := Decode([]byte(file))
		if err != nil {
			t.Fatal(err)
		}
		if got := s.Clean().MiniPanel; got != want {
			t.Errorf("%s opens the mini player's panel %q, not %q", file, got, want)
		}
	}
	if Default().MiniPanel != "" {
		t.Error("a new install opens the mini player with a panel")
	}
}
