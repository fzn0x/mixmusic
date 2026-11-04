package romanize

import "testing"

// TestEthiopic writes Amharic's syllables by BGN/PCGN, the sixth order a
// consonant alone at the end of a word.
func TestEthiopic(t *testing.T) {
	for in, want := range map[string]string{
		"ሰላም":       "selam",
		"አዲስ አበባ":   "adis abeba",
		"አማርኛ ፍቅር":  "amarinya fik'ir",
		"ጤና ይስጥልኝ።": "t'ēna yisit'iliny.",
	} {
		if got := Line(in, "am"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}
