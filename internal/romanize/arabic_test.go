package romanize

import "testing"

// TestArabic reads the vowel points of Arabic where they're written, and
// writes its consonants with the long vowels its letters write where they
// aren't; the article takes the sound of a sun letter.
func TestArabic(t *testing.T) {
	for _, c := range []struct{ lang, in, want string }{
		{"ar", "حَبِيبِي", "habibi"},
		{"ar", "أَنا", "ana"},
		{"ar", "شُكْرًا مَرْحَبًا", "shukran marhaban"},
		{"ar", "الشَّمْس السَّلَامُ عَلَيْكُمْ قَلْبِي", "ash-shams as-salamu alaykum qalbi"},
		{"ar", "حبيبي", "hbibi"},
		{"ar", "الشمس القمر", "ash-shms al-qmr"},
		{"ar", "حبيبي، تعال", "hbibi, t'al"},
		{"fa", "خانه از دل", "khāne az dl"},
		{"fa", "دوست", "dust"},
		{"ur", "دل بھی ہے", "dl bhi he"},
		// The letters tell Persian and Urdu.
		{"", "خانه گل", "khāne gl"},
	} {
		if got := Line(c.in, c.lang); got != c.want {
			t.Errorf("%s: %q, want %q", c.in, got, c.want)
		}
	}
}
