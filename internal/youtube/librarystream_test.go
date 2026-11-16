package youtube

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLibraryListHandsOverEachPage hands a reader that asked the items
// read so far after each page, before the next page is asked for, each
// list a prefix of the whole.
func TestLibraryListHandsOverEachPage(t *testing.T) {
	// The fake holds the pages after the first until the test lets them go.
	gate := make(chan struct{})
	f := &fakeLibraryList{pages: 4, perPage: 2, card: playlistCardAt, asking: func(page int) {
		if page > 0 {
			<-gate
		}
	}}
	got := make(chan []media.Playlist, 8)
	ctx := media.WithPartial(context.Background(), func(read []media.Playlist) { got <- read })
	type answer struct {
		ps  []media.Playlist
		err error
	}
	done := make(chan answer, 1)
	go func() {
		ps, err := f.client(t).LibraryPlaylists(ctx)
		done <- answer{ps, err}
	}()
	first := receive(t, got)
	if !slices.Equal(playlistIDs(first), playlistsOf(1, 2)) {
		t.Fatalf("the first page came as %v", playlistIDs(first))
	}
	select {
	case a := <-done:
		t.Fatalf("the list came whole before its pages: %d, %v", len(a.ps), a.err)
	default:
	}
	var lists [][]media.Playlist
	for range 3 {
		gate <- struct{}{}
		lists = append(lists, receive(t, got))
	}
	for i, l := range lists {
		if want := playlistsOf(i+2, 2); !slices.Equal(playlistIDs(l), want) {
			t.Errorf("after page %d: %v", i+2, playlistIDs(l))
		}
	}
	a := <-done
	if a.err != nil || !slices.Equal(playlistIDs(a.ps), playlistsOf(4, 2)) {
		t.Errorf("the whole list: %d, %v", len(a.ps), a.err)
	}
	// What was handed over stays as it was, as the list grew.
	if !slices.Equal(playlistIDs(first), playlistsOf(1, 2)) {
		t.Errorf("the first hand-over changed to %v", playlistIDs(first))
	}
}

// receive takes a list a reader was handed.
func receive(t *testing.T, got chan []media.Playlist) []media.Playlist {
	t.Helper()
	select {
	case l := <-got:
		return l
	case <-time.After(20 * time.Second):
		t.Fatal("no list was handed over")
		return nil
	}
}

// TestLibraryListHandsOverNothingUnasked reads a list whole, as new
// releases do, for a reader that didn't ask.
func TestLibraryListHandsOverNothingUnasked(t *testing.T) {
	f := &fakeLibraryList{pages: 3, perPage: 2, card: playlistCardAt}
	got, err := f.client(t).LibraryPlaylists(context.Background())
	if err != nil || !slices.Equal(playlistIDs(got), playlistsOf(3, 2)) {
		t.Fatalf("LibraryPlaylists = %d, %v", len(got), err)
	}
}
