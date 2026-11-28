package lyrics

import (
	"reflect"
	"strings"
	"testing"
)

const bloodlustGoatComment = `Lyrics

Jimen ni hizamazuki, namida ni nureru,
Awaremi motome, jibun no kachi wo inoru,
Demo kotae wa shizuka, yabureta sakebi ni,
Aoi sora kara hashigo wa orite konai.
Kizu-

Naita koe ga hibiku mine ni nobori,
Ten kara no sō uta wo sasayaku,
Demo kumo wa tada mujun, haiiro ni nagare,
Kibō wo tooku ni hakonde iku.

Naita koe ga hibiku mine ni nobori,
Ten kara no sō uta wo sasayaku,
Demo kumo wa tada mujun, haiiro ni nagare,
Kibō wo tooku ni hakonde iku.

Yoru ni wa gin no kyōkai wo yume miru,
Kagami no naka de warau koe,
Demo mezameru to sabishii ishi to ame,
Karappo ni hibiku kono mune.

Dakara, yuganda tsubasa de koko ni matsu,
Yami ni shibarareta hikari no yūrei,
Eien ni ten no tobira wa hirakanai,
Ushinawareta tenshi zutto eien ni.

============================

English:

Kneeling on the ground, drenched in tears,
Seeking compassion, praying for my worth,
But the answer is silent, in a broken cry,
From the blue sky, no ladder will descend.
Wounds—

The crying voice echoes, climbing the peak,
Whispering a hymn from the heavens,
But the clouds are only contradictions, flowing in gray,
Carrying hope far away.

The crying voice echoes, climbing the peak,
Whispering a hymn from the heavens,
But the clouds are only contradictions, flowing in gray,
Carrying hope far away.

In the night, I dream of a silver church,
A voice laughing within the mirror,
But when I awaken, there is only lonely stone and rain,
This empty chest reverberates.

So, with twisted wings, I wait here,
A ghost of light bound by darkness,
The gates of heaven will never open,
The lost angel, forever and ever.`

func TestBloodlustBilingualComment(t *testing.T) {
	extracted, sc, ok := ExtractFromComment(bloodlustGoatComment, false, false)
	if !ok || sc < 50 {
		t.Fatalf("ExtractFromComment failed: ok=%v score=%d", ok, sc)
	}
	l := PlainBilingual(extracted, "YouTube video & comments")
	if l.Language != "ja" {
		t.Fatalf("Language = %q, want %q", l.Language, "ja")
	}
	if l.Translation == nil {
		t.Fatalf("expected embedded Translation")
	}
	if l.Translation.From != "ja" || l.Translation.To != "en" {
		t.Fatalf("Translation From=%q To=%q, want ja/en", l.Translation.From, l.Translation.To)
	}
	if len(l.Lines) != 25 || len(l.Translation.Lines) != 25 {
		t.Fatalf("len(Lines)=%d len(Translation.Lines)=%d, want 25/25", len(l.Lines), len(l.Translation.Lines))
	}
	if l.Lines[0].Text != "Jimen ni hizamazuki, namida ni nureru," {
		t.Errorf("line 0 = %q", l.Lines[0].Text)
	}
	if l.Translation.Lines[0].Text != "Kneeling on the ground, drenched in tears," {
		t.Errorf("translation 0 = %q", l.Translation.Lines[0].Text)
	}
	// Stanza blank line preserved with empty translation entry.
	if l.Lines[5].Text != "" || l.Translation.Lines[5].Text != "" {
		t.Errorf("stanza gap line 5 = (%q, %q), want empty", l.Lines[5].Text, l.Translation.Lines[5].Text)
	}
	last := len(l.Lines) - 1
	if l.Lines[last].Text != "Ushinawareta tenshi zutto eien ni." ||
		l.Translation.Lines[last].Text != "The lost angel, forever and ever." {
		t.Errorf("last line = (%q, %q)", l.Lines[last].Text, l.Translation.Lines[last].Text)
	}
}

func TestBilingualThreeBlocksKanjiRomajiEnglish(t *testing.T) {
	raw := `[Japanese]
地面に跪き
涙に濡れる
憐れみ求め
祈るこの声

[Romaji]
Jimen ni hizamazuki
Namida ni nureru
Awaremi motome
Inoru kono koe

[English]
Kneeling on the ground
Drenched in tears
Seeking compassion
This praying voice`

	l := PlainBilingual(raw, "YouTube")
	if l.Language != "ja" || l.Translation == nil {
		t.Fatalf("got lang=%q tr=%v", l.Language, l.Translation)
	}
	if l.Translation.To != "en" || len(l.Lines) != 4 {
		t.Fatalf("unexpected translation meta or line count: %+v", l)
	}
	if l.Lines[0].Text != "地面に跪き" || l.Translation.Lines[0].Romanized != "Jimen ni hizamazuki" || l.Translation.Lines[0].Text != "Kneeling on the ground" {
		t.Fatalf("row 0 = (%q, roman=%q, trans=%q)", l.Lines[0].Text, l.Translation.Lines[0].Romanized, l.Translation.Lines[0].Text)
	}
}

