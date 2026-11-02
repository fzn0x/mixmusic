package match

import (
	"context"
	"errors"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// by is a YouTube Music track of a title by artists, of length seconds.
func by(title string, seconds int, video bool, artists ...string) media.Track {
	t := media.Track{ID: "yt:" + title + "/" + artists[0], Source: media.SourceYouTube, Title: title, DurationMs: seconds * 1000, Video: video}
	for _, a := range artists {
		t.Artists = append(t.Artists, media.Ref{Name: a})
	}
	return t
}

// TestSameSong is music videos of radios, beside the songs a search of
// YouTube Music found for them: their songs, and the covers, karaoke
// versions and other songs it found too.
func TestSameSong(t *testing.T) {
	for _, c := range []struct {
		video, song media.Track
		same        bool
	}{
		{by("I Feel It Coming ft. Daft Punk (Official Video)", 298, true, "The Weeknd"), by("I Feel It Coming (feat. Daft Punk)", 269, false, "The Weeknd", "Daft Punk"), true},
		{by("We Don't Talk Anymore (feat. Selena Gomez)", 230, true, "Charlie Puth"), by("We Don't Talk Anymore (feat. Selena Gomez)", 218, false, "Charlie Puth"), true},
		{by("We Don't Talk Anymore (feat. Selena Gomez)", 230, true, "Charlie Puth"), by("We Don't Talk Anymore (Originally Performed By Charlie Puth and Selena Gomez) [Karaoke Version]", 218, false, "Karaoke Hits"), false},
		{by("アイドル - Idol", 213, true, "YOASOBI"), by("アイドル", 213, false, "YOASOBI"), true},
		{by("アイドル - Idol", 213, true, "YOASOBI"), by("アイドル (アニメ「推しの子」より) [オリジナル歌手：YOASOBI] [Cover]", 214, false, "A Cover Singer"), false},
		{by("Zankyosanka", 185, true, "Aimer"), by("残響散歌 - Zankyosanka", 185, false, "Aimer"), true},
		{by("Smart (Slowed + Reverb ver.)", 160, true, "LE SSERAFIM"), by("EASY (Slowed + Reverb ver.)", 190, false, "LE SSERAFIM"), false},
		// A fan's lyric video names the artist in its title.
		{by("Katy Perry - The One That Got Away | Lirik Terjemahan", 230, true, "A Lyrics Channel"), by("The One That Got Away", 227, false, "Katy Perry"), true},
		{by("Side To Side (Official Video) (feat. Nicki Minaj)", 240, true, "Ariana Grande ft. Nicki Minaj"), by("Side To Side (feat. Nicki Minaj)", 226, false, "Ariana Grande"), true},
		{by("The Voidz - All The Same (Official Audio w/ Lyrics)", 250, true, "The Voidz"), by("All the Same", 245, false, "The Voidz"), true},
		// A tribute is "by" the song it covers.
		{by("Counting Stars", 284, true, "OneRepublic"), by("Counting Stars (Tribute to OneRepublic) (Piano Version)", 250, false, "Counting Stars", "Piano Tributes"), false},
		{by("Song", 200, true, "Ann"), by("Song", 200, false, ""), false},
	} {
		if got := SameSong(c.video, c.song); got != c.same {
			t.Errorf("SameSong(%q by %s, %q by %s) = %v", c.video.Title, c.video.ArtistNames(), c.song.Title, c.song.ArtistNames(), got)
		}
	}
}

// songSearch is YouTube Music's songs, as a search finds them.
type songSearch struct {
	songs  []media.Item
	err    error
	query  string
	filter youtube.Filter
}

func (s *songSearch) Search(_ context.Context, query string, filter youtube.Filter) ([]media.Item, error) {
	s.query, s.filter = query, filter
	return s.songs, s.err
}

func TestSongOfAVideo(t *testing.T) {
	for _, c := range []struct {
		name  string
		video media.Track
		songs []media.Track
		want  string
	}{
		{
			"the very title, over a longer one and a karaoke version",
			by("Time (Official Video)", 250, true, "Ann"),
			[]media.Track{by("Time (Karaoke Version)", 250, false, "Sing Along"), by("Time After Time", 250, false, "Ann"), by("Time", 230, false, "Ann")},
			"Time",
		},
		{
			// The remix runs closest to the video, and the 7" mix is first.
			"the fewest extra words, then YouTube Music's order, whatever their lengths",
			by("What Is Love", 241, true, "Haddaway"),
			[]media.Track{by(`What Is Love (7" Mix)`, 271, false, "Haddaway"), by(`What Is Love (12" Mix)`, 403, false, "Haddaway"), by(`What Is Love (Mosquito Headz - Millennium 7" Remix)`, 229, false, "Haddaway")},
			`What Is Love (7" Mix)`,
		},
		{
			"not a video, though its title is the very one",
			by("Blue (Official Video)", 200, true, "Bo"),
			[]media.Track{by("Blue", 200, true, "Bo"), by("Blue (Live)", 260, false, "Bo")},
			"Blue (Live)",
		},
	} {
		s := &songSearch{songs: items(c.songs...)}
		got, err := New(s, nil).Song(context.Background(), c.video)
		if err != nil || got.Title != c.want {
			t.Errorf("%s: Song = %q, %v; want %q", c.name, got.Title, err, c.want)
		}
		if s.filter != youtube.FilterSongs {
			t.Errorf("%s: searched %s", c.name, s.filter)
		}
	}
}

func TestSongOfAVideoSearches(t *testing.T) {
	s := &songSearch{}
	New(s, nil).Song(context.Background(), by("Paradise (Official Video)", 260, true, "Coldplay"))
	if s.query != "Paradise Coldplay" {
		t.Errorf("searched %q", s.query)
	}
}

func TestNoSongOfAVideo(t *testing.T) {
	video := by("Time (Official Video)", 250, true, "Ann")
	// An upload with no song: a cover, a live take.
	s := &songSearch{songs: items(by("Time", 250, false, "Someone Else"))}
	if _, err := New(s, nil).Song(context.Background(), video); !errors.Is(err, ErrNoMatch) {
		t.Errorf("of another artist's song: err = %v", err)
	}
	s.songs, s.err = nil, errors.New("youtube music: HTTP 500")
	if _, err := New(s, nil).Song(context.Background(), video); err != s.err {
		t.Errorf("YouTube Music down: err = %v", err)
	}
}
