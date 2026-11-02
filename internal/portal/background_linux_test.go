//go:build linux && (amd64 || arm64)

package portal

import (
	"context"
	"errors"
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ebitengine/purego"
)

// A portal of the test's own, on a connection of its own to the session
// bus, stands in for the desktop's: run the test under dbus-run-session,
// as CI does.

var (
	testOnce sync.Once

	gDBusAddressGetForBusSync        func(busType int32, cancellable uintptr, err **gError) uintptr
	gDBusConnectionNewForAddressSync func(address uintptr, flags int32, observer, cancellable uintptr, err **gError) uintptr
	gDBusConnectionCloseSync         func(conn, cancellable uintptr, err **gError) bool
	gDBusNodeInfoNewForXML           func(xml string, err **gError) uintptr
	gDBusNodeInfoLookupInterface     func(info uintptr, name string) uintptr
	gDBusConnectionRegisterObject    func(conn uintptr, path string, iface uintptr, vtable *uintptr, user, free uintptr, err **gError) uint32
	gDBusConnectionUnregisterObject  func(conn uintptr, id uint32) bool
	gDBusConnectionEmitSignal        func(conn uintptr, dest *byte, path, iface, signal string, params uintptr, err **gError) bool
	gDBusMethodInvocationReturnValue func(inv, params uintptr)
	gVariantNewUint32                func(v uint32) uintptr
	gVariantNewObjectPath            func(s string) uintptr
	gVariantNChildren                func(v uintptr) uintptr
	gFree                            func(p uintptr)

	fakeNode   uintptr
	fakeVTable [11]uintptr
	theFake    atomic.Pointer[fakePortal]
)

const fakeXML = `<node>
  <interface name="org.freedesktop.portal.Background">
    <method name="RequestBackground">
      <arg type="s" name="parent_window" direction="in"/>
      <arg type="a{sv}" name="options" direction="in"/>
      <arg type="o" name="handle" direction="out"/>
    </method>
  </interface>
  <interface name="org.freedesktop.portal.Settings">
    <method name="Read">
      <arg type="s" name="namespace" direction="in"/>
      <arg type="s" name="key" direction="in"/>
      <arg type="v" name="value" direction="out"/>
    </method>
  </interface>
</node>`

// connectionFlags are G_DBUS_CONNECTION_FLAGS_AUTHENTICATION_CLIENT and
// …_MESSAGE_BUS_CONNECTION.
const connectionFlags = 1 | 8

// fakeRequest is what a request asked.
type fakeRequest struct {
	token, reason          string
	autostart, activatable bool
	commandline            []string
}

// fakePortal is the test's portal, which answers each request as told.
type fakePortal struct {
	t    *testing.T
	conn uintptr
	ctx  uintptr
	quit atomic.Bool
	done chan struct{}
	// background is the ID of the Background interface's registration.
	background atomic.Uint32

	mu sync.Mutex
	// allow lets MixMusic run in the background, and then start at
	// login as asked; silent answers no request.
	allow, silent bool
	asked         []fakeRequest
}

