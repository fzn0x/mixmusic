//go:build linux && (amd64 || arm64)

package portal

// GIO's GDBus calls the portal, with purego as MyGo and the MPRIS player
// call GIO. A request answers at once with the path of a request object,
// and later with that object's Response signal, once the desktop decided,
// perhaps after asking the user: the signal comes on a GLib main context
// of the request's own, which it runs on its thread until the answer.

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	portalName          = "org.freedesktop.portal.Desktop"
	portalPath          = "/org/freedesktop/portal/desktop"
	backgroundInterface = "org.freedesktop.portal.Background"
	requestInterface    = "org.freedesktop.portal.Request"

	gBusTypeSession = 2
	callTimeoutMs   = 5000
)

// gError is GError.
type gError struct {
	domain  uint32
	code    int32
	message *byte
}

var (
	gioOnce sync.Once
	gioErr  error
	gioLib  uintptr

	gBusGetSync                      func(busType int32, cancellable uintptr, err **gError) uintptr
	gObjectUnref                     func(obj uintptr)
	gErrorFree                       func(err *gError)
	gDBusConnectionGetUniqueName     func(conn uintptr) string
	gDBusConnectionSignalSubscribe   func(conn uintptr, sender, iface, member, path string, arg0 *byte, flags int32, callback, user, free uintptr) uint32
	gDBusConnectionSignalUnsubscribe func(conn uintptr, id uint32)
	gDBusConnectionCallSync          func(conn uintptr, name, path, iface, method string, params, replyType uintptr, flags, timeoutMs int32, cancellable uintptr, err **gError) uintptr
	gMainContextNew                  func() uintptr
	gMainContextPushThreadDefault    func(ctx uintptr)
	gMainContextPopThreadDefault     func(ctx uintptr)
	gMainContextIteration            func(ctx uintptr, mayBlock bool) bool
	gMainContextWakeup               func(ctx uintptr)
	gMainContextUnref                func(ctx uintptr)
	gVariantNewString                func(s string) uintptr
	gVariantNewBoolean               func(v bool) uintptr
	gVariantNewVariant               func(v uintptr) uintptr
	gVariantNewDictEntry             func(key, value uintptr) uintptr
	gVariantNewArray                 func(childType uintptr, children *uintptr, n uintptr) uintptr
	gVariantNewTuple                 func(children *uintptr, n uintptr) uintptr
	gVariantTypeNew                  func(typ string) uintptr
	gVariantTypeFree                 func(typ uintptr)
	gVariantGetChildValue            func(v, i uintptr) uintptr
	gVariantGetUint32                func(v uintptr) uint32
	gVariantGetBoolean               func(v uintptr) bool
	gVariantGetString                func(v, length uintptr) string
	gVariantLookupValue              func(dict uintptr, key string, typ uintptr) uintptr
	gVariantUnref                    func(v uintptr)

	// responseCallback is the GDBusSignalCallback of Response signals.
	responseCallback uintptr
)

