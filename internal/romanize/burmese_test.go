package romanize

import (
	"slices"
	"testing"
)

// TestBurmeseStandard writes the examples BGN/PCGN's system of 1970 gives
// in its tables and notes, as the UN working group on romanization
// systems prints them (report on the current status of UN romanization
// systems, Burmese, version 4.0, 2013), each run of syllables as the
// standard writes the word.
func TestBurmeseStandard(t *testing.T) {
	for in, want := range map[string]string{
		// Tables III and IV: tall ာ, ု and ူ under medials, ြ and ျ, ှ
		// before its consonant, and their exceptions.
		"ခါ": "ka", "ခေါ်": "kaw", "ကျူ": "kyu", "မျိုး": "myo", "မှ": "hma", "ရှ": "sha", "ချ": "cha", "ခြ": "cha",
		"ကြွ": "kywa", "ကျွ": "kywa", "ဘင်္ဂလားအော်": "Bin-gala Aw",
		// Note 2: a after a consonant without a vowel sign.
		"မဒမ": "madama",
		// Note 3: အ.
		"အကာ": "aga", "အိုဘဲ့": "obè", "အပ်": "at", "မအူ": "ma-u", "သီးပင်အိုင်": "Thibin-aing",
		// Note 4: a vowel letter within a word.
		"ကြေဧ": "kye-e",
		// Note 5: a stacked consonant, its upper letter neither voiced nor
		// voicing a stop after it.
		"ပဉ္စ": "panyza", "အန္တိမဘဝ": "andimabawa", "သေတ္တာ": "thetta", "မိစ္ဆာ": "missa", "ဥက္ကဌ": "ukkada",
		// Note 6, and voicing after a nasal (note 1).
		"အင်းကွတ်": "in-gut", "ရွှေငန်": "shwengan",
	} {
		if got := Line(in, "my"); joined(got) != joined(want) {
			t.Errorf("%s: %q, BGN/PCGN %q", in, got, want)
		}
	}
}

// TestBurmeseNames writes towns whose own names in Latin letters write
// them as BGN/PCGN's rules do: Taunggyi and Myitkyina, voiced after a
// nasal and not after a stop, Sittwe, Pyinmana, Myeik and Hinthada.
func TestBurmeseNames(t *testing.T) {
	for in, want := range map[string]string{
		"တောင်ကြီး": "Taunggyi", "မြစ်ကြီးနား": "Myitkyina", "စစ်တွေ": "Sittwe", "ပျဉ်းမနား": "Pyinmana",
		"မြိတ်": "Myeik", "ဟင်္သာတ": "Hinthada",
	} {
		if got := Line(in, "my"); joined(got) != joined(want) {
			t.Errorf("%s: %q, %q", in, got, want)
		}
	}
}

