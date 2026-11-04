package romanize

import (
	"slices"
	"strings"
	"testing"
)

// TestKhmerStandard writes the examples the notes of the UN's system of
// 1972 give, as the UN working group on romanization systems prints them
// (report on the current status of UN romanization systems, Khmer,
// version 4.0, 2013), each run of syllables as the standard writes the
// word. អ starting a word isn't written, as the notes allow: their
// "'ângk" is "ângk".
func TestKhmerStandard(t *testing.T) {
	for in, want := range map[string]string{
		// Note 1: the two series, and a final without a vowel.
		"ក": "kâ", "ក្រ": "krâ", "គ": "kô", "គ្រ": "krô", "កក": "kâk", "អង្គ": "'ângk",
		// Note 2: ៊ and ៉ change the series.
		"ហ៊ាង": "héang", "ញ៉ង": "nhâng",
		// Note 3: a subscript sets the series, but for the nasals and យ
		// រ ល វ ស; ត's subscript is usually ដ.
		"ខ្នង": "khnâng", "ក្ដី": "kdei", "ក្តី": "kdei", "កន្ធាយ": "kânthéay", "ខ្ពង": "khpông", "ល្អ": "l'â",
		"ថ្ម": "thmâ", "ស្វាយ": "svay", "ទាំង": "teăng", "ខ្ញ": "khnhâ",
		// Note 4: ប with ៉ or a subscript is p.
		"បា": "ba", "ប៉ង": "pâng", "ប៉ាតៅ": "patau", "ប្លែង": "plêng", "ប្អ": "p'â", "ប្រាប់": "prăb",
		// Note 5: អ in a cluster.
		"ក្អែក": "k'êk", "អ្វី": "'vei", "អាង": "ang",
		// Note 6: ់ and ័ shorten the vowel; eă before k, ng and h.
		"បត់": "bát", "ខ្ពស់": "khpós", "ចាក់": "chăk", "ច័ក": "chăk", "រពាក់": "rôpeăk", "មាត់": "moăt",
		"ភ័ព្វ": "phoăpv",
		// Notes 7 and 9: ៌ is r before its consonant, and ៍ isn't written.
		"ធម៌": "thôrm", "ពោធិ៍": "poŭthĭ", "ភូមិ៍": "phumĭ", "បុណ្យ": "bŏny",
	} {
		if got := Line(in, "km"); joined(got) != joined(strings.TrimPrefix(want, "'")) {
			t.Errorf("%s: %q, UN %q", in, got, want)
		}
	}
}

// TestKhmerNames writes the provinces of Cambodia as ISO 3166-2:KH names
// them by BGN/PCGN's system of 1972, the UN's.
func TestKhmerNames(t *testing.T) {
	for in, want := range map[string]string{
		"បន្ទាយមានជ័យ": "Bântéay Méan Choăy", "បាត់ដំបង": "Bătdâmbâng", "កំពង់ចាម": "Kâmpóng Cham",
		"កំពង់ឆ្នាំង": "Kâmpóng Chhnăng", "កំពង់ស្ពឺ": "Kâmpóng Spœ", "កំពង់ធំ": "Kâmpóng Thum", "កំពត": "Kâmpôt",
		"កណ្ដាល": "Kândal", "កោះកុង": "Kaôh Kŏng", "ក្រចេះ": "Krâchéh", "ភ្នំពេញ": "Phnum Pénh",
		"ព្រៃវែង": "Prey Vêng", "ពោធិ៍សាត់": "Poŭthĭsăt", "សៀមរាប": "Siĕmréab", "ព្រះសីហនុ": "Preăh Seihânŭ",
		"ស្វាយរៀង": "Svay Riĕng", "តាកែវ": "Takêv", "ឧត្តរមានជ័យ": "Ŏtdâr Méan Choăy", "ឧត្ដរមានជ័យ": "Ŏtdâr Méan Choăy",
		"កែប": "Kêb", "ប៉ៃលិន": "Pailĭn", "ត្បូងឃ្មុំ": "Tbong Khmŭm",
	} {
		if got := Line(in, "km"); joined(got) != joined(want) {
			t.Errorf("%s: %q, ISO 3166-2 %q", in, got, want)
		}
	}
}

