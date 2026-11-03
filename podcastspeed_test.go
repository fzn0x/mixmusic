package main

import "testing"

// TestPodcastSpeedSettings sets the speed of podcasts in its row under the
// songs', apart from theirs, and puts back their own with normal.
func TestPodcastSpeedSettings(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 3200)
	show(s, tt, "/settings/playback")
	songs, ok := tt.Find("Speed of songs")
	if !ok {
		t.Fatalf("no slider of the songs' speed: %q", tt.Texts())
	}
	r, ok := tt.Find("Speed of podcasts")
	if !ok {
		t.Fatalf("no slider of the podcasts' speed: %q", tt.Texts())
	}
	if r.Y <= songs.Y || !tt.HasText("Podcasts") {
		t.Errorf("the podcasts' row isn't under the songs': %v, %v", songs, r)
	}
	if !tt.HasText("Tracks and episodes Spotify plays, in MixMusic or on Spotify Connect, keep their own speed.") {
		t.Errorf("the settings don't tell that Spotify's episodes keep their speed: %q", tt.Texts())
	}
	tt.ClickAt(r.X+r.W-1, r.Y+r.H/2)
	s.drain()
	tt.Frame()
	if got := s.svc.Preferences.Get(); got.PodcastSpeed != 3 || got.SongSpeed != 1 {
		t.Errorf("the podcasts' slider keeps podcasts at %v and songs at %v", got.PodcastSpeed, got.SongSpeed)
	}
	if !tt.HasText("3×, at their own pitch.") {
		t.Errorf("at 3×, the row doesn't say so: %q", tt.Texts())
	}
	click(t, s, tt, "normal")
	if got := s.svc.Preferences.Get().PodcastSpeed; got != 1 {
		t.Errorf("normal keeps the podcasts' speed %v", got)
	}
}

// TestPodcastSpeedFound finds the speed of podcasts by the words of a
// speed, and of podcasts.
func TestPodcastSpeedFound(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 3200)
	for _, q := range []string{"podcast speed", "episodes", "spoken word", "faster", "playback rate"} {
		show(s, tt, settingsPath("playback", q))
		if _, ok := tt.Find("Speed of podcasts"); !ok {
			t.Errorf("searching %q doesn't find the speed of podcasts: %q", q, tt.Texts())
		}
	}
}
