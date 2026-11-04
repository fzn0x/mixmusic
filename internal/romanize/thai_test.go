package romanize

import "testing"

// thaiWords are common words of Thai, and made-up phrases of them, as
// RTGS writes them, a syllable apart. A few need what only a dictionary
// knows: a consonant both ending a syllable and starting the next, as the
// ท of มหาวิทยาลัย (wit tha), and ทร read as th then r, as in โทรศัพท์.
var thaiWords = []struct{ thai, rtgs string }{
	{"สวัสดี", "sa wat di"},
	{"ขอบคุณ", "khop khun"},
	{"ประเทศไทย", "pra thet thai"},
	{"กรุงเทพ", "krung thep"},
	{"ภาษา", "pha sa"},
	{"อาหาร", "a han"},
	{"น้ำ", "nam"},
	{"ข้าว", "khao"},
	{"บ้าน", "ban"},
	{"รถ", "rot"},
	{"เมือง", "mueang"},
	{"ถนน", "tha non"},
	{"โรงเรียน", "rong rian"},
	{"หนังสือ", "nang sue"},
	{"ความรัก", "khwam rak"},
	{"หัวใจ", "hua chai"},
	{"ดวงดาว", "duang dao"},
	{"ท้องฟ้า", "thong fa"},
	{"ทะเล", "tha le"},
	{"ภูเขา", "phu khao"},
	{"ดอกไม้", "dok mai"},
	{"ต้นไม้", "ton mai"},
	{"แม่น้ำ", "mae nam"},
	{"พระอาทิตย์", "phra a thit"},
	{"พระจันทร์", "phra chan"},
	{"กลางคืน", "klang khuen"},
	{"ตอนเช้า", "ton chao"},
	{"วันนี้", "wan ni"},
	{"พรุ่งนี้", "phrung ni"},
	{"เมื่อวาน", "muea wan"},
	{"เพื่อน", "phuean"},
	{"ครอบครัว", "khrop khrua"},
	{"พ่อ", "pho"},
	{"แม่", "mae"},
	{"ลูก", "luk"},
	{"ร้องเพลง", "rong phleng"},
	{"ฟังเพลง", "fang phleng"},
	{"เต้นรำ", "ten ram"},
	{"ยิ้ม", "yim"},
	{"ร้องไห้", "rong hai"},
	{"คิดถึง", "khit thueng"},
	{"ตลอดไป", "ta lot pai"},
	{"สบาย", "sa bai"},
	{"ขนม", "kha nom"},
	{"ฝน", "fon"},
	{"ลม", "lom"},
	{"ไฟ", "fai"},
	{"เย็น", "yen"},
	{"ร้อน", "ron"},
	{"หนาว", "nao"},
	{"ใหม่", "mai"},
	{"เก่า", "kao"},
	{"สวย", "suai"},
	{"ใจดี", "chai di"},
	{"เสียใจ", "sia chai"},
	{"ดีใจ", "di chai"},
	{"เหนื่อย", "nueai"},
	{"กลัว", "klua"},
	{"อยาก", "yak"},
	{"อยู่", "yu"},
	{"ไม่ได้", "mai dai"},
	{"เท่านั้น", "thao nan"},
	{"ด้วยกัน", "duai kan"},
	{"ทุกวัน", "thuk wan"},
	{"ตลอดกาล", "ta lot kan"},
	{"นาฬิกา", "na li ka"},
	{"โทรศัพท์", "tho ra sap"},
	{"ประตู", "pra tu"},
	{"หน้าต่าง", "na tang"},
	{"สนาม", "sa nam"},
	{"ขนาด", "kha nat"},
	{"สมุด", "sa mut"},
	{"ฉลาด", "cha lat"},
	{"ตลาด", "ta lat"},
	{"สถานี", "sa tha ni"},
	{"มหาวิทยาลัย", "ma ha wit tha ya lai"},
	{"ประชาชน", "pra cha chon"},
	{"รัฐบาล", "rat tha ban"},
	{"เพลงรัก", "phleng rak"},
	{"ฉันรักเธอตลอดไป", "chan rak thoe ta lot pai"},
	{"เราร้องเพลงด้วยกัน", "rao rong phleng duai kan"},
	{"ฝนตกที่ทะเล", "fon tok thi tha le"},
	{"ดวงดาวบนท้องฟ้า", "duang dao bon thong fa"},
	{"หัวใจของฉัน", "hua chai khong chan"},
	{"คิดถึงเธอทุกวัน", "khit thueng thoe thuk wan"},
	{"เดินไปด้วยกัน", "doen pai duai kan"},
	{"ลมหนาวพัดมา", "lom nao phat ma"},
	{"แสงจันทร์", "saeng chan"},
	{"อย่าร้องไห้", "ya rong hai"},
}

