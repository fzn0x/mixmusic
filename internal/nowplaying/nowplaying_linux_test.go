//go:build linux && (amd64 || arm64)

package nowplaying

import (
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ebitengine/purego"
)

// What a desktop calls, to try the player as one would.
var (
	testOnce sync.Once

	gVariantLookupValue              func(dict uintptr, key string, typ uintptr) uintptr
	gVariantGetVariant               func(v uintptr) uintptr
	gVariantNChildren                func(v uintptr) uintptr
	gVariantGetBoolean               func(v uintptr) bool
	gDBusConnectionSignalSubscribe   func(conn uintptr, sender, iface, member *byte, path string, arg0 *byte, flags int32, callback, user, free uintptr) uint32
	gDBusConnectionSignalUnsubscribe func(conn uintptr, id uint32)
	gDBusAddressGetForBusSync        func(busType int32, cancellable uintptr, err **gError) uintptr
	gDBusConnectionNewForAddressSync func(address uintptr, flags int32, observer, cancellable uintptr, err **gError) uintptr
	gFree                            func(p uintptr)
	testSignalCallback               uintptr
	testSignals                      atomic.Pointer[chan testSignal]
)

// connectionFlags are G_DBUS_CONNECTION_FLAGS_AUTHENTICATION_CLIENT and
// …_MESSAGE_BUS_CONNECTION.
const connectionFlags = 1 | 8

type testSignal struct {
	name     string
	props    map[string]bool
	position int64
}

// testBus is a client of the session bus.
type testBus struct {
	t    *testing.T
	conn uintptr
}

func newTestBus(t *testing.T) *testBus {
	t.Helper()
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no session bus: run the test under dbus-run-session")
	}
	if err := loadGIO(); err != nil {
		t.Skip(err)
	}
	testOnce.Do(func() {
		for name, fn := range map[string]any{
			"g_variant_lookup_value":                 &gVariantLookupValue,
			"g_variant_get_variant":                  &gVariantGetVariant,
			"g_variant_n_children":                   &gVariantNChildren,
			"g_variant_get_boolean":                  &gVariantGetBoolean,
			"g_dbus_connection_signal_subscribe":     &gDBusConnectionSignalSubscribe,
			"g_dbus_connection_signal_unsubscribe":   &gDBusConnectionSignalUnsubscribe,
			"g_dbus_address_get_for_bus_sync":        &gDBusAddressGetForBusSync,
			"g_dbus_connection_new_for_address_sync": &gDBusConnectionNewForAddressSync,
			"g_free":                                 &gFree,
		} {
			purego.RegisterLibFunc(fn, gioLib, name)
		}
		testSignalCallback = purego.NewCallback(func(_ uintptr, _, _, _, member *byte, params, _ uintptr) {
			ch := testSignals.Load()
			if ch == nil {
				return
			}
			s := testSignal{name: goString(member), props: map[string]bool{}}
			switch s.name {
			case "PropertiesChanged":
				dict := gVariantGetChildValue(params, 1)
				for i := range gVariantNChildren(dict) {
					e := gVariantGetChildValue(dict, i)
					s.props[stringArg(e, 0)] = true
					gVariantUnref(e)
				}
				gVariantUnref(dict)
			case "Seeked":
				s.position = int64Arg(params, 0)
			}
			select {
			case *ch <- s:
			default:
			}
		})
	})
	var gerr *gError
	conn := gBusGetSync(gBusTypeSession, 0, &gerr)
	if conn == 0 {
		t.Fatalf("the session bus: %v", takeError(gerr))
	}
	t.Cleanup(func() { gObjectUnref(conn) })
	return &testBus{t: t, conn: conn}
}

// call calls a method, and returns its reply, to unref, or the error.
func (b *testBus) call(dest, path, iface, method string, params uintptr, reply string) (uintptr, error) {
	var typ uintptr
	if reply != "" {
		typ = gVariantTypeNew(reply)
		defer gVariantTypeFree(typ)
	}
	var gerr *gError
	v := gDBusConnectionCallSync(b.conn, dest, path, iface, method, params, typ, 0, callTimeoutMs, 0, &gerr)
	if v == 0 {
		return 0, takeError(gerr)
	}
	return v, nil
}

