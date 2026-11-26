package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type gqlReq struct {
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
	Extensions    struct {
		PersistedQuery struct {
			SHA256Hash string `json:"sha256Hash"`
		} `json:"persistedQuery"`
	} `json:"extensions"`
}

func newWebFake(t *testing.T, handle func(op string, req gqlReq, w http.ResponseWriter, r *http.Request)) (*Client, *httptest.Server) {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nuances.json":
			w.Write([]byte(`[{"s":"GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ","v":61}]`))
		case "/server-time":
			w.Write([]byte(`{"serverTime":1795664820}`))
		case "/api/token":
			exp := time.Now().Add(time.Hour).UnixMilli()
			w.Write([]byte(`{"accessToken":"WEB-REFRESHED","accessTokenExpirationTimestampMs":` + strconvFormatInt(exp) + `,"isAnonymous":false}`))
		case "/pathfinder/v2/query":
			var req gqlReq
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			handle(req.OperationName, req, w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	auth := NewAuth("")
	auth.WebTokenURL = srv.URL + "/api/token"
	auth.ServerTimeURL = srv.URL + "/server-time"
	auth.NuancesURL = srv.URL + "/nuances.json"
	auth.NuancesFallbackURL = ""

	c := NewClient(auth)
	c.GQLURL = srv.URL + "/pathfinder/v2/query"
	c.HashesURL = ""
	c.SetToken(Token{
		AccessToken: "WEB-ACCESS",
		Expiry:      time.Now().Add(time.Hour),
		SPDC:        "spdc-token-value",
		SPT:         "spt-value",
	})
	return c, srv
}

func TestGQLCatalogSearchPlaylistHomeAlbumArtistTrack(t *testing.T) {
	var mu sync.Mutex
	var ops []string
	c, _ := newWebFake(t, func(op string, req gqlReq, w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ops = append(ops, op)
		mu.Unlock()
		switch op {
		case "profileAttributes":
			w.Write([]byte(`{"data":{"me":{"profile":{"uri":"spotify:user:u1","name":"Alice","avatar":{"sources":[{"url":"https://i/avatar","width":300,"height":300}]}}}}}`))
		case "searchDesktop":
			w.Write([]byte(`{"data":{"searchV2":{
				"tracksV2":{"items":[{"item":{"__typename":"TrackResponseWrapper","data":{"__typename":"Track","id":"t1","uri":"spotify:track:t1","name":"Get Lucky","duration":{"totalMilliseconds":248000},"contentRating":{"label":"NONE"},"artists":{"items":[{"uri":"spotify:artist:a1","profile":{"name":"Daft Punk"}}]},"albumOfTrack":{"uri":"spotify:album:al1","name":"RAM","coverArt":{"sources":[{"url":"https://i/al1","width":640,"height":640}]}}}}}]},
				"albumsV2":{"items":[{"__typename":"AlbumResponseWrapper","data":{"__typename":"Album","uri":"spotify:album:al1","name":"RAM","type":"ALBUM","date":{"year":2013},"artists":{"items":[{"uri":"spotify:artist:a1","profile":{"name":"Daft Punk"}}]},"coverArt":{"sources":[{"url":"https://i/al1","width":640,"height":640}]}}}]},
				"artists":{"items":[{"__typename":"ArtistResponseWrapper","data":{"__typename":"Artist","uri":"spotify:artist:a1","profile":{"name":"Daft Punk"},"visuals":{"avatarImage":{"sources":[{"url":"https://i/a1","width":640,"height":640}]}}}}]},
				"playlists":{"items":[{"__typename":"PlaylistResponseWrapper","data":{"__typename":"Playlist","uri":"spotify:playlist:editorial1","name":"Today's Top Hits","description":"Hits","ownerV2":{"data":{"__typename":"User","uri":"spotify:user:spotify","name":"Spotify"}},"images":{"items":[{"sources":[{"url":"https://i/p1"}]}]}}}]},
				"podcasts":{"items":[{"__typename":"PodcastResponseWrapper","data":{"__typename":"Podcast","uri":"spotify:show:sh1","name":"Dissect","publisher":{"name":"Spotify Studios"},"coverArt":{"sources":[{"url":"https://i/sh1","width":300,"height":300}]}}}]},
				"episodes":{"items":[{"__typename":"EpisodeResponseWrapper","data":{"__typename":"Episode","uri":"spotify:episode:ep1","name":"E1","description":"First","duration":{"totalMilliseconds":1800000},"releaseDate":{"isoString":"2026-03-17T00:00:00Z"},"podcastV2":{"data":{"uri":"spotify:show:sh1","name":"Dissect"}},"coverArt":{"sources":[{"url":"https://i/ep1","width":300,"height":300}]}}}]}
			}}}`))
		case "home":
			if req.Variables["sp_t"] != "spt-value" {
				t.Errorf("home sp_t = %v, want spt-value", req.Variables["sp_t"])
			}
			w.Write([]byte(`{"data":{"home":{"sectionContainer":{"sections":{"items":[
				{"uri":"spotify:section:made-for-you","data":{"__typename":"HomeGenericSectionData","title":{"transformedLabel":"Made For You"}},"sectionItems":{"items":[
					{"content":{"__typename":"PlaylistResponseWrapper","data":{"__typename":"Playlist","uri":"spotify:playlist:37i9dQZF1DXcBWIGoYBM5M","name":"Daily Mix 1","ownerV2":{"data":{"uri":"spotify:user:spotify","name":"Spotify"}},"images":{"items":[{"sources":[{"url":"https://i/dm1"}]}]}}}}
				]}}
			]}}}}}`))
		case "fetchPlaylist":
			if req.Variables["uri"] == "spotify:playlist:missing" {
				w.Write([]byte(`{"data":{"playlistV2":{"__typename":"NotFound"}}}`))
				return
			}
			off := int(req.Variables["offset"].(float64))
			if off == 0 {
				w.Write([]byte(`{"data":{"playlistV2":{"__typename":"Playlist","uri":"spotify:playlist:p1","name":"Editorial Mix","description":"Top","following":true,"ownerV2":{"data":{"uri":"spotify:user:spotify","name":"Spotify"}},"members":{"totalCount":1},"currentUserCapabilities":{"canEditItems":false},"images":{"items":[{"sources":[{"url":"https://i/p1"}]}]},"content":{"totalCount":2,"items":[
					{"uid":"u-1","itemV2":{"__typename":"TrackResponseWrapper","_uri":"spotify:track:t1","data":{"__typename":"Track","uri":"spotify:track:t1","name":"Track 1","trackDuration":{"totalMilliseconds":120000},"artists":{"items":[{"uri":"spotify:artist:a1","profile":{"name":"Daft Punk"}}]},"albumOfTrack":{"uri":"spotify:album:al1","name":"RAM","coverArt":{"sources":[]}}}}}
				]}}}}`))
				return
			}
			w.Write([]byte(`{"data":{"playlistV2":{"__typename":"Playlist","uri":"spotify:playlist:p1","content":{"totalCount":2,"items":[
				{"uid":"u-2","itemV2":{"__typename":"TrackResponseWrapper","_uri":"spotify:track:t2","data":{"__typename":"Track","uri":"spotify:track:t2","name":"Track 2","trackDuration":{"totalMilliseconds":130000},"artists":{"items":[{"uri":"spotify:artist:a1","profile":{"name":"Daft Punk"}}]},"albumOfTrack":{"uri":"spotify:album:al1","name":"RAM","coverArt":{"sources":[]}}}}}
			]}}}}`))
		case "getAlbum":
			w.Write([]byte(`{"data":{"albumUnion":{"__typename":"Album","uri":"spotify:album:al1","name":"RAM","type":"ALBUM","date":{"isoString":"2013-05-20T00:00:00Z","precision":"DAY"},"artists":{"items":[{"uri":"spotify:artist:a1","profile":{"name":"Daft Punk"}}]},"coverArt":{"sources":[{"url":"https://i/al1","width":640,"height":640}]},"tracksV2":{"totalCount":2,"items":[{"track":{"uri":"spotify:track:t1","name":"Track 1","trackNumber":1,"discNumber":1,"duration":{"totalMilliseconds":120000},"contentRating":{"label":"NONE"},"artists":{"items":[{"uri":"spotify:artist:a1","profile":{"name":"Daft Punk"}}]}}}]}}}}`))
		case "queryAlbumTracks":
			w.Write([]byte(`{"data":{"albumUnion":{"__typename":"Album","tracksV2":{"totalCount":2,"items":[{"track":{"uri":"spotify:track:t2","name":"Track 2","trackNumber":2,"discNumber":1,"duration":{"totalMilliseconds":130000},"contentRating":{"label":"EXPLICIT"},"artists":{"items":[{"uri":"spotify:artist:a1","profile":{"name":"Daft Punk"}}]}}}]}}}}`))
		case "queryArtistOverview":
			w.Write([]byte(`{"data":{"artistUnion":{"__typename":"Artist","uri":"spotify:artist:a1","profile":{"name":"Daft Punk"},"visuals":{"avatarImage":{"sources":[{"url":"https://i/a1","width":640,"height":640}]}},"discography":{"topTracks":{"items":[{"track":{"uri":"spotify:track:t1","name":"One More Time","duration":{"totalMilliseconds":320000},"artists":{"items":[{"uri":"spotify:artist:a1","profile":{"name":"Daft Punk"}}]},"albumOfTrack":{"uri":"spotify:album:al2","name":"Discovery","coverArt":{"sources":[]}}}}]},"albums":{"items":[{"releases":{"items":[{"uri":"spotify:album:al1","name":"RAM","type":"ALBUM","date":{"year":2013}}]}}]},"singles":{"items":[{"releases":{"items":[{"uri":"spotify:album:s1","name":"Get Lucky","type":"SINGLE","date":{"year":2013}}]}}]}},"relatedContent":{"relatedArtists":{"items":[{"uri":"spotify:artist:a2","profile":{"name":"Justice"}}]}}}}}`))
		case "queryArtistDiscographyAll":
			w.Write([]byte(`{"data":{"artistUnion":{"discography":{"all":{"totalCount":1,"items":[{"releases":{"items":[{"uri":"spotify:album:al1","id":"al1","name":"RAM","type":"ALBUM","date":{"year":2013,"month":5,"day":20,"precision":"DAY"}}]}}]}}}}}`))
		case "getTrack":
			w.Write([]byte(`{"data":{"trackUnion":{"__typename":"Track","id":"t1","uri":"spotify:track:t1","name":"Get Lucky","trackNumber":1,"duration":{"totalMilliseconds":247632},"contentRating":{"label":"NONE"},"firstArtist":{"items":[{"uri":"spotify:artist:a1","profile":{"name":"Daft Punk"}}]},"otherArtists":{"items":[{"uri":"spotify:artist:a2","profile":{"name":"Pharrell"}}]},"albumOfTrack":{"id":"al1","uri":"spotify:album:al1","name":"RAM","coverArt":{"sources":[{"url":"https://i/al1","width":640,"height":640}]}}}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ctx := context.Background()
	me, err := c.Me(ctx)
	if err != nil || me.ID != "u1" || me.Name != "Alice" {
		t.Fatalf("Me = %+v, %v", me, err)
	}

	sr, err := c.Search(ctx, "daft punk", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sr.Tracks) != 1 || len(sr.Albums) != 1 || len(sr.Artists) != 1 || len(sr.Playlists) != 1 {
		t.Fatalf("Search counts unexpected: %+v", sr)
	}
	if sr.Playlists[0].Title != "Today's Top Hits" || sr.Playlists[0].Editable {
		t.Errorf("Search should keep third-party playlists: %+v", sr.Playlists[0])
	}

	shelves, err := c.Home(ctx, "u1")
	if err != nil || len(shelves) != 1 || shelves[0].Key != "made-for-you" {
		t.Fatalf("Home = %+v, %v", shelves, err)
	}

	pl, err := c.Playlist(ctx, "p1", "u1")
	if err != nil || len(pl.Tracks) != 1 || !pl.Saved || pl.More == "" {
		t.Fatalf("Playlist = %+v, %v", pl, err)
	}
	moreTracks, after, _, err := c.PlaylistPageCounting(ctx, pl.More)
	if err != nil || len(moreTracks) != 1 || after != "" || moreTracks[0].Title != "Track 2" {
		t.Fatalf("PlaylistPageCounting = %+v, after=%q, err=%v", moreTracks, after, err)
	}
	if _, err := c.Playlist(ctx, "missing", "u1"); !errors.Is(err, ErrPlaylistNotShown) {
		t.Errorf("missing playlist err = %v, want ErrPlaylistNotShown", err)
	}

	al, err := c.Album(ctx, "al1")
	if err != nil || len(al.Tracks) != 2 || !al.Explicit {
		t.Fatalf("Album = %+v, %v", al, err)
	}

	ar, err := c.Artist(ctx, "a1")
	if err != nil || len(ar.TopTracks) != 1 || len(ar.Albums) != 1 || len(ar.Singles) != 1 || len(ar.Related) != 1 {
		t.Fatalf("Artist = %+v, %v", ar, err)
	}

	rels, err := c.ArtistReleases(ctx, "a1")
	if err != nil || len(rels) != 1 || rels[0].Released != "2013-05-20" {
		t.Fatalf("ArtistReleases = %+v, %v", rels, err)
	}

	songs, err := c.ArtistSongs(ctx, "a1", nil)
	if err != nil || len(songs) != 2 {
		t.Fatalf("ArtistSongs = %+v, %v", songs, err)
	}

	tr, err := c.Track(ctx, "t1")
	if err != nil || len(tr.Artists) != 2 || tr.DurationMs != 247632 {
		t.Fatalf("Track = %+v, %v", tr, err)
	}

	pods, err := c.SearchPodcasts(ctx, "daft")
	if err != nil || len(pods.Podcasts) != 1 || len(pods.Episodes) != 1 {
		t.Fatalf("SearchPodcasts = %+v, %v", pods, err)
	}
}

func TestGQLLibraryMutationsAndPodcasts(t *testing.T) {
	var seenOps []string
	var movePos map[string]any
	c, _ := newWebFake(t, func(op string, req gqlReq, w http.ResponseWriter, r *http.Request) {
		seenOps = append(seenOps, op)
		switch op {
		case "libraryV3":
			filters, _ := req.Variables["filters"].([]any)
			filter, _ := filters[0].(string)
			switch filter {
			case "Playlists":
				w.Write([]byte(`{"data":{"me":{"libraryV3":{"totalCount":2,"items":[
					{"item":{"__typename":"CollectionResponseWrapper","data":{"__typename":"Collection"}}},
					{"item":{"__typename":"PlaylistResponseWrapper","_uri":"spotify:playlist:p1","data":{"__typename":"Playlist","uri":"spotify:playlist:p1","name":"My List","ownerV2":{"data":{"uri":"spotify:user:u1","name":"Me"}},"content":{"totalCount":5},"images":{"items":[{"sources":[{"url":"https://i/p1"}]}]}}}}
				]}}}}`))
			case "Albums":
				w.Write([]byte(`{"data":{"me":{"libraryV3":{"totalCount":1,"items":[{"item":{"__typename":"AlbumResponseWrapper","_uri":"spotify:album:al1","data":{"__typename":"Album","uri":"spotify:album:al1","name":"Discovery","type":"ALBUM","date":{"year":2001}}}}]}}}}`))
			case "Artists":
				w.Write([]byte(`{"data":{"me":{"libraryV3":{"totalCount":1,"items":[{"item":{"__typename":"ArtistResponseWrapper","_uri":"spotify:artist:a1","data":{"__typename":"Artist","uri":"spotify:artist:a1","profile":{"name":"Daft Punk"}}}}]}}}}`))
			case "Podcasts & Shows":
				w.Write([]byte(`{"data":{"me":{"libraryV3":{"totalCount":1,"items":[{"item":{"__typename":"PodcastResponseWrapper","_uri":"spotify:show:sh1","data":{"__typename":"Podcast","uri":"spotify:show:sh1","name":"Dissect","publisher":{"name":"Spotify Studios"}}}}]}}}}`))
			}
		case "fetchLibraryTracks":
			w.Write([]byte(`{"data":{"me":{"library":{"tracks":{"totalCount":1,"items":[{"track":{"_uri":"spotify:track:t1","data":{"__typename":"Track","uri":"spotify:track:t1","name":"Liked Song","duration":{"totalMilliseconds":180000},"artists":{"items":[{"uri":"spotify:artist:a1","profile":{"name":"Daft Punk"}}]},"albumOfTrack":{"uri":"spotify:album:al1","name":"Discovery"}}}}]}}}}}`))
		case "addToLibrary", "removeFromLibrary", "addItemsToRootlist", "removeItemsFromRootlist", "addToPlaylist":
			w.Write([]byte(`{"data":{}}`))
		case "moveItemsInPlaylist":
			movePos, _ = req.Variables["newPosition"].(map[string]any)
			w.Write([]byte(`{"data":{}}`))
		case "fetchPlaylist":
			off := int(req.Variables["offset"].(float64))
			if req.Variables["limit"].(float64) == 1 && off == 0 {
				w.Write([]byte(`{"data":{"playlistV2":{"following":true,"content":{"totalCount":3,"items":[{"uid":"uid-0"}]}}}}`))
				return
			}
			w.Write([]byte(`{"data":{"playlistV2":{"following":true,"content":{"totalCount":3,"items":[{"uid":"uid-2"}]}}}}`))
		case "areEntitiesInLibrary":
			w.Write([]byte(`{"data":{"lookup":[
				{"__typename":"TrackResponseWrapper","data":{"__typename":"Track","saved":true}},
				{"__typename":"PlaylistResponseWrapper"}
			]}}`))
		case "queryShowMetadataV2":
			w.Write([]byte(`{"data":{"podcastUnionV2":{"__typename":"Podcast","id":"sh1","uri":"spotify:show:sh1","name":"Dissect","publisher":{"name":"Spotify Studios"},"episodesV2":{"totalCount":1}}}}`))
		case "queryPodcastEpisodes":
			w.Write([]byte(`{"data":{"podcastUnionV2":{"episodesV2":{"totalCount":1,"items":[{"entity":{"_uri":"spotify:episode:ep1","data":{"__typename":"Episode","id":"ep1","uri":"spotify:episode:ep1","name":"Ep 1","duration":{"totalMilliseconds":900000},"playedState":{"playPositionMilliseconds":12000,"state":"NOT_STARTED"}}}}]}}}}`))
		case "getEpisodeOrChapter":
			w.Write([]byte(`{"data":{"episodeUnionV2":{"__typename":"Episode","id":"ep1","uri":"spotify:episode:ep1","name":"Ep 1","description":"Check 01:23","duration":{"totalMilliseconds":900000},"releaseDate":{"isoString":"2026-03-17T00:00:00Z"},"podcastV2":{"data":{"id":"sh1","uri":"spotify:show:sh1","name":"Dissect"}}}}}`))
		}
	})

	ctx := context.Background()
	pls, err := c.MyPlaylists(ctx, "u1")
	if err != nil || len(pls) != 1 || pls[0].TrackCount != 5 {
		t.Fatalf("MyPlaylists = %+v, %v", pls, err)
	}
	als, err := c.SavedAlbums(ctx)
	if err != nil || len(als) != 1 || als[0].Title != "Discovery" {
		t.Fatalf("SavedAlbums = %+v, %v", als, err)
	}
	ars, err := c.FollowedArtists(ctx)
	if err != nil || len(ars) != 1 || ars[0].Name != "Daft Punk" {
		t.Fatalf("FollowedArtists = %+v, %v", ars, err)
	}
	tracks, total, err := c.SavedTracks(ctx, 0)
	if err != nil || total != 1 || len(tracks) != 1 {
		t.Fatalf("SavedTracks = %+v, total=%d, err=%v", tracks, total, err)
	}
	shows, err := c.SavedShows(ctx)
	if err != nil || len(shows) != 1 || shows[0].Title != "Dissect" {
		t.Fatalf("SavedShows = %+v, %v", shows, err)
	}

	if err := c.Save(ctx, "spotify:track:t1"); err != nil {
		t.Fatal(err)
	}
	if err := c.Remove(ctx, "spotify:track:t1"); err != nil {
		t.Fatal(err)
	}
	if err := c.FollowPlaylist(ctx, "p1", true); err != nil {
		t.Fatal(err)
	}
	has, err := c.Contains(ctx, "spotify:track:t1", "spotify:playlist:p1")
	if err != nil || !slices.Equal(has, []bool{true, true}) {
		t.Fatalf("Contains = %v, %v", has, err)
	}
	if err := c.AddToPlaylist(ctx, "p1", "spotify:track:t1"); err != nil {
		t.Fatal(err)
	}
	if err := c.MoveInPlaylist(ctx, "p1", 0, 1); err != nil {
		t.Fatal(err)
	}
	if movePos["moveType"] != "BEFORE_UID" || movePos["fromUid"] != "uid-2" {
		t.Errorf("movePos = %#v", movePos)
	}

	show, err := c.Show(ctx, "sh1")
	if err != nil || len(show.Episodes) != 1 || show.Episodes[0].Episode.ResumeMs != 12000 {
		t.Fatalf("Show = %+v, %v", show, err)
	}
	ep, err := c.EpisodeDetails(ctx, "ep1")
	if err != nil || ep.Show.Name != "Dissect" {
		t.Fatalf("EpisodeDetails = %+v, %v", ep, err)
	}
	if !slices.Contains(seenOps, "addItemsToRootlist") || !slices.Contains(seenOps, "addToLibrary") {
		t.Errorf("expected rootlist and library mutations, saw %v", seenOps)
	}
}

func TestGQLHashSyncAndAuthRetry(t *testing.T) {
	var authAttempt atomic.Int32
	var hashAttempts atomic.Int32
	var c *Client

	var srv *httptest.Server
	c, srv = newWebFake(t, func(op string, req gqlReq, w http.ResponseWriter, r *http.Request) {
		switch op {
		case "profileAttributes":
			if authAttempt.Add(1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.Header.Get("Authorization") != "Bearer WEB-REFRESHED" {
				t.Errorf("expected refreshed bearer, got %q", r.Header.Get("Authorization"))
			}
			w.Write([]byte(`{"data":{"me":{"profile":{"uri":"spotify:user:u1","name":"Refreshed"}}}}`))
		case "searchDesktop":
			n := hashAttempts.Add(1)
			if n <= 2 {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			if req.Extensions.PersistedQuery.SHA256Hash != "new-verified-search-hash" {
				t.Errorf("expected synced hash, got %q", req.Extensions.PersistedQuery.SHA256Hash)
			}
			w.Write([]byte(`{"data":{"searchV2":{"tracksV2":{"items":[]},"albumsV2":{"items":[]},"artists":{"items":[]},"playlists":{"items":[]}}}}`))
		}
	})

	hashesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"operations":{
			"searchDesktop":{"hash":"new-verified-search-hash","status":"verified"},
			"profileAttributes":{"hash":"should-be-ignored","status":"not_in_bundle"}
		}}`))
	}))
	defer hashesSrv.Close()
	c.HashesURL = hashesSrv.URL
	_ = srv

	me, err := c.Me(context.Background())
	if err != nil || me.Name != "Refreshed" {
		t.Fatalf("Me after 401 refresh = %+v, %v", me, err)
	}
	if _, err := c.Search(context.Background(), "test", "u1"); err != nil {
		t.Fatalf("Search after hash sync = %v", err)
	}
	if got, _ := c.hashes.hash("profileAttributes"); got == "should-be-ignored" {
		t.Error("status=not_in_bundle overwrote verified hash")
	}
}
