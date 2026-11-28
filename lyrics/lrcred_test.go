package lyrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const sampleLrcRedTTML = `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:lrc="http://lrc.red/lyric-ttml-internal" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" lrc:timing="Word" xml:lang="en-US">
<body>
<div>
<p begin="33.522" end="35.789"><span begin="33.522" end="33.838">Look</span> <span begin="33.838" end="34.158">at</span> <span begin="34.158" end="34.457">the</span> <span begin="34.457" end="35.789">stars</span></p>
<p begin="36.261" end="39.909"><span begin="36.261" end="36.577">Look</span> <span begin="36.577" end="36.925">how</span> <span begin="36.925" end="37.242">they</span> <span begin="37.242" end="37.959">shine</span></p>
</div>
</body>
</tt>`

func TestLrcRedISRCAndSearchAndHoldoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search.json":
			w.Write([]byte(`{"hits":[
				{"isrc":"GBAYE0000267","title":"Yellow","artist":"Coldplay","album":"Parachutes","duration":267},
				{"isrc":"GBAYE0000268","title":"Yellow (Live)","artist":"Coldplay","album":"Live","duration":270},
				{"isrc":"GBAYE0000269","title":"Yellow (Acoustic)","artist":"Coldplay","album":"Acoustic","duration":260},
				{"isrc":"GBAYE0000270","title":"Yellow (Demo)","artist":"Coldplay","album":"Demo","duration":180}
			]}`))
		case "/s/GBAYE0000267.ttml", "/s/GBAYE0000268.ttml", "/s/GBAYE0000269.ttml", "/s/GBAYE0000270.ttml":
			w.Write([]byte(sampleLrcRedTTML))
		case "/s/US0000000429.ttml":
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewLrcRed("MixMusicTest")
	c.BaseURL = srv.URL

	if c.Asks(Query{}) {
		t.Error("expected Asks=false on empty query")
	}
	if c.Asks(Query{ISRC: "not-valid!"}) {
		t.Error("expected Asks=false for invalid ISRC without title")
	}
	if !c.Asks(Query{Title: "Yellow", Artist: "Coldplay"}) {
		t.Error("expected Asks=true for title/artist query")
	}

	// 1. Direct ISRC lookup (normalized dashes/case).
	l, err := c.Get(context.Background(), Query{ISRC: "gb-aye-00-00267"})
	if err != nil {
		t.Fatalf("Get(ISRC): %v", err)
	}
	if !l.WordTimed() || l.Source != "lrc.red" || l.SourceURL != LrcRedSite+"/s/GBAYE0000267" || l.Language != "en" {
		t.Fatalf("unexpected lyrics metadata: %+v", l)
	}
	if len(l.Lines) != 2 || l.Lines[0].Text != "Look at the stars" || len(l.Lines[0].Words) != 4 {
		t.Fatalf("unexpected lines: %+v", l.Lines)
	}

	// 2. Title/artist search fallback without ISRC.
	l2, err := c.Get(context.Background(), Query{Title: "Yellow", Artist: "Coldplay", Duration: 267 * time.Second})
	if err != nil || !l2.WordTimed() || l2.SourceURL != LrcRedSite+"/s/GBAYE0000267" {
		t.Fatalf("Get(search fallback) = %+v, %v", l2, err)
	}

	// 3. Rejects wrong artist in search fallback.
	if _, err := c.Get(context.Background(), Query{Title: "Yellow", Artist: "Wrong Artist"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("wrong artist err = %v, want ErrNotFound", err)
	}

	// 4. Multi-candidate Search preloads top 3 and leaves #4 lazy.
	cands, err := c.Search(context.Background(), Query{Title: "Yellow", Artist: "Coldplay", Duration: 267 * time.Second})
	if err != nil || len(cands) != 4 || !cands[0].Loaded || cands[3].Loaded || cands[3].ID != "GBAYE0000270" {
		t.Fatalf("Search = %+v, %v", cands, err)
	}

	// 5. Missing ISRC without title -> ErrNotFound.
	if _, err := c.Get(context.Background(), Query{ISRC: "US0000000404"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("404 err = %v, want ErrNotFound", err)
	}

	// 6. HTTP 429 triggers holdoff.
	if _, err := c.Get(context.Background(), Query{ISRC: "US0000000429"}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("429 err = %v, want ErrUnavailable", err)
	}
	if _, err := c.Get(context.Background(), Query{ISRC: "GBAYE0000267"}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("during holdoff err = %v, want ErrUnavailable", err)
	}
}
