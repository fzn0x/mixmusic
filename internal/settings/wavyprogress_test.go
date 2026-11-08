package settings

import "testing"

// TestWavyProgress keeps the seek bar straight in new installs and in the
// settings of earlier versions, which don't have the key, until the user
// asks for the wavy one (#382).
func TestWavyProgress(t *testing.T) {
	for file, want := range map[string]bool{`{}`: false, `{"sidebarCollapsed":true}`: false, `{"wavyProgress":true}`: true, `{"wavyProgress":false}`: false} {
		s, err := Decode([]byte(file))
		if err != nil {
			t.Fatal(err)
		}
		if s.Clean().WavyProgress != want {
			t.Errorf("%s draws a wavy progress bar %v, not %v", file, s.WavyProgress, want)
		}
	}
	if Default().WavyProgress {
		t.Error("a new install draws a wavy progress bar")
	}
}