func loadGIO() error {
	gioOnce.Do(func() {
		lib, err := purego.Dlopen("libgio-2.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			gioErr = fmt.Errorf("%w: GIO: %w", ErrUnsupported, err)
			return
		}
		gioLib = lib
		for name, fn := range map[string]any{
			"g_bus_get_sync":                       &gBusGetSync,
			"g_object_unref":                       &gObjectUnref,
			"g_error_free":                         &gErrorFree,
			"g_dbus_connection_get_unique_name":    &gDBusConnectionGetUniqueName,
			"g_dbus_connection_signal_subscribe":   &gDBusConnectionSignalSubscribe,
			"g_dbus_connection_signal_unsubscribe": &gDBusConnectionSignalUnsubscribe,
			"g_dbus_connection_call_sync":          &gDBusConnectionCallSync,
			"g_main_context_new":                   &gMainContextNew,
			"g_main_context_push_thread_default":   &gMainContextPushThreadDefault,
			"g_main_context_pop_thread_default":    &gMainContextPopThreadDefault,
			"g_main_context_iteration":             &gMainContextIteration,
			"g_main_context_wakeup":                &gMainContextWakeup,
			"g_main_context_unref":                 &gMainContextUnref,
			"g_variant_new_string":                 &gVariantNewString,
			"g_variant_new_boolean":                &gVariantNewBoolean,
			"g_variant_new_variant":                &gVariantNewVariant,
			"g_variant_new_dict_entry":             &gVariantNewDictEntry,
			"g_variant_new_array":                  &gVariantNewArray,
			"g_variant_new_tuple":                  &gVariantNewTuple,
			"g_variant_type_new":                   &gVariantTypeNew,
			"g_variant_type_free":                  &gVariantTypeFree,
			"g_variant_get_child_value":            &gVariantGetChildValue,
			"g_variant_get_uint32":                 &gVariantGetUint32,
			"g_variant_get_boolean":                &gVariantGetBoolean,
			"g_variant_get_string":                 &gVariantGetString,
			"g_variant_lookup_value":               &gVariantLookupValue,
			"g_variant_unref":                      &gVariantUnref,
		} {
			if _, err := purego.Dlsym(lib, name); err != nil {
				gioErr = fmt.Errorf("%w: GIO: %w", ErrUnsupported, err)
				return
			}
			purego.RegisterLibFunc(fn, lib, name)
		}
		responseCallback = purego.NewCallback(onResponse)
	})
	return gioErr
}

// request is a request waiting for its Response.
type request struct {
	path   string
	done   bool
	answer Answer
}

var (
	// requests makes one request at a time, which current is.
	requests sync.Mutex
	current  atomic.Pointer[request]
	// tokens number the requests, whose handle_token names their path.
	tokens atomic.Uint64
)

func requestBackground(ctx context.Context, b Background) (Answer, error) {
	if err := loadGIO(); err != nil {
		return Answer{}, err
	}
	requests.Lock()
	defer requests.Unlock()
	// The main context belongs to this thread while it is its default.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var gerr *gError
	conn := gBusGetSync(gBusTypeSession, 0, &gerr)
	if conn == 0 {
		return Answer{}, fmt.Errorf("portal: the session bus: %w", takeError(gerr))
	}
	defer gObjectUnref(conn)
	mctx := gMainContextNew()
	gMainContextPushThreadDefault(mctx)
	defer func() {
		gMainContextPopThreadDefault(mctx)
		gMainContextUnref(mctx)
	}()

	// The Response may come before the call returns: subscribe first, to
	// the path the portal will give the request.
	token := "mixmusic" + strconv.FormatUint(tokens.Add(1), 10)
	r := &request{path: requestPath(gDBusConnectionGetUniqueName(conn), token)}
	current.Store(r)
	defer current.Store(nil)
	sub := subscribe(conn, r.path)
	defer func() { gDBusConnectionSignalUnsubscribe(conn, sub) }()

	options := []uintptr{
		entry("handle_token", gVariantNewString(token)),
		entry("reason", gVariantNewString(b.Reason)),
		entry("autostart", gVariantNewBoolean(b.Autostart)),
		entry("dbus-activatable", gVariantNewBoolean(false)),
	}
	if len(b.Commandline) > 0 {
		args := make([]uintptr, len(b.Commandline))
		for i, a := range b.Commandline {
			args[i] = gVariantNewString(a)
		}
		options = append(options, entry("commandline", array("s", args)))
	}
	params := tuple(gVariantNewString(""), array("{sv}", options))
	handle, err := call(conn, portalPath, backgroundInterface, "RequestBackground", params, "(o)")
	if err != nil {
		if missing(remoteError(err.Error())) {
			return Answer{}, fmt.Errorf("%w: %w", ErrNoBackground, err)
		}
		return Answer{}, fmt.Errorf("portal: asking to run in the background: %w", err)
	}
	if handle != r.path {
		// Portals before 0.9 choose the path themselves.
		gDBusConnectionSignalUnsubscribe(conn, sub)
		r.path = handle
		sub = subscribe(conn, r.path)
	}

	woke := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		gMainContextWakeup(mctx)
		close(woke)
	})
	defer func() {
		if !stop() {
			<-woke // it wakes mctx: before mctx goes
		}
	}()
	for !r.done && ctx.Err() == nil {
		gMainContextIteration(mctx, true)
	}
	if !r.done {
		// The desktop may still ask the user: take the question back, if
		// the request is still there.
		_, _ = call(conn, r.path, requestInterface, "Close", tuple(), "")
		return Answer{}, fmt.Errorf("portal: the desktop didn't answer: %w", ctx.Err())
	}
	return r.answer, nil
}