// TestBurmese parts Burmese into its syllables, a space apart: common
// words, and made-up lines.
func TestBurmese(t *testing.T) {
	for in, want := range map[string]string{
		"မင်္ဂလာပါ": "min ga la ba", "မြန်မာ": "myan ma", "မန္တလေး": "man da le", "ရန်ကုန်": "yan gôn",
		"ရွှေတိဂုံ": "shwe di gôn", "ချစ်တယ်": "chit tè", "အချစ်": "a gyit", "နှလုံးသား": "hna lôn tha",
		"မျက်ရည်": "myet yi", "ကောင်းကင်": "kaung gin", "ကြယ်": "kyè", "သီချင်း": "thi gyin", "လည်း": "le",
		"ကျောင်း": "kyaung", "ဝက်": "wet", "ဝတ်": "wut", "ဝန်": "wun", "ဝံ": "wun", "ကွန်": "kun",
		"ကွင်း": "kwin", "ကွက်": "kwet", "နှင့်": "hnin", "ကိုယ်": "ko", "ဧည့်": "e", "ယောက်ျား": "yauk kya",
		"ဗိုလ်ချုပ်": "bo gyôk", "ပြဿနာ": "pya ttha na", "ဦး": "u", "၎င်း": "lagaung", "အိမ်": "ein",
		"မင်းကို ချစ်တယ်": "min go chit tè",
		"ငါ့ရင်ထဲမှာ":     "nga yin dè hma",
		"၁၂ ကြိမ်။":       "12 kyein.",
		"ရွှေ၊ ငွေ baby":  "shwe, ngwe baby",
	} {
		if got := Line(in, "my"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// burmeseUnseen are more common words of Burmese, written by hand from
// BGN/PCGN's tables and notes, which burmeseRun's rules weren't made
// from: how right it is on them is how right it is.
var burmeseUnseen = []struct{ burmese, bgn string }{
	{"နေကောင်းလား", "ne gaung la"}, {"ကျေးဇူး", "kye zu"}, {"ငါ", "nga"}, {"မင်း", "min"}, {"သူ", "thu"},
	{"ကျွန်တော်", "kyun daw"}, {"ကျွန်မ", "kyun ma"}, {"ရေ", "ye"}, {"ထမင်း", "ta min"}, {"စား", "sa"},
	{"သွား", "thwa"}, {"လာ", "la"}, {"အိပ်", "eik"}, {"ငို", "ngo"}, {"ရယ်", "yè"}, {"ပျော်", "pyaw"},
	{"ဝမ်းနည်း", "wun ni"}, {"အလွမ်း", "a lun"}, {"နှလုံး", "hna lôn"}, {"ကြယ်တွေ", "kyè dwe"},
	{"လမင်း", "la min"}, {"နေမင်း", "ne min"}, {"မိုး", "mo"}, {"လေ", "le"}, {"ပန်း", "pan"}, {"မြို့", "myo"},
	{"ရွာ", "ywa"}, {"လမ်း", "lan"}, {"မေမေ", "me me"}, {"ဖေဖေ", "pe be"}, {"ညီအစ်ကို", "nyi it ko"},
	{"သူငယ်ချင်း", "thu ngè gyin"}, {"ချစ်သူ", "chit thu"}, {"အမြဲ", "a myè"}, {"ထာဝရ", "ta wa ya"},
	{"တစ်ယောက်တည်း", "tit yauk ti"}, {"ဘယ်တော့မှ", "bè daw hma"}, {"မျှော်လင့်", "hmyaw lin"},
	{"အိပ်မက်", "eik met"}, {"နံနက်", "nan net"}, {"ကမ္ဘာ", "kam ba"}, {"ဘဝ", "ba wa"},
	{"ပျော်ရွှင်", "pyaw shwin"}, {"စိတ်", "seik"}, {"ယုံကြည်", "yôn gyi"}, {"စောင့်", "saung"},
	{"သတိရ", "tha di ya"}, {"ပင်လယ်", "pin lè"}, {"တောင်", "taung"}, {"ပြည်", "pyi"},
}

// TestBurmeseUnseen tells how right the rules are on words they weren't
// made from.
func TestBurmeseUnseen(t *testing.T) {
	wrong := 0
	for _, c := range burmeseUnseen {
		if got := Line(c.burmese, "my"); got != c.bgn {
			wrong++
			t.Logf("%s: %q, BGN/PCGN %q", c.burmese, got, c.bgn)
		}
	}
	right := 100 * (len(burmeseUnseen) - wrong) / len(burmeseUnseen)
	t.Logf("%d%% of %d words right", right, len(burmeseUnseen))
	if right < 95 {
		t.Errorf("only %d%% of the words are right", right)
	}
}

// TestBurmeseWrong are what BGN/PCGN writes by how a word is said, which
// rules can't tell: ည် said e, as in Kyaukse and နာမည်, nan me; voicing
// where a word, not a syllable, starts, as of တင်, tin, in "thank you";
// and a vowel said short and toneless, as in Pathein and သီချင်း, tha
// chin.
func TestBurmeseWrong(t *testing.T) {
	for in, want := range map[string]string{
		"ကျောက်ဆည်": "kyauk si", "နာမည်": "na mi", "ကျေးဇူးတင်ပါတယ်": "kye zu din ba dè", "ပုသိမ်": "pu thein",
	} {
		if got := Line(in, "my"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// TestBurmeseWords gives each word of a line synced word by word its
// syllables, voiced as the line has them.
func TestBurmeseWords(t *testing.T) {
	line, words := Words("မင်းကို ချစ်တယ်", "my", []string{"မင်း", "ကို", "ချစ်", "တယ်"})
	if line != "min go chit tè" || !slices.Equal(words, []string{"min", "go", "chit", "tè"}) {
		t.Errorf("%q %q", line, words)
	}
}
