package browser

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// The fixtures of testdata are cookie stores as Firefox keeps them, which
// SQLite itself wrote (testdata/make_fixtures.py), their values made up:
// cookies.sqlite, of the current schema, whose last changes are in its
// write-ahead log; cookies-new.sqlite, whose file holds its first page
// alone, and its log the rest; and cookies-v12.sqlite, of an older
// schema, without a log.

// fixture copies a cookie store of testdata, with its log, into a folder
// of its own, as cookies.sqlite.
func fixture(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	for _, suffix := range []string{"", "-wal"} {
		data, err := os.ReadFile(filepath.Join("testdata", name+suffix))
		if os.IsNotExist(err) && suffix != "" {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cookies.sqlite"+suffix), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// cookieList writes cookies as name=value@domain, sorted.
func cookieList(t *testing.T, dir string) []string {
	t.Helper()
	cookies, err := firefoxCookies(dir)
	if err != nil {
		t.Fatal(err)
	}
	var list []string
	for _, c := range cookies {
		if !strings.HasPrefix(c.Name, "F") {
			v := c.Value
			if len(v) > 20 {
				v = v[:4] + "…" + v[len(v)-3:]
			}
			list = append(list, c.Name+"="+v+"@"+c.Domain)
		}
	}
	slices.Sort(list)
	return list
}

// TestFirefoxCookies reads Firefox's cookies as of the last commit of its
// log, with the long value that overflows its page, but none of a
// container or kept for a site embedded in another.
func TestFirefoxCookies(t *testing.T) {
	dir := fixture(t, "cookies.sqlite")
	want := []string{"HOSTONLY=music-only@music.youtube.com", "LOGIN_INFO=LLLL…end@.youtube.com", "NID=made-up-nid@.google.com",
		"SAPISID=new-sapisid@.youtube.com", "SID=made-up-sid@.youtube.com", "__Secure-3PAPISID=made-up-3p@.youtube.com"}
	if got := cookieList(t, dir); !slices.Equal(got, want) {
		t.Errorf("read %q, want %q", got, want)
	}
	cookies, err := firefoxCookies(dir)
	if err != nil {
		t.Fatal(err)
	}
	fillers := 0
	for _, c := range cookies {
		switch {
		case strings.HasPrefix(c.Name, "F"):
			fillers++
			if c.Name == "F000" {
				t.Error("the row the log deleted was read")
			}
		case c.Name == "LOGIN_INFO":
			if len(c.Value) != 3003 || !c.HttpOnly || !c.Secure {
				t.Errorf("LOGIN_INFO came as %d letters, httpOnly %v, secure %v", len(c.Value), c.HttpOnly, c.Secure)
			}
		}
		if want := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC); !c.Expires.Equal(want) {
			t.Errorf("%s expires %v, want %v", c.Name, c.Expires, want)
		}
	}
	if fillers != 79 {
		t.Errorf("read %d of the 79 other cookies, which take pages below others", fillers)
	}

	// Without its log, the file is the database as it was before.
	os.Remove(filepath.Join(dir, "cookies.sqlite-wal"))
	if got := cookieList(t, dir); !slices.Contains(got, "SAPISID=old-sapisid@.youtube.com") || slices.Contains(got, "__Secure-3PAPISID=made-up-3p@.youtube.com") {
		t.Errorf("without the log, read %q", got)
	}
}

// TestFirefoxCookiesInTheLog reads a new store, whose file holds nothing
// yet but its first page.
func TestFirefoxCookiesInTheLog(t *testing.T) {
	if got, want := cookieList(t, fixture(t, "cookies-new.sqlite")), []string{"SAPISID=only-in-the-log@.youtube.com"}; !slices.Equal(got, want) {
		t.Errorf("read %q, want %q", got, want)
	}
}

// TestFirefoxCookiesInSeconds reads the expiry of an older Firefox, in
// seconds.
func TestFirefoxCookiesInSeconds(t *testing.T) {
	cookies, err := firefoxCookies(fixture(t, "cookies-v12.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cookies) != 1 || cookies[0].Value != "in-seconds" || !cookies[0].Expires.Equal(time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("read %+v", cookies)
	}
}

// TestFirefoxCookiesNotYet waits, as there is no store yet, while Firefox
// starts.
func TestFirefoxCookiesNotYet(t *testing.T) {
	if _, err := firefoxCookies(t.TempDir()); !os.IsNotExist(err) {
		t.Errorf("no store gave %v", err)
	}
}

// TestLogFrames leaves out of the log the frames after its last commit,
// those whose checksum fails, and those of another log, whose salts
// differ; and the whole log when its header's checksum fails.
func TestLogFrames(t *testing.T) {
	file, err := os.ReadFile(filepath.Join("testdata", "cookies.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	wal, err := os.ReadFile(filepath.Join("testdata", "cookies.sqlite-wal"))
	if err != nil {
		t.Fatal(err)
	}
	const pageSize = 1024
	frames := (len(wal) - 32) / (24 + pageSize)
	if got := len(readLog(wal, pageSize)); got == 0 || frames < 2 {
		t.Fatalf("the log has %d frames, %d pages", frames, got)
	}
	sapisid := func(wal []byte) string {
		t.Helper()
		db, err := openSQLite(file, wal)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := db.table("moz_cookies")
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.text("name") == "SAPISID" && r.text("originAttributes") == "" {
				return r.text("value")
			}
		}
		return ""
	}
	if got := sapisid(wal); got != "new-sapisid" {
		t.Errorf("with the log, SAPISID is %q", got)
	}
	// The last frame commits: without it, nothing of the transaction is.
	cut := wal[:len(wal)-(24+pageSize)]
	if got := sapisid(cut); got != "old-sapisid" {
		t.Errorf("with the commit cut off, SAPISID is %q", got)
	}
	// A frame whose page changed fails its checksum, as do those after.
	bad := slices.Clone(wal)
	bad[32+24+100] ^= 0xff
	if got := sapisid(bad); got != "old-sapisid" {
		t.Errorf("with a damaged frame, SAPISID is %q", got)
	}
	// A frame of a log before, whose salts differ.
	stale := slices.Clone(wal)
	binary.BigEndian.PutUint32(stale[32+8:], binary.BigEndian.Uint32(stale[32+8:])+1)
	if got := sapisid(stale); got != "old-sapisid" {
		t.Errorf("with a stale frame, SAPISID is %q", got)
	}
	header := slices.Clone(wal)
	header[12]++ // the checkpoint's number, which the header's checksum covers
	if got := sapisid(header); got != "old-sapisid" {
		t.Errorf("with a damaged header, SAPISID is %q", got)
	}
}

// TestDamagedDatabase reads every cut and every changed byte of a store
// without failing other than with an error: Firefox may be writing it.
func TestDamagedDatabase(t *testing.T) {
	file, err := os.ReadFile(filepath.Join("testdata", "cookies.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	wal, err := os.ReadFile(filepath.Join("testdata", "cookies.sqlite-wal"))
	if err != nil {
		t.Fatal(err)
	}
	read := func(file, wal []byte) {
		if db, err := openSQLite(file, wal); err == nil {
			db.table("moz_cookies")
		}
	}
	for n := 0; n < len(file); n += 97 {
		read(file[:n], wal)
		read(file, wal[:min(n, len(wal))])
	}
	for i := 0; i < len(file); i += 7 {
		changed := slices.Clone(file)
		changed[i] ^= 0x5a
		read(changed, nil)
	}
	if _, err := openSQLite([]byte("not a database at all, but text"), nil); err != errNotSQLite {
		t.Errorf("text gave %v", err)
	}
}

// TestRecord reads each kind of value of a row.
func TestRecord(t *testing.T) {
	payload := []byte{
		10,                           // the header's size
		0, 1, 2, 4, 6, 7, 8, 9, 0x13, // NULL, ints of 1, 2, 4 and 8 bytes, a float, 0, 1, 3 letters
		0xff,
		0x01, 0x00,
		0xff, 0xff, 0xff, 0xfe,
		0x00, 0x00, 0x01, 0x8f, 0x00, 0x00, 0x00, 0x00,
		0x3f, 0xf8, 0, 0, 0, 0, 0, 0,
		'a', 'b', 'c',
	}
	got, err := record(payload)
	if err != nil {
		t.Fatal(err)
	}
	want := []any{nil, int64(-1), int64(256), int64(-2), int64(0x18f00000000), 1.5, int64(0), int64(1), "abc"}
	if !slices.Equal(got, want) {
		t.Errorf("read %v, want %v", got, want)
	}
	if _, err := record([]byte{3, 1, 1, 7}); err == nil {
		t.Error("a row shorter than its header says was read")
	}
}

// TestUvarint reads SQLite's integers of one to nine bytes.
func TestUvarint(t *testing.T) {
	for _, c := range []struct {
		in   []byte
		want uint64
		n    int
	}{
		{[]byte{0x7f}, 127, 1},
		{[]byte{0x81, 0x00}, 128, 2},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 1<<64 - 1, 9},
		{[]byte{0x81}, 0, 0},
	} {
		if got, n := uvarint(c.in); got != c.want || n != c.n {
			t.Errorf("uvarint(% x) = %d, %d; want %d, %d", c.in, got, n, c.want, c.n)
		}
	}
}

// TestColumnsOf reads the columns of a CREATE TABLE statement, and which
// is the row ID, but not its constraints.
func TestColumnsOf(t *testing.T) {
	names, rowid := columnsOf(`CREATE TABLE moz_cookies (id INTEGER PRIMARY KEY, "name" TEXT, value TEXT DEFAULT 'a,b', ` +
		`expiry INTEGER CHECK (expiry > 0), CONSTRAINT moz_uniqueid UNIQUE (name, value))`)
	if !slices.Equal(names, []string{"id", "name", "value", "expiry"}) || rowid != 0 {
		t.Errorf("columns %q, row ID %d", names, rowid)
	}
	if _, rowid := columnsOf("CREATE TABLE t (a TEXT, b INTEGER)"); rowid != -1 {
		t.Errorf("a table without one has row ID %d", rowid)
	}
}