func TestBilingualInterleavedPairsAndTriples(t *testing.T) {
	pairs := `地面に跪き
Kneeling on the ground

涙に濡れる
Drenched in tears

青い空から
From the blue sky

祈るこの声
This praying voice`

	l := PlainBilingual(pairs, "Comment")
	if l.Language != "ja" || l.Translation == nil || len(l.Lines) != 4 {
		t.Fatalf("interleaved pairs failed: %+v", l)
	}
	if l.Lines[2].Text != "青い空から" || l.Translation.Lines[2].Text != "From the blue sky" {
		t.Errorf("pair 2 = (%q, %q)", l.Lines[2].Text, l.Translation.Lines[2].Text)
	}

	parenInterleaved := `Jimen ni hizamazuki
(Kneeling on the ground)
Namida ni nureru
(Drenched in tears)
Aoi sora kara
(From the blue sky)
Ushinawareta tenshi
(The lost angel forever)`
	lp := PlainBilingual(parenInterleaved, "Comment")
	if lp.Translation == nil || len(lp.Lines) != 4 {
		t.Fatalf("parenthesized interleaved failed: %+v", lp)
	}
	if lp.Translation.Lines[1].Text != "Drenched in tears" {
		t.Errorf("expected unwrapped translation, got %q", lp.Translation.Lines[1].Text)
	}

	triples := `地面に跪き
Jimen ni hizamazuki
Kneeling on the ground

涙に濡れる
Namida ni nureru
Drenched in tears

青い空から
Aoi sora kara
From the blue sky

永遠の翼
Eien no tsubasa
Wings of eternity`
	lt := PlainBilingual(triples, "Comment")
	if lt.Translation == nil || len(lt.Lines) != 4 {
		t.Fatalf("interleaved triples failed: %+v", lt)
	}
	if lt.Lines[3].Text != "永遠の翼" || lt.Translation.Lines[3].Romanized != "Eien no tsubasa" || lt.Translation.Lines[3].Text != "Wings of eternity" {
		t.Errorf("triple 3 = %+v, %+v", lt.Lines[3], lt.Translation.Lines[3])
	}
}

func TestBilingualLRCRoundTripAndNegativeCases(t *testing.T) {
	orig := Lyrics{
		Synced:   true,
		Source:   "YouTube video & comments",
		Language: "ja",
		Lines: []Line{
			{TimeMs: 1000, Text: "地面に跪き", Words: []Word{{Text: "地面に", StartMs: 1000, EndMs: 1500}, {Text: "跪き", StartMs: 1500, EndMs: 1500}}},
			{TimeMs: 2500, Text: ""},
			{TimeMs: 3000, Text: "涙に濡れる"},
		},
		Translation: &Translation{
			From: "ja",
			To:   "en",
			Lines: []TranslatedLine{
				{Text: "Kneeling on the ground", Romanized: "Jimen ni hizamazuki"},
				{},
				{Text: "Drenched in tears", Romanized: "Namida ni nureru"},
			},
		},
	}
	lrc := orig.LRC()
	if !strings.Contains(lrc, "--- Romanized ---") || !strings.Contains(lrc, "--- Translation (en) ---") {
		t.Fatalf("expected marked blocks in LRC:\n%s", lrc)
	}
	round, ok := FromText(lrc, orig.Source)
	if !ok || !reflect.DeepEqual(round, orig) {
		t.Fatalf("LRC roundtrip mismatch:\ngot:  %+v\nwant: %+v", round, orig)
	}

	// Negative case 1: normal monolingual lyrics with blank lines and backing vocal adlibs.
	mono := `Can someone please just turn the lights down
(yeah)
I'm just tryna see you not through the phone
(oh yeah)

I got your picture in my nikon
(ooh)
Flash hit my face until my life's gone
(x2)`
	lm := PlainBilingual(mono, "LRCLIB")
	if lm.Translation != nil {
		t.Errorf("expected monolingual lyrics not to produce translation, got %+v", lm.Translation)
	}

	// Negative case 2: lyrics + short mismatched commenter note after a divider.
	commentWithNote := `Jimen ni hizamazuki, namida ni nureru,
Awaremi motome, jibun no kachi wo inoru,
Demo kotae wa shizuka, yabureta sakebi ni,
Aoi sora kara hashigo wa orite konai.
----------------------------
Thanks for listening everyone!
Hope this helps you sing along.`
	lNote := PlainBilingual(commentWithNote, "Comment")
	if lNote.Translation != nil {
		t.Errorf("mismatched short footer after divider should not pair as translation: %+v", lNote.Translation)
	}
}
