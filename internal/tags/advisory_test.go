package tags

import "testing"

// TestAdvisory reads the content advisory of files of each kind: iTunes'
// rtng item of an MP4 file, ITUNESADVISORY or EXPLICIT as an ID3v2 TXXX
// frame, of each version, a Vorbis comment or an MP4 file's own item; and
// a track marked clean, or of no advisory, isn't explicit. The files are
// made up.
func TestAdvisory(t *testing.T) {
	txxx4 := func(desc, value string) []byte {
		return frame4("TXXX", 0, append([]byte{3}, desc+"\x00"+value...))
	}
	title4 := frame4("TIT2", 0, append([]byte{3}, "Salt on the Wind"...))
	mp4 := func(items ...[]byte) []byte {
		ilst := mbox("ilst", append([][]byte{item("\xa9nam", 1, []byte("Salt on the Wind"))}, items...)...)
		meta := mbox("meta", make([]byte, 4), mbox("hdlr", make([]byte, 25)), ilst)
		return cat(mbox("ftyp", []byte("M4A "), be32(0)), mbox("moov", mbox("udta", meta)), mbox("mdat", make([]byte, 100)))
	}
	freeform := func(name, value string) []byte {
		return mbox("----", mbox("mean", make([]byte, 4), []byte("com.apple.iTunes")), mbox("name", make([]byte, 4), []byte(name)),
			mbox("data", be32(1), make([]byte, 4), []byte(value)))
	}
	for _, c := range []struct {
		name string
		file []byte
		want bool
	}{
		{"ID3v2.4 ITUNESADVISORY 1", cat(id3(4, 0, cat(title4, txxx4("ITUNESADVISORY", "1"))), mp3), true},
		{"ID3v2.4 iTunesAdvisory 2, clean", cat(id3(4, 0, cat(title4, txxx4("iTunesAdvisory", "2"))), mp3), false},
		{"ID3v2.4 EXPLICIT yes", cat(id3(4, 0, cat(title4, txxx4("EXPLICIT", "yes"))), mp3), true},
		{"ID3v2.4 another TXXX", cat(id3(4, 0, cat(title4, txxx4("MOOD", "1"))), mp3), false},
		{"ID3v2.4 no advisory", cat(id3(4, 0, title4), mp3), false},
		{"ID3v2.3 ITUNESADVISORY in UTF-16", cat(id3(3, 0, cat(frame3("TIT2", 0, append([]byte{0}, "Salt on the Wind"...)), frame3("TXXX", 0,
			cat([]byte{1}, utf16le("ITUNESADVISORY"), []byte{0, 0}, utf16le("1"))))), mp3), true},
		{"ID3v2.2 TXX", cat(id3(2, 0, cat(frame2("TT2", append([]byte{0}, "Salt on the Wind"...)),
			frame2("TXX", append([]byte{0}, "ITUNESADVISORY\x004"...)))), mp3), true},
		{"FLAC ITUNESADVISORY", flacFile([]byte{4}, vcomments("TITLE=Salt on the Wind", "itunesadvisory=1")), true},
		{"FLAC EXPLICIT", flacFile([]byte{4}, vcomments("TITLE=Salt on the Wind", "EXPLICIT=true")), true},
		{"FLAC clean", flacFile([]byte{4}, vcomments("TITLE=Salt on the Wind", "ITUNESADVISORY=2")), false},
		{"MP4 rtng 1", mp4(item("rtng", 21, []byte{1})), true},
		{"MP4 rtng 4", mp4(item("rtng", 21, []byte{4})), true},
		{"MP4 rtng 2, clean", mp4(item("rtng", 21, []byte{2})), false},
		{"MP4 rtng 0", mp4(item("rtng", 21, []byte{0})), false},
		{"MP4 ITUNESADVISORY item", mp4(freeform("ITUNESADVISORY", "1")), true},
		{"MP4 no advisory", mp4(), false},
	} {
		got := read(t, c.file)
		if got.Explicit != c.want {
			t.Errorf("%s: explicit %v, not %v", c.name, got.Explicit, c.want)
		}
		if got.Title != "Salt on the Wind" && c.want {
			t.Errorf("%s: the title, %q, wasn't read", c.name, got.Title)
		}
	}
}
