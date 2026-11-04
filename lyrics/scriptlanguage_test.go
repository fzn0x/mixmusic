package lyrics

import "testing"

// TestGuessScriptLanguage tells the languages of Cyrillic by the letters
// only they write, and Assamese and Odia by theirs.
func TestGuessScriptLanguage(t *testing.T) {
	for want, text := range map[string]string{
		"be": "Папяровы змей над залівам\nСонца ўзышло",
		"bg": "Хвърчило от хартия над залива\nПът към щастието",
		"mk": "Змеј од хартија над заливот\nЌе те чекам",
		"sr": "Змај од папира изнад залива\nЂак и ћерка",
		"kk": "Қағаз батпырауық шығанақ үстінде",
		"ky": "Кагаз учурма булуңдун үстүндө",
		"mn": "Цаасан шувуу булангийн дээгүүр өнгөрөв",
		"ru": "Бумажный змей над заливом\nПодъезд и съёмка",
		"as": "উপসাগৰৰ ওপৰত কাগজৰ চিলা",
		"or": "ଉପସାଗର ଉପରେ କାଗଜ ଗୁଡ଼ି",
	} {
		if got := GuessLanguage(lines(text)); got != want {
			t.Errorf("%s: guessed %q", want, got)
		}
	}
}

// TestScriptLanguage tells the language of the lines of lyrics in another
// script than Latin letters, among lines in English.
func TestScriptLanguage(t *testing.T) {
	for want, text := range map[string]string{
		"ko": "Paper kites over the bay\nI know you're waiting for the night\n널 기다릴게",
		"ja": "Paper kites over the bay\nずっと待ってる",
		"uk": "Paper kites over the bay\nЇжак і ґанок",
		"":   "Paper kites over the bay\nI know you're waiting",
	} {
		if got := ScriptLanguage(lines(text)); got != want {
			t.Errorf("%q: %q, want %q", text, got, want)
		}
	}
}
