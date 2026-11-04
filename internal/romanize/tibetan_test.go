package romanize

import "testing"

// TestTibetan writes Tibetan by Extended Wylie, the root of a syllable
// without a vowel sign found by its prefixes and suffixes.
func TestTibetan(t *testing.T) {
	for in, want := range map[string]string{
		"བཀྲ་ཤིས་བདེ་ལེགས།": "bkra shis bde legs/",
		"བོད ལྷ་ས":          "bod lha sa",
		"སངས་རྒྱས":          "sangs rgyas",
		"བསྒྲུབས":           "bsgrubs",
		"བསམ་ ལགས་ དགའ་":    "bsam lags dga'",
		"གཡག":               "g.yag",
		"པདྨེ་ཧཱུྃ":         "padme hU~M",
	} {
		if got := Line(in, "bo"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}
