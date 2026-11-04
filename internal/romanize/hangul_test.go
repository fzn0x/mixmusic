package romanize

import "testing"

// TestKorean writes Korean by the Revised Romanization, with its sound
// changes within a word, as the notice of 2000 writes its own examples,
// and made-up lines.
func TestKorean(t *testing.T) {
	for in, want := range map[string]string{
		// The notice's examples: nasalization and liquidization,
		"백마": "baengma", "종로": "jongno", "왕십리": "wangsimni", "별내": "byeollae", "신라": "silla",
		// palatalization, aspiration,
		"해돋이": "haedoji", "같이": "gachi", "굳히다": "guchida", "좋고": "joko", "놓다": "nota",
		"잡혀": "japyeo", "낳지": "nachi",
		// and tensing, which isn't written.
		"압구정": "apgujeong", "낙동강": "nakdonggang", "죽변": "jukbyeon", "낙성대": "nakseongdae",
		"합정": "hapjeong", "팔당": "paldang", "샛별": "saetbyeol", "울산": "ulsan",
		// g, d, b and r at the end and before a consonant.
		"구미": "gumi", "영동": "yeongdong", "옥천": "okcheon", "월곶": "wolgot", "벚꽃": "beotkkot",
		"한밭": "hanbat", "설악": "seorak", "칠곡": "chilgok", "임실": "imsil", "울릉": "ulleung",
		"대관령": "daegwallyeong", "속리산": "songnisan",
		// Liaison, the pairs of finals, and ㅎ.
		"한국어": "hangugeo", "좋다": "jota", "읽어": "ilgeo", "읽고": "ilgo", "좋아": "joa", "않아": "ana",
		"싫어": "sireo", "없어": "eopseo", "앉아": "anja", "국물": "gungmul", "있는": "inneun", "놓는": "nonneun",
		"설날": "seollal", "빨리": "ppalli",
		// Made-up lines, a word at a time, as spaces part them.
		"밤하늘 별빛이 내려": "bamhaneul byeolbichi naeryeo", "종이 연이 날아": "jongi yeoni nara",
	} {
		if got := Line(in, "ko"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// TestKoreanWrong are what the Revised Romanization writes otherwise, as
// it knows words: n added between the parts of a compound, as 학여울,
// Hangnyeoul, and nouns that keep their h, as 묵호, Mukho. They're
// written as said without those.
func TestKoreanWrong(t *testing.T) {
	for in, want := range map[string]string{"학여울": "hagyeoul", "알약": "aryak", "묵호": "muko", "집현전": "jipyeonjeon", "신문로": "sinmullo"} {
		if got := Line(in, "ko"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// TestKoreanMixed keeps the English of a line of Korean, a space apart,
// and writes letters of Hangul alone, as ㅋㅋ, as themselves.
func TestKoreanMixed(t *testing.T) {
	for in, want := range map[string]string{
		"사랑해 baby":   "saranghae baby",
		"사랑해baby":    "saranghae baby",
		"Oh 오늘밤":     "Oh oneulbam",
		"(사랑해)!":     "(saranghae)!",
		"「사랑해」":      "“saranghae”",
		"ㅋㅋㅋ 웃어":     "kkk useo",
		"baby, 내 사랑": "baby, nae sarang",
	} {
		if got := Line(in, "ko"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}
