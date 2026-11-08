package app

import (
	"context"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// toldServices prepare each YouTube track as its stream, which tells of no
// 5.1 audio, at the address the app gives it, without the network.
type toldServices struct{ endingServices }

func (toldServices) Prepare(_ context.Context, t media.Track) (player.Source, error) {
	_, videoID, err := media.ParseID(t.ID)
	if err != nil {
		return player.Source{}, err
	}
	return player.Source{Mode: player.ModeStream, URL: streamURL(videoID), Track: t}, nil
}

func (s toldServices) PrepareStream(ctx context.Context, t media.Track) (player.Source, error) {
	return s.Prepare(ctx, t)
}

// TestSurroundToldReachesThePlayer has the track that plays offer the 5.1
// audio YouTube's TV app told of after its stream was found, as the
// resolver tells the app: its 5.1 button shows then.
func TestSurroundToldReachesThePlayer(t *testing.T) {
	a := newQuietApp(t, nil)
	if a.resolver.Told == nil {
		t.Fatal("the resolver tells the app nothing of 5.1 audio told late")
	}
	stream := &quietStream{}
	svc := toldServices{endingServices{offlineServices{playerServices{a}}, &plays{}}}
	p := player.New(svc, stream, connectDevice{a}, player.Saved{})
	stream.p.Store(p)
	a.goPlayer.Store(p)
	t.Cleanup(func() {
		p.Close()
		p.Flush()
		svc.plays.end()
	})
	const videoID = "aaaaaaaaaaa"
	p.PlayTracks([]media.Track{ytTrack(videoID)}, 0)
	eventually(t, "the track playing", func() bool {
		st := p.State()
		return st.Playing && st.Source != nil && st.Source.URL == streamURL(videoID)
	})
	if st := p.State(); st.Source.SurroundURL != "" {
		t.Fatalf("5.1 audio before it was told of: %q", st.Source.SurroundURL)
	}
	a.resolver.Told(videoID)
	eventually(t, "the 5.1 audio offered", func() bool {
		st := p.State()
		return st.Source != nil && st.Source.SurroundURL == surroundStreamURL(videoID)
	})
}
