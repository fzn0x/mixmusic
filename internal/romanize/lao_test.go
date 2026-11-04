package romanize

import (
	"slices"
	"strings"
	"testing"
)

// joined is romanized text as a standard prints it, a word at a time:
// without the spaces between syllables Line writes, the hyphens standards
// part some syllables with, and capitals.
func joined(s string) string {
	return strings.ToLower(strings.NewReplacer(" ", "", "-", "").Replace(s))
}

// TestLaoStandard writes the examples BGN/PCGN's system of 1966 gives in
// its notes, as the UN working group on romanization systems prints them
// (report on the current status of UN romanization systems, Lao, version
// 4.0, 2013), each run of syllables as the standard writes the word.
func TestLaoStandard(t *testing.T) {
	for in, want := range map[string]string{
		// ຽ as a final, after ຫ, and between consonants.
		"ຂາຽ": "khay", "ຫຍາບ": "gnap", "ສຽງ": "siang",
		// ຼ, and ໆ.
		"ຫຼາມ": "lam", "ແຊວໆ": "xèoxèo",
		// ຫ before a sonorant is silent; ອ is a vowel's seat.
		"ຫລວງພຣະບາງ": "Louangphrabang", "ອີກ": "ik", "ຊະເອມ": "xa-ém", "ບ້ານແກ້ງອີ": "Ban Kèng-I",
		// ວ: v starting a syllable, o second in a cluster and at the end,
		// ou after i.
		"ທັນວາ": "thanva", "ນາວ": "nao", "ແກວ": "kèo", "ດຽວ": "diao", "ຄິວ": "khiou", "ຕີວ": "tiou",
		// The second of a cluster, said or not.
		"ບ້ານວັດພຣະໄຊ": "Ban Vatphraxai", "ສວາຍ": "soay", "ແຂວງ": "khoèng", "ຂວານ": "khoan",
		// A word written with the a it says, and a letter under ໌.
		"ສະຫວັນນະເຂດ": "Savannakhét", "ເບີຣ໌": "beur",
	} {
		if got := Line(in, "lo"); joined(got) != joined(want) {
			t.Errorf("%s: %q, BGN/PCGN %q", in, got, want)
		}
	}
}

// TestLaoNames writes the provinces of Laos as ISO 3166-2:LA names them,
// by the Commission Nationale de Toponymie's system of 1965 that
// BGN/PCGN's builds on, and the two write alike.
func TestLaoNames(t *testing.T) {
	for in, want := range map[string]string{
		"ອັດຕະປື": "Attapu", "ບໍ່ແກ້ວ": "Bokèo", "ບໍລິຄໍາໄຊ": "Bolikhamxai", "ຈຳປາສັກ": "Champasak",
		"ຫົວພັນ": "Houaphan", "ຄໍາມ່ວນ": "Khammouan", "ຫລວງນໍ້າທາ": "Louang Namtha", "ຫລວງພະບາງ": "Louangphabang",
		"ອຸດົມໄຊ": "Oudômxai", "ຜົ້ງສາລີ": "Phôngsali", "ສາລະວັນ": "Salavan", "ສະຫວັນນະເຂດ": "Savannakhét",
		"ວຽງຈັນ": "Viangchan", "ໄຊຍະບູລີ": "Xaignabouli", "ໄຊສົມບູນ": "Xaisômboun", "ເຊກອງ": "Xékong",
		// Today's spellings, with ຫຼ and ຳ.
		"ຫຼວງພະບາງ": "Louangphabang", "ຫຼວງນ້ຳທາ": "Louang Namtha",
	} {
		if got := Line(in, "lo"); joined(got) != joined(want) {
			t.Errorf("%s: %q, ISO 3166-2 %q", in, got, want)
		}
	}
}

