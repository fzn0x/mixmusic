package romanize

import "testing"

// TestArmenian writes Armenian by BGN/PCGN (1981): ե and ո at the start of
// a word, և, ու and the aspirates' apostrophes.
func TestArmenian(t *testing.T) {
	for in, want := range map[string]string{
		"Երևան Գյումրի Վանաձոր Էջմիածին Ոսկեվազ Արարատ": "Yerevan Gyumri Vanadzor Ejmiatsin Voskevaz Ararat",
		"Չարենցավան Ծաղկաձոր Թալին Փարաքար":             "Ch’arents’avan Tsaghkadzor T’alin P’arak’ar",
		"ով ոսկի և հայ": "ov voski yev hay",
	} {
		if got := Line(in, "hy"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// TestGeorgian writes Georgian by its national system of 2002, its
// ejectives apart from its aspirates by an apostrophe.
func TestGeorgian(t *testing.T) {
	in := "თბილისი ქუთაისი ბათუმი ფოთი ჭიათურა ყვარელი წყალტუბო ცხინვალი ღვინო"
	want := "tbilisi kutaisi batumi poti ch’iatura q’vareli ts’q’alt’ubo tskhinvali ghvino"
	if got := Line(in, "ka"); got != want {
		t.Errorf("%q, want %q", got, want)
	}
	// Mtavruli, the capitals.
	if got := Line("ᲡᲐᲥᲐᲠᲗᲕᲔᲚᲝ", "ka"); got != "SAKARTVELO" {
		t.Errorf("Mtavruli: %q", got)
	}
}