func startFake(t *testing.T) *fakePortal {
	t.Helper()
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no session bus: run the test under dbus-run-session")
	}
	if err := loadGIO(); err != nil {
		t.Skip(err)
	}
	testOnce.Do(func() {
		for name, fn := range map[string]any{
			"g_dbus_address_get_for_bus_sync":        &gDBusAddressGetForBusSync,
			"g_dbus_connection_new_for_address_sync": &gDBusConnectionNewForAddressSync,
			"g_dbus_connection_close_sync":           &gDBusConnectionCloseSync,
			"g_dbus_node_info_new_for_xml":           &gDBusNodeInfoNewForXML,
			"g_dbus_node_info_lookup_interface":      &gDBusNodeInfoLookupInterface,
			"g_dbus_connection_register_object":      &gDBusConnectionRegisterObject,
			"g_dbus_connection_unregister_object":    &gDBusConnectionUnregisterObject,
			"g_dbus_connection_emit_signal":          &gDBusConnectionEmitSignal,
			"g_dbus_method_invocation_return_value":  &gDBusMethodInvocationReturnValue,
			"g_variant_new_uint32":                   &gVariantNewUint32,
			"g_variant_new_object_path":              &gVariantNewObjectPath,
			"g_variant_n_children":                   &gVariantNChildren,
			"g_free":                                 &gFree,
		} {
			purego.RegisterLibFunc(fn, gioLib, name)
		}
		var gerr *gError
		if fakeNode = gDBusNodeInfoNewForXML(fakeXML, &gerr); fakeNode == 0 {
			panic(takeError(gerr))
		}
		fakeVTable[0] = purego.NewCallback(fakeMethod)
	})

	var gerr *gError
	addr := gDBusAddressGetForBusSync(gBusTypeSession, 0, &gerr)
	if addr == 0 {
		t.Fatalf("the session bus's address: %v", takeError(gerr))
	}
	conn := gDBusConnectionNewForAddressSync(addr, connectionFlags, 0, 0, &gerr)
	gFree(addr)
	if conn == 0 {
		t.Fatalf("a connection of the portal's own: %v", takeError(gerr))
	}
	f := &fakePortal{t: t, conn: conn, done: make(chan struct{}), allow: true}
	ready := make(chan error, 1)
	go f.serve(ready)
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	theFake.Store(f)
	t.Cleanup(func() {
		theFake.Store(nil)
		f.quit.Store(true)
		gMainContextWakeup(f.ctx)
		<-f.done
		gDBusConnectionCloseSync(f.conn, 0, nil)
		gObjectUnref(f.conn)
	})
	// The portal's name, which the requests go to.
	typ := gVariantTypeNew("(u)")
	defer gVariantTypeFree(typ)
	reply := gDBusConnectionCallSync(conn, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "RequestName",
		tuple(gVariantNewString(portalName), gVariantNewUint32(4)), typ, 0, callTimeoutMs, 0, &gerr)
	if reply == 0 {
		t.Fatalf("RequestName: %v", takeError(gerr))
	}
	defer gVariantUnref(reply)
	v := gVariantGetChildValue(reply, 0)
	defer gVariantUnref(v)
	if got := gVariantGetUint32(v); got != 1 {
		t.Fatalf("RequestName answered %d: another portal has the name", got)
	}
	return f
}

// serve registers the portal's object, and answers its calls on a main
// context of its own, until the test ends.
func (f *fakePortal) serve(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(f.done)
	f.ctx = gMainContextNew()
	gMainContextPushThreadDefault(f.ctx)
	defer func() {
		gMainContextPopThreadDefault(f.ctx)
		gMainContextUnref(f.ctx)
	}()
	var ids []uint32
	for _, name := range []string{backgroundInterface, "org.freedesktop.portal.Settings"} {
		var gerr *gError
		id := gDBusConnectionRegisterObject(f.conn, portalPath, gDBusNodeInfoLookupInterface(fakeNode, name), &fakeVTable[0], 0, 0, &gerr)
		if id == 0 {
			ready <- takeError(gerr)
			return
		}
		ids = append(ids, id)
	}
	f.background.Store(ids[0])
	ready <- nil
	for !f.quit.Load() {
		gMainContextIteration(f.ctx, true)
	}
	for _, id := range ids {
		gDBusConnectionUnregisterObject(f.conn, id)
	}
}

// removeBackground takes the Background interface away, as a desktop
// without it has none.
func (f *fakePortal) removeBackground() {
	if id := f.background.Swap(0); id != 0 {
		gDBusConnectionUnregisterObject(f.conn, id)
	}
}

func (f *fakePortal) answer(allow, silent bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allow, f.silent = allow, silent
}

func (f *fakePortal) requests() []fakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.asked)
}