// player calls a method of the player.
func (b *testBus) player(name, method string, params uintptr) error {
	if params == 0 {
		params = tuple()
	}
	v, err := b.call(name, objectPath, playerInterface, method, params, "")
	if err == nil {
		gVariantUnref(v)
	}
	return err
}

// get returns a property of the player, to unref.
func (b *testBus) get(name, iface, prop string) uintptr {
	b.t.Helper()
	reply, err := b.call(name, objectPath, "org.freedesktop.DBus.Properties", "Get",
		tuple(gVariantNewString(iface), gVariantNewString(prop)), "(v)")
	if err != nil {
		b.t.Fatalf("Get %s: %v", prop, err)
	}
	defer gVariantUnref(reply)
	boxed := gVariantGetChildValue(reply, 0)
	defer gVariantUnref(boxed)
	return gVariantGetVariant(boxed)
}

func (b *testBus) getString(name, iface, prop string) string {
	v := b.get(name, iface, prop)
	defer gVariantUnref(v)
	return gVariantGetString(v, 0)
}

func (b *testBus) getBool(name, iface, prop string) bool {
	v := b.get(name, iface, prop)
	defer gVariantUnref(v)
	return gVariantGetBoolean(v)
}

func (b *testBus) getDouble(name, prop string) float64 {
	v := b.get(name, playerInterface, prop)
	defer gVariantUnref(v)
	return gVariantGetDouble(v)
}

func (b *testBus) getInt64(name, prop string) int64 {
	v := b.get(name, playerInterface, prop)
	defer gVariantUnref(v)
	return gVariantGetInt64(v)
}

func (b *testBus) set(name, prop string, value uintptr) error {
	v, err := b.call(name, objectPath, "org.freedesktop.DBus.Properties", "Set",
		tuple(gVariantNewString(playerInterface), gVariantNewString(prop), gVariantNewVariant(value)), "")
	if err == nil {
		gVariantUnref(v)
	}
	return err
}

func (b *testBus) hasOwner(name string) bool {
	b.t.Helper()
	reply, err := b.call("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "NameHasOwner",
		tuple(gVariantNewString(name)), "(b)")
	if err != nil {
		b.t.Fatalf("NameHasOwner: %v", err)
	}
	defer gVariantUnref(reply)
	v := gVariantGetChildValue(reply, 0)
	defer gVariantUnref(v)
	return gVariantGetBoolean(v)
}

// listen gathers the player's signals until the test ends.
func (b *testBus) listen() <-chan testSignal {
	ch := make(chan testSignal, 64)
	testSignals.Store(&ch)
	var quit atomic.Bool
	ready, done := make(chan uintptr), make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		ctx := gMainContextNew()
		gMainContextPushThreadDefault(ctx)
		id := gDBusConnectionSignalSubscribe(b.conn, nil, nil, nil, objectPath, nil, 0, testSignalCallback, 0, 0)
		ready <- ctx
		for !quit.Load() {
			gMainContextIteration(ctx, true)
		}
		gDBusConnectionSignalUnsubscribe(b.conn, id)
		gMainContextPopThreadDefault(ctx)
		gMainContextUnref(ctx)
	}()
	ctx := <-ready
	b.t.Cleanup(func() {
		testSignals.Store(nil)
		quit.Store(true)
		gMainContextWakeup(ctx)
		<-done
	})
	// The bus has the subscription once it answers what follows it.
	if v, err := b.call("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "GetId", tuple(), "(s)"); err != nil {
		b.t.Fatalf("GetId: %v", err)
	} else {
		gVariantUnref(v)
	}
	return ch
}

func nextSignal(t *testing.T, ch <-chan testSignal, name string) testSignal {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case s := <-ch:
			if s.name == name {
				return s
			}
		case <-timeout:
			t.Fatalf("no %s signal", name)
		}
	}
}

func request(t *testing.T, got <-chan Request) Request {
	t.Helper()
	select {
	case r := <-got:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("no request")
	}
	return Request{}
}