// subscribe calls onResponse with the Response of the request at path, on
// the thread's main context.
func subscribe(conn uintptr, path string) uint32 {
	return gDBusConnectionSignalSubscribe(conn, portalName, requestInterface, "Response", path, nil, 0, responseCallback, 0, 0)
}

// onResponse takes the Response of the current request: its code, 0 when
// the request succeeded, and its results, a{sv}.
func onResponse(_ uintptr, _, path, _, _ *byte, params, _ uintptr) {
	r := current.Load()
	if r == nil || r.done || goString(path) != r.path {
		return
	}
	r.done = true
	code := gVariantGetChildValue(params, 0)
	ok := gVariantGetUint32(code) == 0
	gVariantUnref(code)
	results := gVariantGetChildValue(params, 1)
	defer gVariantUnref(results)
	// The results tell what holds even when the request failed.
	r.answer = Answer{Background: lookupBool(results, "background"), Autostart: lookupBool(results, "autostart")}
	if !ok {
		r.answer.Background = false
	}
}

// call calls a method of the portal, and returns the string or object path
// of its reply, when reply is "(s)" or "(o)".
func call(conn uintptr, path, iface, method string, params uintptr, reply string) (string, error) {
	var typ uintptr
	if reply != "" {
		typ = gVariantTypeNew(reply)
		defer gVariantTypeFree(typ)
	}
	var gerr *gError
	v := gDBusConnectionCallSync(conn, portalName, path, iface, method, params, typ, 0, callTimeoutMs, 0, &gerr)
	if v == 0 {
		return "", takeError(gerr)
	}
	defer gVariantUnref(v)
	if reply == "" {
		return "", nil
	}
	s := gVariantGetChildValue(v, 0)
	defer gVariantUnref(s)
	return gVariantGetString(s, 0), nil
}

// lookupBool returns the boolean under key in an a{sv}, false when there
// is none.
func lookupBool(dict uintptr, key string) bool {
	typ := gVariantTypeNew("b")
	defer gVariantTypeFree(typ)
	v := gVariantLookupValue(dict, key, typ)
	if v == 0 {
		return false
	}
	defer gVariantUnref(v)
	return gVariantGetBoolean(v)
}

func entry(key string, value uintptr) uintptr {
	return gVariantNewDictEntry(gVariantNewString(key), gVariantNewVariant(value))
}

func tuple(items ...uintptr) uintptr {
	var p *uintptr
	if len(items) > 0 {
		p = &items[0]
	}
	v := gVariantNewTuple(p, uintptr(len(items)))
	runtime.KeepAlive(items)
	return v
}

// array makes an array of items, of type typ.
func array(typ string, items []uintptr) uintptr {
	t := gVariantTypeNew(typ)
	defer gVariantTypeFree(t)
	var p *uintptr
	if len(items) > 0 {
		p = &items[0]
	}
	v := gVariantNewArray(t, p, uintptr(len(items)))
	runtime.KeepAlive(items)
	return v
}

// takeError returns err as a Go error, and frees it.
func takeError(err *gError) error {
	if err == nil {
		return errors.New("unknown error")
	}
	defer gErrorFree(err)
	return errors.New(goString(err.message))
}

// goString copies a C string.
func goString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}