// fakeMethod answers RequestBackground with the request's path, then its
// Response, as xdg-desktop-portal does: background is allowed or not, and
// autostart follows the request when it is.
func fakeMethod(_ uintptr, sender, _, _, _ *byte, params, inv, _ uintptr) {
	f := theFake.Load()
	if f == nil {
		gDBusMethodInvocationReturnValue(inv, 0)
		return
	}
	opts := gVariantGetChildValue(params, 1)
	req := fakeRequest{
		token:       lookupString(opts, "handle_token"),
		reason:      lookupString(opts, "reason"),
		autostart:   lookupBool(opts, "autostart"),
		activatable: lookupBool(opts, "dbus-activatable"),
		commandline: lookupStrings(opts, "commandline"),
	}
	gVariantUnref(opts)
	f.mu.Lock()
	f.asked = append(f.asked, req)
	allow, silent := f.allow, f.silent
	f.mu.Unlock()
	path := requestPath(goString(sender), req.token)
	gDBusMethodInvocationReturnValue(inv, tuple(gVariantNewObjectPath(path)))
	if silent {
		return
	}
	code := uint32(0)
	if !allow {
		code = 1
	}
	results := array("{sv}", []uintptr{
		entry("background", gVariantNewBoolean(allow)),
		entry("autostart", gVariantNewBoolean(allow && req.autostart)),
	})
	var gerr *gError
	if !gDBusConnectionEmitSignal(f.conn, nil, path, requestInterface, "Response", tuple(gVariantNewUint32(code), results), &gerr) {
		f.t.Errorf("the Response: %v", takeError(gerr))
	}
}

func lookupString(dict uintptr, key string) string {
	typ := gVariantTypeNew("s")
	defer gVariantTypeFree(typ)
	v := gVariantLookupValue(dict, key, typ)
	if v == 0 {
		return ""
	}
	defer gVariantUnref(v)
	return gVariantGetString(v, 0)
}

func lookupStrings(dict uintptr, key string) []string {
	typ := gVariantTypeNew("as")
	defer gVariantTypeFree(typ)
	v := gVariantLookupValue(dict, key, typ)
	if v == 0 {
		return nil
	}
	defer gVariantUnref(v)
	var ss []string
	for i := range gVariantNChildren(v) {
		s := gVariantGetChildValue(v, i)
		ss = append(ss, gVariantGetString(s, 0))
		gVariantUnref(s)
	}
	return ss
}

// TestBackgroundPortal asks a portal to start MixMusic at login, and to
// stop, as the desktop's would be asked, and hears its answers: allowed,
// refused, none, and no Background portal at all.
func TestBackgroundPortal(t *testing.T) {
	f := startFake(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	login := Background{Reason: "MixMusic starts when you log in", Autostart: true, Commandline: []string{"mixmusic", "--mygo-opened-at-login"}}

	got, err := RequestBackground(ctx, login)
	if err != nil {
		t.Fatal(err)
	}
	if got != (Answer{Background: true, Autostart: true}) {
		t.Errorf("allowed to start at login: %+v", got)
	}
	got, err = RequestBackground(ctx, Background{Reason: "MixMusic plays on in the tray", Commandline: login.Commandline})
	if err != nil {
		t.Fatal(err)
	}
	if got != (Answer{Background: true}) {
		t.Errorf("asked not to start at login: %+v", got)
	}
	asked := f.requests()
	if len(asked) != 2 {
		t.Fatalf("the portal was asked %d times, want 2", len(asked))
	}
	first := asked[0]
	if first.reason != login.Reason || !first.autostart || first.activatable || !slices.Equal(first.commandline, login.Commandline) {
		t.Errorf("the portal was asked %+v", first)
	}
	if first.token == "" || first.token == asked[1].token {
		t.Errorf("the requests' tokens are %q and %q: each needs one of its own", first.token, asked[1].token)
	}
	if asked[1].autostart {
		t.Error("the second request asked to start at login")
	}

	// The user said no, or forbade it before.
	f.answer(false, false)
	got, err = RequestBackground(ctx, login)
	if err != nil {
		t.Fatal(err)
	}
	if got != (Answer{}) {
		t.Errorf("refused: %+v", got)
	}

	// No answer: the request gives up when its context ends.
	f.answer(true, true)
	short, stop := context.WithTimeout(ctx, 300*time.Millisecond)
	defer stop()
	start := time.Now()
	if _, err := RequestBackground(short, login); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("unanswered: %v", err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("an unanswered request took %v to give up", d)
	}

	// A desktop without the Background portal.
	f.removeBackground()
	if _, err := RequestBackground(ctx, login); !errors.Is(err, ErrNoBackground) {
		t.Errorf("without a Background portal: %v", err)
	}
}
