package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// shownEpisode is an episode of a show, released on day.
func shownEpisode(id, day string) media.Track {
	return media.Track{ID: "yt:" + id, Title: id, Episode: &media.Episode{
		Show: media.Ref{ID: "ytpod:show", Name: "A Show"}, Released: day}}
}

func ids(tracks []media.Track) []string {
	out := make([]string, len(tracks))
	for i, t := range tracks {
		out[i] = t.Title
	}
	return out
}

// TestEpisodeQueueGoesForwardInTime shows a show's page, which lists its
// episodes newest first, queueing them from the one chosen forward in
// time, so that the next newer one follows it (#589).
func TestEpisodeQueueGoesForwardInTime(t *testing.T) {
	// Newest first, as a show's page lists them.
	page := []media.Track{
		shownEpisode("ep5", "2026-11-20"),
		shownEpisode("ep4", "2026-11-13"),
		shownEpisode("ep3", "2026-11-06"),
		shownEpisode("ep2", "2026-10-30"),
		shownEpisode("ep1", "2026-10-23"),
	}
	got := ids(episodeQueue(page, 2, func(media.Track) bool { return false }))
	want := []string{"ep3", "ep4", "ep5"}
	if len(got) != len(want) {
		t.Fatalf("the queue is %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the queue is %v, want %v", got, want)
		}
	}
}

// TestEpisodeQueuePlaysTheNewestAlone shows Play, which starts the newest
// episode, queueing it alone: nothing came out after it.
func TestEpisodeQueuePlaysTheNewestAlone(t *testing.T) {
	page := []media.Track{
		shownEpisode("ep5", "2026-11-20"),
		shownEpisode("ep4", "2026-11-13"),
	}
	got := ids(episodeQueue(page, 0, func(media.Track) bool { return false }))
	if len(got) != 1 || got[0] != "ep5" {
		t.Errorf("the queue is %v, want [ep5]", got)
	}
}

// TestEpisodeQueueLeavesOutHeard shows the episodes the user heard left
// out of the queue, but the one they chose, which plays even so.
func TestEpisodeQueueLeavesOutHeard(t *testing.T) {
	page := []media.Track{
		shownEpisode("ep5", "2026-11-20"),
		shownEpisode("ep4", "2026-11-13"),
		shownEpisode("ep3", "2026-11-06"),
		shownEpisode("ep2", "2026-10-30"),
	}
	heard := map[string]bool{"ep3": true, "ep4": true}
	got := ids(episodeQueue(page, 3, func(t media.Track) bool { return heard[t.Title] }))
	want := []string{"ep2", "ep5"}
	if len(got) != len(want) {
		t.Fatalf("the queue is %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the queue is %v, want %v", got, want)
		}
	}
	// The episode chosen plays even though it was heard.
	if got := ids(episodeQueue(page, 2, func(t media.Track) bool { return t.Title == "ep3" })); len(got) == 0 || got[0] != "ep3" {
		t.Errorf("the chosen episode isn't first: %v", got)
	}
}

// TestEpisodeQueueWithoutDays shows a page whose episodes tell no day
// queueing them from the one chosen forward in time all the same: the page
// lists them newest first.
func TestEpisodeQueueWithoutDays(t *testing.T) {
	page := []media.Track{shownEpisode("ep3", ""), shownEpisode("ep2", ""), shownEpisode("ep1", "")}
	got := ids(episodeQueue(page, 2, func(media.Track) bool { return false }))
	want := []string{"ep1", "ep2", "ep3"}
	if len(got) != len(want) {
		t.Fatalf("the queue is %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the queue is %v, want %v", got, want)
		}
	}
}