// TestKhmer parts Khmer into its syllables, a space apart: common words,
// and made-up lines.
func TestKhmer(t *testing.T) {
	for in, want := range map[string]string{
		"កម្ពុជា": "kâm pŭ chéa", "ខ្មែរ": "khmêr", "ស្នេហ៍": "snéh", "ចិត្ត": "chĕtt", "បេះដូង": "béh dong",
		"នឹក": "nœ\u0306k", "ស្រី": "srei", "ប្រុស": "prŏs", "ជីវិត": "chi vĭt", "ថ្ងៃ": "thngai", "យប់": "yób",
		"ពេលវេលា": "pél vé léa", "ទឹកភ្នែក": "tœ\u0306k phnêk", "សុបិន": "sŏ bĕn", "អរគុណ": "âr kŭn",
		"ជួប": "chuŏb", "ទៅ": "tŏu", "មក": "môk", "ណាស់": "năs", "ខ្លាំង": "khlăng", "ចាំ": "chăm",
		"ភ្លេង": "phléng", "ចម្រៀង": "châm riĕng", "សង្គម": "sâng kôm", "អង្គរវត្ត": "âng kôr vôtt",
		"សង្សារ": "sâng sar", "សប្បាយ": "sâb bay", "មនុស្ស": "mô nŭss", "សាកល": "sa kâl", "ពិន្ទុ": "pĭn tŭ",
		"វិទ្យា": "vĭt yéa", "សន្តិភាព": "sân tĕ phéap", "ស្តាប់": "sdăb", "ស្ត្រី": "strei", "ក៏": "kâ",
		"កញ្ញា": "kânh nhéa", "ផ្កាយ": "phkay", "ព្រះច័ន្ទ": "preăh chănt", "ស្រមោល": "srâ moŭl",
		"សួស្តី": "suŏ sdei", "ជំរាបសួរ": "chum réab suŏr", "ឯង": "êng", "ឥឡូវ": "ĕ lov",
		"ស្រីស្អាត":       "srei s'at",
		"ខ្ញុំស្រលាញ់អូន": "khnhom srâ loănh on",
		"បងស្រលាញ់អូន":    "bâng srâ loănh on",
		"ខ្ញុំ ១២ ឆ្នាំ។": "khnhom 12 chhnăm.",
		"ស្រលាញ់ៗ":        "srâ loănh loănh",
		"ចាំបងផង\u200Bណា": "chăm bâng phâng na",
		"(ស្រលាញ់) baby":  "(srâ loănh) baby",
		"ទៅរកអូន":         "tŏu rôk on",
		"ថ្ងៃនេះ":         "thngai néh",
	} {
		if got := Line(in, "km"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// khmerUnseen are more common words and phrases of Khmer, written by hand
// from the UN's tables, which khmerRun's rules weren't made from: how
// right it is on them is how right it is.
var khmerUnseen = []struct{ khmer, un string }{
	{"ស្រលាញ់", "srâ loănh"}, {"ស្នេហា", "sné ha"}, {"អូន", "on"}, {"បង", "bâng"}, {"ខ្ញុំ", "khnhom"},
	{"នឹករលឹក", "nœ\u0306k rô lœ\u0306k"}, {"យំ", "yum"}, {"សើច", "saeuch"}, {"ញញឹម", "nhô nhœ\u0306m"},
	{"ភ្លៀង", "phliĕng"}, {"ខ្យល់", "khyál"}, {"ព្រះអាទិត្យ", "preăh a tĭty"}, {"ផ្កា", "phka"},
	{"មេឃ", "mékh"}, {"សមុទ្រ", "sâ mŭtr"}, {"ទឹក", "tœ\u0306k"}, {"ភ្នំ", "phnum"}, {"ផ្ទះ", "phteăh"},
	{"ម្ដាយ", "mday"}, {"ឪពុក", "âu pŭk"}, {"ប្អូន", "p'on"}, {"គ្រួសារ", "kruŏ sar"}, {"ស្រុក", "srŏk"},
	{"ភូមិ", "phu mĭ"}, {"ផ្លូវ", "phlov"}, {"ពេលយប់", "pél yób"}, {"ពេលព្រឹក", "pél prœ\u0306k"},
	{"ថ្ងៃនេះ", "thngai néh"}, {"ថ្ងៃស្អែក", "thngai s'êk"}, {"ម្សិលមិញ", "msĭl mĭnh"},
	{"ស្រស់ស្អាត", "srás s'at"}, {"ល្អ", "l'â"}, {"ធំ", "thum"}, {"តូច", "toch"}, {"ក្ដៅ", "kdau"},
	{"ត្រជាក់", "trâ cheăk"}, {"ទៅណា", "tŏu na"}, {"មកពីណា", "môk pi na"}, {"ចង់", "cháng"},
	{"ស្គាល់", "skoăl"}, {"ភ្លេច", "phléch"}, {"រង់ចាំ", "róng chăm"}, {"ជានិច្ច", "chéa nĭchch"},
	{"រហូត", "rô hot"}, {"តែមួយ", "tê muŏy"}, {"ពីរ", "pir"}, {"បី", "bei"}, {"បួន", "buŏn"},
	{"ប្រាំ", "prăm"}, {"ដប់", "dáb"}, {"រយ", "rôy"}, {"ពាន់", "poăn"}, {"លាន", "léan"},
	{"សុខសប្បាយ", "sŏkh sâb bay"}, {"អរគុណច្រើន", "âr kŭn chraeun"}, {"សុំទោស", "som toŭs"},
	{"លាហើយ", "léa haeuy"}, {"ជំរាបលា", "chum réab léa"}, {"ចម្លើយ", "châm leuy"}, {"សំណួរ", "sâm nuŏr"},
	{"ពិភពលោក", "pĭ phôp loŭk"}, {"ក្ដីស្រមៃ", "kdei srâ mey"}, {"ចម្រៀងស្នេហ៍", "châm riĕng snéh"},
	{"ខ្មែរ", "khmêr"},
}

// TestKhmerUnseen tells how right the rules are on words they weren't made
// from.
func TestKhmerUnseen(t *testing.T) {
	wrong := 0
	for _, c := range khmerUnseen {
		if got := Line(c.khmer, "km"); got != c.un {
			wrong++
			t.Logf("%s: %q, UN %q", c.khmer, got, c.un)
		}
	}
	right := 100 * (len(khmerUnseen) - wrong) / len(khmerUnseen)
	t.Logf("%d%% of %d words right", right, len(khmerUnseen))
	if right < 95 {
		t.Errorf("only %d%% of the words are right", right)
	}
}

// TestKhmerWrong are what the standard writes by the word, which rules
// can't tell: the open syllables of Pali and Sanskrit, a final cluster of
// Sanskrit after a vowel, and words said against their series or their
// spelling. ISO 3166-2:KH writes "Rôtânôkiri", "Môndól Kiri" and "Preăh
// Vihéar"; by the UN's note 1, សមុទ្រ is "sâmŭtr" and ពិភពលោក
// "pĭphôploŭk". ISO writes ស្ទឹងត្រែង "Stoĕng Trêng", its œ with a breve
// as oĕ.
func TestKhmerWrong(t *testing.T) {
	for in, want := range map[string]string{
		"រតនគិរី": "rô tân kĭ ri", "មណ្ឌលគិរី": "môn dôl kĭ ri", "ព្រះវិហារ": "preăh vĭ har", "សមុទ្រ": "sâ mŭ trô",
		// A word's first syllable without a vowel sign after an open long
		// vowel, which rules take for its final: ស្រី រលូន, srei rô lun.
		"ស្រីរលូន": "sreir lun", "ភ្លឺរលោង": "phlœr loŭng",
		"ពិភពលោក": "pĭph pô loŭk", "ស្ទឹងត្រែង": "stœ\u0306ng trêng",
	} {
		if got := Line(in, "km"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// TestKhmerWords gives each word of a line synced word by word its
// syllables, and a syllable timed alone its share of one whose final is
// the next one's subscript's consonant.
func TestKhmerWords(t *testing.T) {
	line, words := Words("បងស្រលាញ់អូន", "km", []string{"បង", "ស្រលាញ់", "អូន"})
	if line != "bâng srâ loănh on" || !slices.Equal(words, []string{"bâng", "srâ loănh", "on"}) {
		t.Errorf("%q %q", line, words)
	}
	line, words = Words("កម្ពុជា", "km", []string{"ក", "ម្ពុ", "ជា"})
	if line != "kâm pŭ chéa" || len(words) != 3 || words[2] != "chéa" {
		t.Errorf("%q %q", line, words)
	}
}
