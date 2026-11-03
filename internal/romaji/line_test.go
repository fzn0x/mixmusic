package romaji

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ikawaha/kagome-dict/dict"
)

// moduleDictionary is the IPA dictionary of kagome's module, in the Go
// module cache, where `go mod download github.com/ikawaha/kagome-dict/ipa@v1.2.6`
// puts it: the dictionary MixMusic downloads, which tests never download.
func moduleDictionary(t testing.TB) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Skipf("no Go module cache: %v", err)
	}
	path := filepath.Join(strings.TrimSpace(string(out)), "github.com", "ikawaha", "kagome-dict", "ipa@v1.2.6", "ipa.dict")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("the IPA dictionary isn't in the module cache: go mod download github.com/ikawaha/kagome-dict/ipa@v1.2.6 puts it there")
	}
	return path
}

// testRomanizer returns a Romanizer of the module's dictionary, loaded
// once.
func testRomanizer(t testing.TB) *Romanizer {
	t.Helper()
	path := moduleDictionary(t)
	testDictOnce.Do(func() { testDict, testDictErr = dict.LoadDictFile(path) })
	if testDictErr != nil {
		t.Fatal(testDictErr)
	}
	r, err := NewRomanizer(testDict)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

var (
	testDictOnce sync.Once
	testDict     *dict.Dict
	testDictErr  error
)

// TestLine writes lines of Japanese in romaji, kanji read by the
// dictionary, word by word.
func TestLine(t *testing.T) {
	r := testRomanizer(t)
	for _, c := range []struct{ in, want string }{
		{"今日は良い天気", "kyō wa yoi tenki"},
		{"今日はいい天気", "kyō wa ii tenki"},
		{"東京へ行きたい", "tōkyō e ikitai"},
		{"君を愛してる", "kimi o aishiteru"},
		{"こんにちは", "konnichiwa"},
		{"ありがとう", "arigatō"},
		{"思う", "omou"},
		{"先生", "sensei"},
		{"映画を見ました", "eiga o mimashita"},
		{"お母さん", "okāsan"},
		{"食べています", "tabete imasu"},
		{"雪が降っている", "yuki ga futte iru"},
		{"一緒に歌おう", "issho ni utaō"},
		{"明日もきっと晴れるでしょう", "ashita mo kitto hareru deshō"},
		{"コーヒーを飲んだ", "kōhī o nonda"},
		{"大きい", "ōkii"},
		{"夢の中へ", "yume no naka e"},
		{"そうだね", "sō da ne"},
		{"きれいでした", "kirei deshita"},
		{"行かなかった", "ikanakatta"},
		{"忘れちゃった", "wasurechatta"},
		{"本屋", "hon'ya"},
		// Punctuation, Latin letters and digits stay, Japanese
		// punctuation as its Latin kin.
		{"ねえ、どうして？", "nē, dōshite?"},
		{"「愛」と言った", "“ai” to itta"},
		{"I love you 永遠に", "I love you eien ni"},
		{"Summerの日", "Summer no hi"},
		{"君が好き!", "kimi ga suki!"},
		{"ｶﾀｶﾅ", "katakana"},
		// Lines with nothing Japanese need no romaji.
		{"Oh yeah", ""},
		{"", ""},
	} {
		if got := r.Line(c.in); got != c.want {
			t.Errorf("Line(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestKanaLine writes the kana of lines in romaji, and leaves their kanji
// as they are: romaji until the dictionary is there.
func TestKanaLine(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"今日はいい天気", "今日 haii 天気"},
		{"きょうはいいてんき", "kyōhaiitenki"},
		{"君が好き!", "君 ga 好 ki!"},
		{"ねえ、ラブ", "nee, rabu"},
		{"Love you", ""},
	} {
		if got := KanaLine(c.in); got != c.want {
			t.Errorf("KanaLine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
