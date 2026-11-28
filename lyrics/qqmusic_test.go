package lyrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQQMusicSearchGetByIDAndTranslation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://y.qq.com/" {
			t.Errorf("missing Referer header: %q", r.Header.Get("Referer"))
		}
		switch r.URL.Path {
		case "/soso/fcgi-bin/client_search_cp":
			w.Write([]byte(`{
				"data": {"song": {"list": [
					{"songmid": "mid1", "songname": "アイドル", "albumname": "THE BOOK 3", "interval": 213, "singer": [{"name": "YOASOBI"}]},
					{"songmid": "mid2", "songname": "アイドル (Live)", "albumname": "Live", "interval": 212, "singer": [{"name": "YOASOBI"}]},
					{"songmid": "mid3", "songname": "アイドル (Remix)", "albumname": "Remix", "interval": 210, "singer": [{"name": "YOASOBI"}]},
					{"songmid": "mid4", "songname": "アイドル (Acoustic)", "albumname": "Acoustic", "interval": 90, "singer": [{"name": "YOASOBI"}]}
				]}}
			}`))
		case "/lyric/fcgi-bin/fcg_query_lyric_new.fcg":
			switch r.URL.Query().Get("songmid") {
			case "mid1":
				w.Write([]byte(`{
					"retcode": 0,
					"lyric": "[ti:アイドル]\n[00:00.50]アイドル - YOASOBI\n[00:00.60]词：Ayase\n[00:00.70]Mix：Naoki Itai\n[00:00.80]OP：Sony Music\n[00:01.00]無敵の笑顔で荒らすメディア\n[00:04.00]It&#39;s a secret",
					"trans": "[00:00.00]//\n[00:01.00]Conquering the media with an invincible smile\n[00:04.00]It&#39;s a secret"
				}`))
			case "mid2", "mid3":
				w.Write([]byte(`{"retcode": 0, "lyric": "[00:01.00]Alt version\n[00:04.00]Second line"}`))
			case "mid4":
				w.Write([]byte(`{"retcode": 0, "lyric": "[00:01.00]Acoustic lazy line\n[00:04.00]End"}`))
			case "instr":
				w.Write([]byte(`{"retcode": 0, "lyric": "[00:00.00]此歌曲为没有填词的纯音乐，请您欣赏"}`))
			default:
				w.Write([]byte(`{"retcode": -1901, "lyric": ""}`))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	q := NewQQMusic("MixMusicTest")
	q.BaseURL = srv.URL

	l, err := q.Get(context.Background(), Query{Title: "アイドル", Artist: "YOASOBI", Duration: 213 * time.Second})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !l.Synced || l.Source != "QQ Music" {
		t.Fatalf("unexpected lyrics meta: %+v", l)
	}
	if len(l.Lines) != 2 || l.Lines[0].Text != "無敵の笑顔で荒らすメディア" || l.Lines[1].Text != "It's a secret" {
		t.Fatalf("unexpected stripped/unescaped lines: %+v", l.Lines)
	}
	if l.Translation == nil || len(l.Translation.Lines) != 2 || l.Translation.Lines[0].Text != "Conquering the media with an invincible smile" {
		t.Fatalf("unexpected translation: %+v", l.Translation)
	}

	cands, err := q.Search(context.Background(), Query{Title: "アイドル", Artist: "YOASOBI", Duration: 213 * time.Second})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(cands) != 4 || !cands[0].Loaded || cands[3].Loaded || cands[3].ID != "mid4" {
		t.Fatalf("expected 3 preloaded + 1 lazy candidate, got %+v", cands)
	}
	lazy, err := q.GetByID(context.Background(), "mid4")
	if err != nil || len(lazy.Lines) != 2 || lazy.Lines[0].Text != "Acoustic lazy line" {
		t.Fatalf("GetByID(mid4) = %+v, %v", lazy, err)
	}
	if _, err := q.GetByID(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing mid err = %v, want ErrNotFound", err)
	}
	inst, err := q.GetByID(context.Background(), "instr")
	if err != nil || !inst.Instrumental || inst.Source != "QQ Music" {
		t.Errorf("instrumental notice = %+v, %v", inst, err)
	}
}

func TestQQMusicRejectsWrongArtist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/soso/fcgi-bin/client_search_cp" {
			w.Write([]byte(`{"data":{"song":{"list":[{"songmid":"wrong","songname":"Nikon","interval":177,"singer":[{"name":"Oceans"}]}]}}}`))
			return
		}
		w.Write([]byte(`{"retcode":0,"lyric":"[00:01.00]Wrong song\n[00:03.00]Line two"}`))
	}))
	defer srv.Close()

	q := NewQQMusic("MixMusicTest")
	q.BaseURL = srv.URL
	if _, err := q.Get(context.Background(), Query{Title: "nikon", Artist: "guardin", Duration: 148 * time.Second}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong artist, got %v", err)
	}
}
