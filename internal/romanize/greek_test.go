package romanize

import "testing"

// TestGreek writes Greek by ELOT 743, its stress marks kept, its digraphs
// by what follows them.
func TestGreek(t *testing.T) {
	for in, want := range map[string]string{
		"Αθήνα": "Athína", "Θεσσαλονίκη": "Thessaloníki", "Πειραιάς": "Peiraiás", "Χανιά": "Chaniá", "Ψαρά": "Psará",
		// αυ, ευ and ηυ before voiced and voiceless sounds, and at the end.
		"Ναύπλιο": "Náfplio", "Εύβοια": "Évvoia", "Γαύδος": "Gávdos", "Ευχή": "Efchí", "Ηύρα": "Ívra",
		"ευχαριστώ": "efcharistó", "Αύριο": "Ávrio",
		// ου, μπ at the ends and within a word, ντ, γκ and γγ.
		"ουρανός": "ouranós", "μπορούμε": "boroúme", "Μπάμπης": "Bámpis", "Νταλάρας": "Ntaláras",
		"Γκρέκο": "Gkréko", "Άγγελος": "Ángelos",
		// A diaeresis parts two vowels.
		"Αϊδίνι": "Aïdíni",
		// Capitals.
		"Θεός ΘΕΟΣ": "Theós THEOS",
		// A made-up line, its question mark Latin's.
		"Πού πας, αγάπη μου;": "Poú pas, agápi mou?",
	} {
		if got := Line(in, "el"); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}
