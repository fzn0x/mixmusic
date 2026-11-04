package romanize

import "testing"

// TestRomanizesLaoKhmerBurmese tells the lines in Lao, Khmer and Burmese
// of lyrics in their languages, and of lyrics whose language isn't known.
func TestRomanizesLaoKhmerBurmese(t *testing.T) {
	for _, c := range []struct {
		in, lang string
		want     bool
	}{
		{"ຂ້ອຍຮັກເຈົ້າ", "lo", true},
		{"ខ្ញុំស្រលាញ់អូន", "km", true},
		{"မင်းကို ချစ်တယ်", "my", true},
		{"ខ្ញុំស្រលាញ់អូន", "", true},
		{"ຂ້ອຍຮັກເຈົ້າ", "km", false},
		{"ฉันรักเธอ", "lo", false},
		{"Paper kites", "my", false},
	} {
		if got := Romanizes(c.in, c.lang); got != c.want {
			t.Errorf("%q (%s): %v", c.in, c.lang, got)
		}
	}
}

// TestLaoKhmerBurmeseMixed keeps each script's run apart, and the Latin
// letters of a line as they are.
func TestLaoKhmerBurmeseMixed(t *testing.T) {
	for _, c := range []struct{ in, lang, want string }{
		{"ສະບາຍດີ ไทย ខ្មែរ မြန်မာ", "", "sa bay di thai khmêr myan ma"},
		{"baby ຂ້ອຍຮັກເຈົ້າ", "lo", "baby khoy hak chao"},
		{"Oh oh មក", "km", "Oh oh môk"},
		{"ငါ baby", "my", "nga baby"},
	} {
		if got := Line(c.in, c.lang); got != c.want {
			t.Errorf("%q: %q, want %q", c.in, got, c.want)
		}
	}
}
