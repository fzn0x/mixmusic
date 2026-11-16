package lastfm

import (
	"context"
	"net/url"
	"testing"
)

func TestLove(t *testing.T) {
	var got []url.Values
	c := testServer(t, func(form url.Values) (int, string) {
		got = append(got, form)
		return 200, `{}`
	})
	ctx := context.Background()
	song := Track{Artist: "Ann", Title: "Song", Album: "Album"}
	if err := c.Love(ctx, "SK", song, true); err != nil {
		t.Fatal(err)
	}
	if err := c.Love(ctx, "SK", song, false); err != nil {
		t.Fatal(err)
	}
	for i, method := range []string{"track.love", "track.unlove"} {
		f := got[i]
		if f.Get("method") != method || f.Get("sk") != "SK" || f.Get("artist") != "Ann" || f.Get("track") != "Song" || f.Has("album") {
			t.Errorf("call %d: %v", i, f)
		}
	}
}
