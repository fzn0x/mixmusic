package spotify

import (
	"context"
	"reflect"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestEpisodeDetails reads an episode's details: its show, day and length,
// and its plain description, made up, whose addresses are links and whose
// times seek; without it, the HTML description as plain text.
func TestEpisodeDetails(t *testing.T) {
	c, api := newFake(t, map[string]string{
		"GET /episodes/e1": `{"id": "e1", "name": "Monday", "type": "episode", "duration_ms": 1830000, "release_date": "2026-11-02",
			"description": "Owls at dawn. More at https://example.com/owls.\n00:00 Hello\n12:34 The barn",
			"html_description": "<p>Owls at dawn.</p>",
			"show": {"id": "s1", "name": "The Daily Thing"}}`,
		"GET /episodes/e2": `{"id": "e2", "name": "Tuesday", "type": "episode", "duration_ms": 60000, "release_date": "2026-11",
			"description": "", "html_description": "<p>Bats &amp; owls</p><p>At <a href=\"https://example.com\">our site</a></p>"}`,
	})
	d, err := c.EpisodeDetails(context.Background(), "e1")
	if err != nil {
		t.Fatal(err)
	}
	if api.requests[0] != "GET /episodes/e1?" {
		t.Errorf("asked %s", api.requests)
	}
	if d.Show != (media.Ref{ID: "sp:s1", Name: "The Daily Thing"}) || d.Released != "2026-11-02" || d.DurationMs != 1830000 || d.Plays != 0 {
		t.Errorf("details = %+v", d)
	}
	want := [][]media.TextRun{
		{{Text: "Owls at dawn. More at "}, {Text: "https://example.com/owls", Link: "https://example.com/owls"}, {Text: "."}},
		{{Text: "00:00", Seek: true}, {Text: " Hello"}},
		{{Text: "12:34", Seek: true, AtMs: 754000}, {Text: " The barn"}},
	}
	if !reflect.DeepEqual(d.Description, want) {
		t.Errorf("description = %+v", d.Description)
	}
	d, err = c.EpisodeDetails(context.Background(), "e2")
	want = [][]media.TextRun{{{Text: "Bats & owls"}}, {{Text: "At our site"}}}
	if err != nil || !reflect.DeepEqual(d.Description, want) || d.Show != (media.Ref{}) {
		t.Errorf("details of HTML = %+v, %v", d, err)
	}
}
