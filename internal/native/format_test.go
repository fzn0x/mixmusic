package native

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

func TestFormat(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                                     "0:00",
		-time.Second:                          "0:00",
		208 * time.Second:                     "3:28",
		3723 * time.Second:                    "1:02:03",
		59*time.Second + 999*time.Millisecond: "0:59",
	} {
		if got := clock(d); got != want {
			t.Errorf("clock(%v) = %q, want %q", d, got, want)
		}
	}
	if got := longDuration(42 * time.Minute); got != "42 min" {
		t.Errorf("longDuration: %q", got)
	}
	if got := longDuration(72*time.Minute + 20*time.Second); got != "1 hr 12 min" {
		t.Errorf("longDuration: %q", got)
	}
	if got := plural(1, "track"); got != "1 track" {
		t.Errorf("plural: %q", got)
	}
	if got := plural(12345, "track"); got != "12,345 tracks" {
		t.Errorf("plural: %q", got)
	}
	if got := plural(2, "match", "matches"); got != "2 matches" {
		t.Errorf("plural: %q", got)
	}
	if got := byteSize(512 << 20); got != "512 MB" {
		t.Errorf("byteSize: %q", got)
	}
	if got := byteSize(1536); got != "1.5 KB" {
		t.Errorf("byteSize: %q", got)
	}
	images := []media.Image{{URL: "big", Width: 1200}, {URL: "small", Width: 60}, {URL: "mid", Width: 544}}
	if got := artworkURL(images, 300); got != "mid" {
		t.Errorf("artwork for 300: %q", got)
	}
	if got := artworkURL(images, 2000); got != "big" {
		t.Errorf("artwork for 2000: %q", got)
	}
	if got := names([]media.Ref{{Name: "Ann"}, {Name: "Bo"}}); got != "Ann, Bo" {
		t.Errorf("names: %q", got)
	}
}

// TestPlaysLabel writes counts as YouTube Music does, rounded down to three
// figures at most, plays for a song, views for a video.
func TestPlaysLabel(t *testing.T) {
	for n, want := range map[int64]string{
		148: "148", 1000: "1K", 1299: "1.2K", 2200: "2.2K", 2300: "2.3K", 21000: "21K", 757_000: "757K", 999_999: "999K",
		1_000_000: "1M", 13_000_000: "13M", 905_000_000: "905M", 1_200_000_000: "1.2B", 1_900_000_000: "1.9B",
	} {
		if got := compactCount(n); got != want {
			t.Errorf("compactCount(%d) = %q, want %q", n, got, want)
		}
	}
	for _, c := range []struct {
		t    media.Track
		want string
	}{
		{media.Track{Plays: 1_200_000_000}, "1.2B plays"},
		{media.Track{Plays: 905_000_000, Video: true}, "905M views"},
		{media.Track{Plays: 1}, "1 play"},
		{media.Track{}, ""},
	} {
		if got := playsLabel(c.t); got != c.want {
			t.Errorf("playsLabel(%+v) = %q, want %q", c.t, got, c.want)
		}
	}
}
