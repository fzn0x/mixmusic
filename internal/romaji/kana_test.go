package romaji

import "testing"

// TestKana writes kana in Hepburn romaji: small kana, っ, ん before a
// vowel or y, long vowels, katakana and the sounds of loanwords, leaving
// what isn't kana as it is.
func TestKana(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"あいうえお", "aiueo"},
		{"かきくけこ", "kakikukeko"},
		{"さしすせそ", "sashisuseso"},
		{"たちつてと", "tachitsuteto"},
		{"はひふへほ", "hahifuheho"},
		{"がぎぐげご ざじずぜぞ だぢづでど ばびぶべぼ ぱぴぷぺぽ", "gagigugego zajizuzezo dajizudedo babibubebo papipupepo"},
		{"なにぬねの まみむめも やゆよ らりるれろ わをん", "naninuneno mamimumemo yayuyo rarirurero waon"},
		// Small kana join the kana before them.
		{"きゃきゅきょ", "kyakyukyo"},
		{"しゃしゅしょ ちゃちゅちょ じゃじゅじょ", "shashusho chachucho jajujo"},
		{"にゃ ひゃ みゃ りゃ ぎゃ びゃ ぴゃ", "nya hya mya rya gya bya pya"},
		{"ぁ ゃ", "a ya"},
		// っ doubles the consonant after it, and ch is tch.
		{"きって", "kitte"},
		{"がっこう", "gakkō"},
		{"まっちゃ", "matcha"},
		{"ざっし", "zasshi"},
		{"あっ", "a"},
		// ん before a vowel or y is n'.
		{"きんようび", "kin'yōbi"},
		{"れんあい", "ren'ai"},
		{"こんにちは", "konnichiha"},
		{"しんぶん", "shinbun"},
		// Long vowels: ou and oo are ō, uu ū, as ー lengthens.
		{"きょう", "kyō"},
		{"とうきょう", "tōkyō"},
		{"おおきい", "ōkii"},
		{"すうがく", "sūgaku"},
		{"いい", "ii"},
		{"せんせい", "sensei"},
		{"コーヒー", "kōhī"},
		{"ラーメン", "rāmen"},
		{"スーパー", "sūpā"},
		// Katakana, and the sounds of loanwords.
		{"カタカナ", "katakana"},
		{"ファン ティー ディズニー ウィーン ヴァイオリン", "fan tī dizunī wīn vaiorin"},
		{"チェック シェア ジェット", "chekku shea jetto"},
		{"ヴ", "vu"},
		// Halfwidth katakana, as wide, voiced or not.
		{"ｶﾀｶﾅ ｶﾞｰﾙ", "katakana gāru"},
		// Iteration marks repeat the kana before them, voiced or not.
		{"いすゞ", "isuzu"},
		{"こゝろ", "kokoro"},
		// What isn't kana stays as it is.
		{"今日は天気", "今日ha天気"},
		{"Love ラブ 2", "Love rabu 2"},
		{"", ""},
	} {
		if got := Kana(c.in); got != c.want {
			t.Errorf("Kana(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestKanaUnguessed makes long vowels of ー alone, as Romanizer has them
// from the dictionary: ou is a verb's o and u.
func TestKanaUnguessed(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"オモウ", "omou"},
		{"トーキョー", "tōkyō"},
		{"オカーサン", "okāsan"},
		{"ネー", "nē"},
	} {
		if got := kana(katakana(c.in)); got != c.want {
			t.Errorf("kana(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestJapanese tells Japanese by its kana and kanji, and kana from Han.
func TestJapanese(t *testing.T) {
	for _, c := range []struct {
		in             string
		japanese, kana bool
	}{
		{"今日は", true, true},
		{"天気", true, false},
		{"コーヒー", true, true},
		{"ー", true, true},
		{"Love", false, false},
		{"사랑", false, false},
	} {
		if got := Japanese(c.in); got != c.japanese {
			t.Errorf("Japanese(%q) = %v", c.in, got)
		}
		if got := HasKana(c.in); got != c.kana {
			t.Errorf("HasKana(%q) = %v", c.in, got)
		}
	}
}
