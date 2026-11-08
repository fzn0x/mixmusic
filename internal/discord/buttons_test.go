package discord

import (
	"context"
	"encoding/json"
	"testing"
)

// TestSetActivityButtons sends Discord an activity's buttons and its badge
// in SET_ACTIVITY's payload, as Discord's IPC takes them: a list of labels
// and URLs beside the assets, and the small image and its text among them.
func TestSetActivityButtons(t *testing.T) {
	c, f := newFake(t)
	a := Activity{
		Type:       Listening,
		Details:    "Song",
		LargeImage: "https://example.com/cover.jpg",
		SmallImage: "https://example.com/spotify.png",
		SmallText:  "Spotify",
		Buttons:    []Button{{"Listen on Spotify", "https://open.spotify.com/track/4uLU6hMCjMI75M1A2tKUQC"}},
	}
	done := async(func() error { return c.SetActivity(context.Background(), a) })
	p := f.accept(t)
	p.handshake()
	cmd := p.command()
	if err := wait(t, done); err != nil {
		t.Fatal(err)
	}
	want := `{"type":2,"details":"Song",` +
		`"assets":{"large_image":"https://example.com/cover.jpg","small_image":"https://example.com/spotify.png","small_text":"Spotify"},` +
		`"buttons":[{"label":"Listen on Spotify","url":"https://open.spotify.com/track/4uLU6hMCjMI75M1A2tKUQC"}]}`
	if got := string(cmd.Args.Activity); got != want {
		t.Errorf("activity = %s\nwant %s", got, want)
	}
	var sent struct {
		Buttons []struct{ Label, URL string }
	}
	if err := json.Unmarshal(cmd.Args.Activity, &sent); err != nil || len(sent.Buttons) != 1 {
		t.Fatalf("buttons = %+v, %v", sent.Buttons, err)
	}

	// The same activity without its button is another, which Discord is
	// told of.
	a.Buttons = nil
	done = async(func() error { return c.SetActivity(context.Background(), a) })
	cmd = p.command()
	if err := wait(t, done); err != nil {
		t.Fatal(err)
	}
	var again map[string]json.RawMessage
	if err := json.Unmarshal(cmd.Args.Activity, &again); err != nil {
		t.Fatal(err)
	}
	if _, ok := again["buttons"]; ok {
		t.Errorf("the button stayed: %s", cmd.Args.Activity)
	}
}