func TestMPRIS(t *testing.T) {
	b := newTestBus(t)
	got := make(chan Request, requests)
	c, err := Open(func(r Request) { got <- r }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if b.hasOwner(busName) {
		t.Fatal("the player is on the bus before anything shows")
	}

	info := Info{
		ID:       "yt:abc-DEF_12",
		Title:    "Song",
		Artists:  []string{"Ann", "Bo"},
		Album:    "Album",
		Artwork:  "https://example.com/art.jpg",
		Duration: 200 * time.Second,
		Position: 10 * time.Second,
		At:       time.Now(),
		Playing:  true,
		Volume:   0.5,
	}
	c.Show(&info)
	eventually(t, "the player on the bus", func() bool { return b.hasOwner(busName) })

	if got := b.getString(busName, rootInterface, "Identity"); got != "MixMusic" {
		t.Errorf("Identity %q", got)
	}
	if got := b.getString(busName, rootInterface, "DesktopEntry"); got != "mixmusic" {
		t.Errorf("DesktopEntry %q", got)
	}
	if !b.getBool(busName, rootInterface, "CanRaise") || b.getBool(busName, rootInterface, "CanQuit") {
		t.Error("CanRaise or CanQuit")
	}
	if got := b.getString(busName, playerInterface, "PlaybackStatus"); got != "Playing" {
		t.Errorf("PlaybackStatus %q", got)
	}
	if got := b.getDouble(busName, "Volume"); got != 0.5 {
		t.Errorf("Volume %v", got)
	}
	if got := b.getInt64(busName, "Position"); got < 10e6 || got > 12e6 {
		t.Errorf("Position %d µs, want about 10 s", got)
	}
	for _, p := range []string{"CanSeek", "CanPlay", "CanPause", "CanGoNext", "CanGoPrevious", "CanControl"} {
		if !b.getBool(busName, playerInterface, p) {
			t.Errorf("%s is false", p)
		}
	}

	md := b.get(busName, playerInterface, "Metadata")
	str := func(key string) string {
		v := gVariantLookupValue(md, key, 0)
		if v == 0 {
			t.Errorf("Metadata has no %s", key)
			return ""
		}
		defer gVariantUnref(v)
		return gVariantGetString(v, 0)
	}
	if got := str("mpris:trackid"); got != trackID(info.ID) {
		t.Errorf("mpris:trackid %q", got)
	}
	if got := str("xesam:title"); got != "Song" {
		t.Errorf("xesam:title %q", got)
	}
	if got := str("xesam:album"); got != "Album" {
		t.Errorf("xesam:album %q", got)
	}
	if got := str("mpris:artUrl"); got != info.Artwork {
		t.Errorf("mpris:artUrl %q", got)
	}
	if v := gVariantLookupValue(md, "mpris:length", 0); v == 0 || gVariantGetInt64(v) != 200e6 {
		t.Error("mpris:length isn't 200 s")
	} else {
		gVariantUnref(v)
	}
	if v := gVariantLookupValue(md, "xesam:artist", 0); v == 0 || gVariantNChildren(v) != 2 || stringArg(v, 0) != "Ann" || stringArg(v, 1) != "Bo" {
		t.Error("xesam:artist isn't Ann and Bo")
	} else {
		gVariantUnref(v)
	}
	gVariantUnref(md)

	// What a desktop asks.
	for method, want := range map[string]Command{
		"PlayPause": Toggle, "Play": Play, "Pause": Pause, "Stop": Pause, "Next": Next, "Previous": Previous,
	} {
		if err := b.player(busName, method, 0); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		if r := request(t, got); r.Command != want {
			t.Errorf("%s asked %v, want %v", method, r.Command, want)
		}
	}
	if v, err := b.call(busName, objectPath, rootInterface, "Raise", tuple(), ""); err != nil {
		t.Fatalf("Raise: %v", err)
	} else {
		gVariantUnref(v)
	}
	if r := request(t, got); r.Command != Raise {
		t.Errorf("Raise asked %v", r.Command)
	}
	// A position for another track is dropped: only the second asks.
	if err := b.player(busName, "SetPosition", tuple(gVariantNewObjectPath(trackID("yt:other")), gVariantNewInt64(50e6))); err != nil {
		t.Fatal(err)
	}
	if err := b.player(busName, "SetPosition", tuple(gVariantNewObjectPath(trackID(info.ID)), gVariantNewInt64(30e6))); err != nil {
		t.Fatal(err)
	}
	if r := request(t, got); r.Command != Seek || r.Position != 30*time.Second {
		t.Errorf("SetPosition asked %+v", r)
	}
	if err := b.player(busName, "Seek", tuple(gVariantNewInt64(5e6))); err != nil {
		t.Fatal(err)
	}
	if r := request(t, got); r.Command != Seek || r.Position < 15*time.Second || r.Position > 17*time.Second {
		t.Errorf("Seek asked %+v, want about 15 s", r)
	}
	if err := b.player(busName, "Seek", tuple(gVariantNewInt64(1000e6))); err != nil {
		t.Fatal(err)
	}
	if r := request(t, got); r.Command != Next {
		t.Errorf("a seek past the end asked %+v, want Next", r)
	}
	if err := b.set(busName, "Volume", gVariantNewDouble(0.25)); err != nil {
		t.Fatal(err)
	}
	if r := request(t, got); r.Command != SetVolume || r.Volume != 0.25 {
		t.Errorf("setting Volume asked %+v", r)
	}
	if err := b.set(busName, "Volume", gVariantNewDouble(3)); err != nil {
		t.Fatal(err)
	}
	if r := request(t, got); r.Command != SetVolume || r.Volume != 1 {
		t.Errorf("a volume of 3 asked %+v", r)
	}
	if err := b.player(busName, "OpenUri", tuple(gVariantNewString("https://example.com"))); err == nil {
		t.Error("OpenUri didn't fail")
	}
	if err := b.set(busName, "PlaybackStatus", gVariantNewString("Paused")); err == nil {
		t.Error("PlaybackStatus was set")
	}

	// What it tells.
	signals := b.listen()
	paused := info
	paused.Playing, paused.Position, paused.At = false, 11*time.Second, time.Now()
	c.Show(&paused)
	if s := nextSignal(t, signals, "PropertiesChanged"); !s.props["PlaybackStatus"] || s.props["Metadata"] {
		t.Errorf("pausing changed %v", s.props)
	}
	if got := b.getString(busName, playerInterface, "PlaybackStatus"); got != "Paused" {
		t.Errorf("PlaybackStatus %q once paused", got)
	}
	other := paused
	other.ID, other.Title, other.Position = "yt:other", "Other", 0
	c.Show(&other)
	if s := nextSignal(t, signals, "PropertiesChanged"); !s.props["Metadata"] {
		t.Errorf("another track changed %v", s.props)
	}
	seeked := other
	seeked.Position, seeked.At = 100*time.Second, time.Now()
	c.Show(&seeked)
	if s := nextSignal(t, signals, "Seeked"); s.position != 100e6 {
		t.Errorf("Seeked to %d µs, want 100 s", s.position)
	}
	louder := seeked
	louder.Volume = 0.75
	c.Show(&louder)
	if s := nextSignal(t, signals, "PropertiesChanged"); !s.props["Volume"] || len(s.props) != 1 {
		t.Errorf("the volume changed %v", s.props)
	}

	c.Show(nil)
	eventually(t, "the player leaving the bus", func() bool { return !b.hasOwner(busName) })
	c.Show(&louder)
	eventually(t, "the player back on the bus", func() bool { return b.hasOwner(busName) })
}

// TestMPRISNameTaken has the player take a name of its process when
// another player has its own.
func TestMPRISNameTaken(t *testing.T) {
	b := newTestBus(t)
	var gerr *gError
	addr := gDBusAddressGetForBusSync(gBusTypeSession, 0, &gerr)
	if addr == 0 {
		t.Fatal(takeError(gerr))
	}
	other := gDBusConnectionNewForAddressSync(addr, connectionFlags, 0, 0, &gerr)
	gFree(addr)
	if other == 0 {
		t.Fatal(takeError(gerr))
	}
	defer gObjectUnref(other)
	o := &testBus{t: t, conn: other}
	if v, err := o.call("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "RequestName",
		tuple(gVariantNewString(busName), gVariantNewUint32(dbusNameFlagDoNotQueue)), "(u)"); err != nil {
		t.Fatal(err)
	} else {
		gVariantUnref(v)
	}

	c, err := Open(func(Request) {}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Show(&Info{ID: "yt:1", Title: "One"})
	instance := busName + ".instance" + strconv.Itoa(os.Getpid())
	eventually(t, "the player on the bus", func() bool { return b.hasOwner(instance) })
	if got := b.getString(instance, playerInterface, "PlaybackStatus"); got != "Paused" {
		t.Errorf("PlaybackStatus %q", got)
	}
}