// TestThai parts Thai into its syllables and writes them by RTGS, right
// on all but a few words in a hundred, which it names: those a dictionary
// would know.
func TestThai(t *testing.T) {
	wrong := 0
	for _, c := range thaiWords {
		if got := Line(c.thai, "th"); got != c.rtgs {
			wrong++
			t.Logf("%s: %q, RTGS %q", c.thai, got, c.rtgs)
		}
	}
	right := 100 * (len(thaiWords) - wrong) / len(thaiWords)
	t.Logf("%d%% of %d words and phrases right", right, len(thaiWords))
	if right < 94 {
		t.Errorf("only %d%% of the words and phrases are right", right)
	}
}

// thaiUnseen are more common words and phrases of Thai, which thaiRun's
// rules weren't made from: how right it is on them is how right it is.
var thaiUnseen = []struct{ thai, rtgs string }{
	{"แมว", "maeo"},
	{"หมา", "ma"},
	{"ปลา", "pla"},
	{"ไก่", "kai"},
	{"นก", "nok"},
	{"ช้าง", "chang"},
	{"ม้า", "ma"},
	{"เสือ", "suea"},
	{"ผีเสื้อ", "phi suea"},
	{"กาแฟ", "ka fae"},
	{"ขนมปัง", "kha nom pang"},
	{"ผลไม้", "phon la mai"},
	{"มะม่วง", "ma muang"},
	{"กล้วย", "kluai"},
	{"ส้ม", "som"},
	{"ทุเรียน", "thu rian"},
	{"ประตูบ้าน", "pra tu ban"},
	{"ห้องนอน", "hong non"},
	{"เตียง", "tiang"},
	{"หมอน", "mon"},
	{"ผ้าห่ม", "pha hom"},
	{"กระจก", "kra chok"},
	{"ดินสอ", "din so"},
	{"ปากกา", "pak ka"},
	{"โต๊ะ", "to"},
	{"เก้าอี้", "kao i"},
	{"รองเท้า", "rong thao"},
	{"เสื้อผ้า", "suea pha"},
	{"กระเป๋า", "kra pao"},
	{"หมวก", "muak"},
	{"แว่นตา", "waen ta"},
	{"นาฬิกาปลุก", "na li ka pluk"},
	{"สีแดง", "si daeng"},
	{"สีเขียว", "si khiao"},
	{"สีฟ้า", "si fa"},
	{"สีขาว", "si khao"},
	{"สีดำ", "si dam"},
	{"ตัวเลข", "tua lek"},
	{"หนึ่ง", "nueng"},
	{"สอง", "song"},
	{"สาม", "sam"},
	{"สี่", "si"},
	{"ห้า", "ha"},
	{"หก", "hok"},
	{"เจ็ด", "chet"},
	{"แปด", "paet"},
	{"เก้า", "kao"},
	{"สิบ", "sip"},
	{"ร้อย", "roi"},
	{"พัน", "phan"},
	{"ล้าน", "lan"},
	{"ฉันชอบแมว", "chan chop maeo"},
	{"เขาไปตลาด", "khao pai ta lat"},
	{"เรากินข้าวเย็น", "rao kin khao yen"},
	{"น้องสาวร้องเพลง", "nong sao rong phleng"},
	{"ฟ้าสวยมาก", "fa suai mak"},
	{"ดอกไม้บานแล้ว", "dok mai ban laeo"},
}

// TestThaiUnseen tells how right the rules are on words they weren't made
// from.
func TestThaiUnseen(t *testing.T) {
	wrong := 0
	for _, c := range thaiUnseen {
		if got := Line(c.thai, "th"); got != c.rtgs {
			wrong++
			t.Logf("%s: %q, RTGS %q", c.thai, got, c.rtgs)
		}
	}
	right := 100 * (len(thaiUnseen) - wrong) / len(thaiUnseen)
	t.Logf("%d%% of %d words and phrases right", right, len(thaiUnseen))
	if right < 85 {
		t.Errorf("only %d%% of the words and phrases are right", right)
	}
}

// TestThaiMarks reads the marks of Thai: ๆ says the syllable before again,
// ์ silences its letter, and the tones aren't written.
func TestThaiMarks(t *testing.T) {
	for in, want := range map[string]string{
		"เด็กๆ":       "dek dek",
		"จันทร์":      "chan",
		"ไม่ ใช่":     "mai chai",
		"รัก 2 ครั้ง": "rak 2 khrang",
	} {
		if got := Line(in, "th"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}
