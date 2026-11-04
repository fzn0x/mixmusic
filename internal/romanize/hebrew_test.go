package romanize

import "testing"

// TestHebrew reads Hebrew's vowel points where they're written, by the
// Academy's rules, and writes its consonants with the long vowels their
// letters write where they aren't.
func TestHebrew(t *testing.T) {
	for in, want := range map[string]string{
		"שָׁלוֹם":              "shalom",
		"תּוֹדָה לְךָ":         "toda lekha",
		"אֲנִי אוֹהֵב אוֹתָךְ": "ani ohev otakh",
		"יְרוּשָׁלַיִם":        "yerushalayim",
		"רוּחַ":                "ruah",
		"בְּרֵאשִׁית":          "breshit",
		"שלום":                 "shlom",
		"אני אוהב אותך":        "ani ohv otkh",
	} {
		if got := Line(in, "he"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
	if got := Line("ייִדיש", "yi"); got != "yidish" {
		t.Errorf("Yiddish: %q", got)
	}
}
