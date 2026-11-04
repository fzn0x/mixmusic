package romanize

import "testing"

// TestBrahmic writes the Brahmic scripts by Hunterian, as each language
// reads its own: Hindi and its kin drop the a a consonant carries where
// it isn't said, Bengali says it o, and Tamil voices its consonants
// between vowels and after nasals.
func TestBrahmic(t *testing.T) {
	for _, c := range []struct{ lang, in, want string }{
		// Hindi: the vowel a dropped at the end, and between a vowel and
		// a consonant with one, by Ohala's rule.
		{"hi", "दिल प्यार कमला समझना बचपन जीवन संगम", "dil pyār kamlā samajhnā bachpan jīvan sangam"},
		{"hi", "भारत हिन्दी नमस्ते आज कल सपना अपना करना", "bhārat hindī namaste āj kal sapnā apnā karnā"},
		// Kept after a cluster ending in r, y or v; and by a nasal before.
		{"hi", "मित्र सत्य ज़िंदगी फ़िल्म", "mitra satya zindagī film"},
		// The nukta's consonants, the candrabindu and ड़.
		{"hi", "ख़ुशी धड़कन मैं हूँ", "khushī dharkan main hūn"},
		// Sanskrit keeps every a.
		{"sa", "संस्कृतम्", "sanskritam"},
		{"bn", "আমি তোমাকে ভালোবাসি", "ami tomake bhalobashi"},
		{"bn", "মন করবে কখন চাঁদ বাংলা কলকাতা ঢাকা", "mon korbe kokhon chand bangla kolkata dhaka"},
		{"as", "গুৱাহাটী", "guwahati"},
		{"pa", "ਪੰਜਾਬੀ ਪੱਕਾ ਇੱਕ ਦਿਲ", "panjābī pakkā ikk dil"},
		{"gu", "અમદાવાદ ગુજરાત", "amdāvād gujrāt"},
		{"or", "ଓଡ଼ିଆ", "oriā"},
		{"ta", "தமிழ் அன்பு பக்கம் நன்றி வெற்றி பாடல் அதை முத்தம் பச்சை", "thamizh anbu pakkam nandri vetri pādal adhai muttham pachchai"},
		{"ta", "தங்கம் நெஞ்சம் கண்டேன் வந்தேன் காதல் மதுரை", "thangam nenjam kandēn vandhēn kādhal madhurai"},
		{"te", "తెలుగు ప్రేమ నువ్వు నేను హైదరాబాద్", "telugu prēma nuvvu nēnu haidarābād"},
		{"kn", "ಕನ್ನಡ ಪ್ರೀತಿ ಬೆಂಗಳೂರು ಮೈಸೂರು", "kannada prīti bengalūru maisūru"},
		{"ml", "മലയാളം എന്റെ എന്ത് സ്നേഹം തിരുവനന്തപുരം", "malayālam ente enthu snēham thiruvananthapuram"},
		{"si", "සිංහල ලංකාව කොළඹ මහනුවර", "sinhala lankāva kolamba mahanuvara"},
		// Digits, and the danda as a full stop.
		{"hi", "दिल १२ बार।", "dil 12 bār."},
	} {
		if got := Line(c.in, c.lang); got != c.want {
			t.Errorf("%s: %q,\nwant %q", c.lang, got, c.want)
		}
	}
}