// TestLao parts Lao into its syllables, a space apart: common words, and
// made-up lines.
func TestLao(t *testing.T) {
	for in, want := range map[string]string{
		"ສະບາຍດີ": "sa bay di", "ຂອບໃຈ": "khop chai", "ລາວ": "lao", "ເພງ": "phéng", "ຮັກ": "hak",
		"ຄວາມຮັກ": "khoam hak", "ຫົວໃຈ": "houa chai", "ດວງດາວ": "douang dao", "ທ້ອງຟ້າ": "thong fa",
		"ນ້ຳຕາ": "nam ta", "ຄິດຮອດ": "khit hot", "ເຈົ້າ": "chao", "ຂ້ອຍ": "khoy", "ຢູ່": "you",
		"ໃຫ້": "hai", "ບໍ່": "bo", "ມື້ນີ້": "mu ni", "ຕະຫຼອດໄປ": "ta lot pai", "ເມືອງ": "muang",
		"ເດີນ": "deun", "ເຮົາ": "hao", "ແມ່ນ້ຳຂອງ": "mè nam khong", "ອອກ": "ok", "ເວລາ": "vé la",
		"ສາວງາມ": "sao ngam", "ຫຼາຍກວ່າ": "lay koa", "ມັກວ່າ": "mak va", "ຫວານ": "van", "ໜ້າ": "na",
		"ໃຫຍ່": "gnai", "ເຫງົາ": "ngao", "ຫວດ": "houat", "ເບິ່ງ": "beung", "ເຂີຍ": "kheuy",
		"ຂ້ອຍຮັກເຈົ້າ":          "khoy hak chao",
		"ເຈົ້າຢູ່ໃສ ຂ້ອຍຄິດຮອດ": "chao you sai khoy khit hot",
		"ຮ້ອງເພງ ໑໒ ເທື່ອ":      "hong phéng 12 thua",
		"ດາວ, ເດືອນ ແລະ baby":   "dao, duan lè baby",
	} {
		if got := Line(in, "lo"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// laoUnseen are more common words of Lao, written by hand from BGN/PCGN's
// tables, which laoRun's rules weren't made from: how right it is on them
// is how right it is.
var laoUnseen = []struct{ lao, bgn string }{
	{"ແມວ", "mèo"}, {"ໝາ", "ma"}, {"ປາ", "pa"}, {"ໄກ່", "kai"}, {"ນົກ", "nôk"}, {"ຊ້າງ", "xang"},
	{"ມ້າ", "ma"}, {"ເສືອ", "sua"}, {"ກາເຟ", "ka fé"}, {"ໝາກໄມ້", "mak mai"}, {"ໝາກມ່ວງ", "mak mouang"},
	{"ກ້ວຍ", "kouay"}, {"ເຮືອນ", "huan"}, {"ປະຕູ", "pa tou"}, {"ໂຕະ", "tô"}, {"ຕັ່ງ", "tang"},
	{"ເກີບ", "keup"}, {"ເສື້ອ", "sua"}, {"ໝວກ", "mouak"}, {"ສີແດງ", "si dèng"}, {"ສີຂຽວ", "si khiao"},
	{"ສີຂາວ", "si khao"}, {"ສີດຳ", "si dam"}, {"ໜຶ່ງ", "nung"}, {"ສອງ", "song"}, {"ສາມ", "sam"},
	{"ສີ່", "si"}, {"ຫ້າ", "ha"}, {"ຫົກ", "hôk"}, {"ເຈັດ", "chét"}, {"ແປດ", "pèt"}, {"ເກົ້າ", "kao"},
	{"ສິບ", "sip"}, {"ຮ້ອຍ", "hoy"}, {"ພັນ", "phan"}, {"ລ້ານ", "lan"}, {"ໂຮງຮຽນ", "hông hian"},
	{"ນັກຮຽນ", "nak hian"}, {"ຄູ", "khou"}, {"ພໍ່", "pho"}, {"ແມ່", "mè"}, {"ລູກ", "louk"}, {"ອ້າຍ", "ay"},
	{"ເອື້ອຍ", "uay"}, {"ນ້ອງ", "nong"}, {"ໝູ່", "mou"}, {"ຝົນ", "fôn"}, {"ລົມ", "lôm"}, {"ໄຟ", "fai"},
	{"ເຢັນ", "yén"}, {"ຮ້ອນ", "hon"}, {"ໜາວ", "nao"}, {"ງາມ", "ngam"}, {"ເກົ່າ", "kao"}, {"ໃໝ່", "mai"},
	{"ກິນເຂົ້າ", "kin khao"}, {"ນອນ", "non"}, {"ຍ່າງ", "gnang"}, {"ແລ່ນ", "lèn"}, {"ຫົວເລາະ", "houa lo"},
	{"ຮ້ອງໄຫ້", "hong hai"}, {"ເພື່ອນ", "phuan"}, {"ຄວາມສຸກ", "khoam souk"}, {"ໃຈດີ", "chai di"},
	{"ເສຍໃຈ", "sia chai"}, {"ດີໃຈ", "di chai"}, {"ຂອບໃຈຫຼາຍໆ", "khop chai lay lay"}, {"ພະຈັນ", "pha chan"},
	{"ຕາເວັນ", "ta vén"}, {"ດອກໄມ້", "dok mai"}, {"ທະເລ", "tha lé"}, {"ພູເຂົາ", "phou khao"},
	{"ແມ່ນ້ຳ", "mè nam"}, {"ຖະໜົນ", "tha nôn"}, {"ລົດ", "lôt"}, {"ເຮືອບິນ", "hua bin"},
	{"ໂທລະສັບ", "thô la sap"}, {"ປຶ້ມ", "pum"}, {"ຈົດໝາຍ", "chôt may"}, {"ຫວັງ", "vang"},
	{"ສັນຍາ", "san gna"}, {"ລືມ", "lum"}, {"ຈື່", "chu"}, {"ຄອຍ", "khoy"}, {"ກັບມາ", "kap ma"},
	{"ໄປນຳກັນ", "pai nam kan"}, {"ຕະຫຼອດການ", "ta lot kan"}, {"ຍິ້ມ", "gnim"}, {"ເຕັ້ນ", "tén"},
	{"ເສັ້ນທາງ", "sén thang"}, {"ໂລກ", "lôk"}, {"ຊີວິດ", "xi vit"}, {"ຄວາມຝັນ", "khoam fan"},
	{"ດວງໃຈ", "douang chai"}, {"ສຽງເພງ", "siang phéng"}, {"ກາງຄືນ", "kang khun"}, {"ຕອນເຊົ້າ", "ton xao"},
	{"ມື້ອື່ນ", "mu un"}, {"ມື້ວານ", "mu van"}, {"ຂ້ອຍຮັກເຈົ້າຫຼາຍ", "khoy hak chao lay"},
}

// TestLaoUnseen tells how right the rules are on words they weren't made
// from.
func TestLaoUnseen(t *testing.T) {
	wrong := 0
	for _, c := range laoUnseen {
		if got := Line(c.lao, "lo"); got != c.bgn {
			wrong++
			t.Logf("%s: %q, BGN/PCGN %q", c.lao, got, c.bgn)
		}
	}
	right := 100 * (len(laoUnseen) - wrong) / len(laoUnseen)
	t.Logf("%d%% of %d words right", right, len(laoUnseen))
	if right < 95 {
		t.Errorf("only %d%% of the words are right", right)
	}
}

// TestLaoWrong are what BGN/PCGN writes by the word, which rules can't
// tell: the a a few words say without writing it, and a loanword's
// consonant that ends a syllable and starts the next. The standard writes
// ຂວາວ "khavao", ສວັນນະເຂດ "Savannakhét" and ຄຼິສຕຽນ "Khlitsatian".
func TestLaoWrong(t *testing.T) {
	for in, want := range map[string]string{"ຂວາວ": "khoao", "ສວັນນະເຂດ": "soan na khét", "ຄຼິສຕຽນ": "khli sa tian"} {
		if got := Line(in, "lo"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// TestLaoWords gives each word of a line synced word by word its
// syllables.
func TestLaoWords(t *testing.T) {
	line, words := Words("ຂ້ອຍຮັກເຈົ້າ", "lo", []string{"ຂ້ອຍ", "ຮັກ", "ເຈົ້າ"})
	if line != "khoy hak chao" || !slices.Equal(words, []string{"khoy", "hak", "chao"}) {
		t.Errorf("%q %q", line, words)
	}
}
