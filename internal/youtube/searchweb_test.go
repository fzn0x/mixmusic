package youtube

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// webSearchFixture is the shape of the website's search answer, cut down to
// what parseWebSearch reads: two sections, the first with two videos and a
// channel, the second with one video.
const webSearchFixture = `{
  "contents": {
    "twoColumnSearchResultsRenderer": {
      "primaryContents": {
        "sectionListRenderer": {
          "contents": [
            {
              "itemSectionRenderer": {
                "contents": [
                  {
                    "videoRenderer": {
                      "videoId": "rFZHOHl-L8A",
                      "title": {"runs": [{"text": "lofi hip hop radio"}]},
                      "ownerText": {"runs": [{
                        "text": "Lofi Girl",
                        "navigationEndpoint": {"browseEndpoint": {"browseId": "UCSJ4gkVC6NrvII8umztf0Ow"}}
                      }]},
                      "lengthText": {"simpleText": "6:10:58"},
                      "viewCountText": {"simpleText": "1,234,567 views"},
                      "thumbnail": {"thumbnails": [
                        {"url": "https://i.ytimg.com/vi/rFZHOHl-L8A/hqdefault.jpg?sqp=x", "width": 480, "height": 360}
                      ]}
                    }
                  },
                  {
                    "videoRenderer": {
                      "videoId": "n61ULEU7CO0",
                      "title": {"runs": [{"text": "Best of lofi hip hop"}]},
                      "ownerText": {"runs": [{"text": "Lofi Girl"}]},
                      "lengthText": {"simpleText": "3:45"},
                      "viewCountText": {"simpleText": "905M views"}
                    }
                  },
                  {"channelRenderer": {"channelId": "UCxxxx", "title": {"simpleText": "A channel"}}}
                ]
              }
            },
            {
              "itemSectionRenderer": {
                "contents": [
                  {
                    "videoRenderer": {
                      "videoId": "uWzSqdcfPZw",
                      "title": {"runs": [{"text": "lofi hip hop radio – beats to sleep"}]},
                      "ownerText": {"runs": [{"text": "STEEZYASFUCK"}]}
                    }
                  }
                ]
              }
            }
          ]
        }
      }
    }
  }
}`

// TestParseWebSearch reads the videos of the website's answer, leaving out
// what isn't a video, and keeps each one's channel, length and views.
func TestParseWebSearch(t *testing.T) {
	var v any
	if err := json.Unmarshal([]byte(webSearchFixture), &v); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	tracks := parseWebSearch(node{v})
	if len(tracks) != 3 {
		t.Fatalf("got %d videos, want 3", len(tracks))
	}
	first := tracks[0]
	if first.ID != "yt:rFZHOHl-L8A" {
		t.Errorf("id = %q", first.ID)
	}
	if first.Title != "lofi hip hop radio" {
		t.Errorf("title = %q", first.Title)
	}
	if len(first.Artists) != 1 || first.Artists[0].Name != "Lofi Girl" {
		t.Errorf("artists = %+v", first.Artists)
	}
	if first.Artists[0].ID != "yt:UCSJ4gkVC6NrvII8umztf0Ow" {
		t.Errorf("channel id = %q", first.Artists[0].ID)
	}
	if first.DurationMs != ((6*60+10)*60+58)*1000 {
		t.Errorf("duration = %d", first.DurationMs)
	}
	if first.Plays != 1234567 {
		t.Errorf("plays = %d", first.Plays)
	}
	if !first.Video {
		t.Error("video flag not set")
	}
	if first.URL != "https://www.youtube.com/watch?v=rFZHOHl-L8A" {
		t.Errorf("url = %q", first.URL)
	}
	if len(first.Artwork) == 0 {
		t.Error("no artwork")
	}
	// A video without a channel keeps its name and no ID.
	if len(tracks[1].Artists) != 1 || tracks[1].Artists[0].ID != "" {
		t.Errorf("second video's artists = %+v", tracks[1].Artists)
	}
	if tracks[1].Plays != 905000000 {
		t.Errorf("second video's plays = %d", tracks[1].Plays)
	}
	// A video without a length or views keeps zeroes.
	if tracks[2].DurationMs != 0 || tracks[2].Plays != 0 {
		t.Errorf("third video = %d ms, %d plays", tracks[2].DurationMs, tracks[2].Plays)
	}
}

// TestLiveSearchWeb searches YouTube itself, which changes its answers from
// time to time. It runs with MIXMUSIC_LIVE=1:
//
//	MIXMUSIC_LIVE=1 go test ./internal/youtube -run LiveSearchWeb -v
func TestLiveSearchWeb(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to search YouTube")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	res, err := New().SearchWeb(ctx, "lofi hip hop")
	if err != nil {
		t.Fatalf("SearchWeb: %v", err)
	}
	tracks := res.Videos
	if len(tracks) == 0 {
		t.Fatal("no videos")
	}
	for _, tr := range tracks {
		if tr.ID == "" || tr.Title == "" {
			t.Errorf("incomplete video: %+v", tr)
		}
		if tr.Source != "youtube" {
			t.Errorf("source = %q", tr.Source)
		}
	}
	t.Logf("%d videos, first: %s — %s", len(tracks), tracks[0].Title, tracks[0].URL)
}
