//go:build linux && (amd64 || arm64)

package nowplaying

// Linux has an MPRIS player on the session bus while something shows, which
// desktops show with their media controls and pass their media keys to.
// GIO's GDBus serves it, called with purego as MyGo calls GIO: the player's
// object calls back on a thread of its own, which runs a GLib main context.

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	busName         = "org.mpris.MediaPlayer2.mixmusic"
	objectPath      = "/org/mpris/MediaPlayer2"
	rootInterface   = "org.mpris.MediaPlayer2"
	playerInterface = "org.mpris.MediaPlayer2.Player"
	// noTrack is the track ID of no track.
	noTrack = "/org/mpris/MediaPlayer2/TrackList/NoTrack"
	// trackPrefix starts the track IDs of tracks, which go on with the hex
	// of the track's ID, as object paths allow only [A-Za-z0-9_].
	trackPrefix = "/org/mixmusic/track/"

	gBusTypeSession = 2
	// dbusNameFlagDoNotQueue fails to own a name taken, rather than wait.
	dbusNameFlagDoNotQueue = 4
	// The replies to RequestName that own the name.
	dbusRequestNamePrimaryOwner = 1
	dbusRequestNameAlreadyOwner = 4
	callTimeoutMs               = 5000
)

// mprisXML is what the player's object has: the two interfaces of MPRIS
// 2.2, without the optional LoopStatus, Shuffle and Fullscreen.
const mprisXML = `<node>
  <interface name="org.mpris.MediaPlayer2">
    <method name="Raise"/>
    <method name="Quit"/>
    <property name="CanQuit" type="b" access="read"/>
    <property name="CanRaise" type="b" access="read"/>
    <property name="HasTrackList" type="b" access="read"/>
    <property name="Identity" type="s" access="read"/>
    <property name="DesktopEntry" type="s" access="read"/>
    <property name="SupportedUriSchemes" type="as" access="read"/>
    <property name="SupportedMimeTypes" type="as" access="read"/>
  </interface>
  <interface name="org.mpris.MediaPlayer2.Player">
    <method name="Next"/>
    <method name="Previous"/>
    <method name="Pause"/>
    <method name="PlayPause"/>
    <method name="Stop"/>
    <method name="Play"/>
    <method name="Seek"><arg direction="in" name="Offset" type="x"/></method>
    <method name="SetPosition"><arg direction="in" name="TrackId" type="o"/><arg direction="in" name="Position" type="x"/></method>
    <method name="OpenUri"><arg direction="in" name="Uri" type="s"/></method>
    <signal name="Seeked"><arg name="Position" type="x"/></signal>
    <property name="PlaybackStatus" type="s" access="read"/>
    <property name="Rate" type="d" access="readwrite"/>
    <property name="Metadata" type="a{sv}" access="read"/>
    <property name="Volume" type="d" access="readwrite"/>
    <property name="Position" type="x" access="read"/>
    <property name="MinimumRate" type="d" access="read"/>
    <property name="MaximumRate" type="d" access="read"/>
    <property name="CanGoNext" type="b" access="read"/>
    <property name="CanGoPrevious" type="b" access="read"/>
    <property name="CanPlay" type="b" access="read"/>
    <property name="CanPause" type="b" access="read"/>
    <property name="CanSeek" type="b" access="read"/>
    <property name="CanControl" type="b" access="read"/>
  </interface>
</node>`

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

	gBusGetSync                          func(busType int32, cancellable uintptr, err **gError) uintptr
	gObjectUnref                         func(obj uintptr)
	gErrorFree                           func(err *gError)
	gDBusNodeInfoNewForXML               func(xml string, err **gError) uintptr
	gDBusNodeInfoLookupInterface         func(info uintptr, name string) uintptr
	gDBusConnectionRegisterObject        func(conn uintptr, path string, iface uintptr, vtable *uintptr, user, free uintptr, err **gError) uint32
	gDBusConnectionUnregisterObject      func(conn uintptr, id uint32) bool
	gDBusConnectionEmitSignal            func(conn uintptr, dest *byte, path, iface, signal string, params uintptr, err **gError) bool
	gDBusConnectionCallSync              func(conn uintptr, name, path, iface, method string, params, replyType uintptr, flags, timeoutMs int32, cancellable uintptr, err **gError) uintptr
	gDBusMethodInvocationReturnValue     func(inv, params uintptr)
	gDBusMethodInvocationReturnDBusError func(inv uintptr, name, message string)
	gMainContextNew                      func() uintptr
	gMainContextPushThreadDefault        func(ctx uintptr)
	gMainContextPopThreadDefault         func(ctx uintptr)
	gMainContextIteration                func(ctx uintptr, mayBlock bool) bool
	gMainContextWakeup                   func(ctx uintptr)
	gMainContextUnref                    func(ctx uintptr)
	gVariantNewString                    func(s string) uintptr
	gVariantNewObjectPath                func(s string) uintptr
	gVariantNewInt64                     func(v int64) uintptr
	gVariantNewUint32                    func(v uint32) uintptr
	gVariantNewDouble                    func(v float64) uintptr
	gVariantNewBoolean                   func(v bool) uintptr
	gVariantNewVariant                   func(v uintptr) uintptr
	gVariantNewDictEntry                 func(key, value uintptr) uintptr
	gVariantNewArray                     func(childType uintptr, children *uintptr, n uintptr) uintptr
	gVariantNewTuple                     func(children *uintptr, n uintptr) uintptr
	gVariantTypeNew                      func(typ string) uintptr
	gVariantTypeFree                     func(typ uintptr)
	gVariantGetChildValue                func(v, i uintptr) uintptr
	gVariantGetInt64                     func(v uintptr) int64
	gVariantGetUint32                    func(v uintptr) uint32
	gVariantGetDouble                    func(v uintptr) float64
	gVariantGetString                    func(v, length uintptr) string
	gVariantUnref                        func(v uintptr)

	// mprisNode is mprisXML parsed; mprisVTable the GDBusInterfaceVTable
	// of both interfaces: the callbacks, and padding.
	mprisNode   uintptr
	mprisVTable [11]uintptr
	// mprisCurrent is the controls the callbacks serve.
	mprisCurrent atomic.Pointer[linuxControls]
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
			"g_bus_get_sync":                             &gBusGetSync,
			"g_object_unref":                             &gObjectUnref,
			"g_error_free":                               &gErrorFree,
			"g_dbus_node_info_new_for_xml":               &gDBusNodeInfoNewForXML,
			"g_dbus_node_info_lookup_interface":          &gDBusNodeInfoLookupInterface,
			"g_dbus_connection_register_object":          &gDBusConnectionRegisterObject,
			"g_dbus_connection_unregister_object":        &gDBusConnectionUnregisterObject,
			"g_dbus_connection_emit_signal":              &gDBusConnectionEmitSignal,
			"g_dbus_connection_call_sync":                &gDBusConnectionCallSync,
			"g_dbus_method_invocation_return_value":      &gDBusMethodInvocationReturnValue,
			"g_dbus_method_invocation_return_dbus_error": &gDBusMethodInvocationReturnDBusError,
			"g_main_context_new":                         &gMainContextNew,
			"g_main_context_push_thread_default":         &gMainContextPushThreadDefault,
			"g_main_context_pop_thread_default":          &gMainContextPopThreadDefault,
			"g_main_context_iteration":                   &gMainContextIteration,
			"g_main_context_wakeup":                      &gMainContextWakeup,
			"g_main_context_unref":                       &gMainContextUnref,
			"g_variant_new_string":                       &gVariantNewString,
			"g_variant_new_object_path":                  &gVariantNewObjectPath,
			"g_variant_new_int64":                        &gVariantNewInt64,
			"g_variant_new_uint32":                       &gVariantNewUint32,
			"g_variant_new_double":                       &gVariantNewDouble,
			"g_variant_new_boolean":                      &gVariantNewBoolean,
			"g_variant_new_variant":                      &gVariantNewVariant,
			"g_variant_new_dict_entry":                   &gVariantNewDictEntry,
			"g_variant_new_array":                        &gVariantNewArray,
			"g_variant_new_tuple":                        &gVariantNewTuple,
			"g_variant_type_new":                         &gVariantTypeNew,
			"g_variant_type_free":                        &gVariantTypeFree,
			"g_variant_get_child_value":                  &gVariantGetChildValue,
			"g_variant_get_int64":                        &gVariantGetInt64,
			"g_variant_get_uint32":                       &gVariantGetUint32,
			"g_variant_get_double":                       &gVariantGetDouble,
			"g_variant_get_string":                       &gVariantGetString,
			"g_variant_unref":                            &gVariantUnref,
		} {
			if _, err := purego.Dlsym(lib, name); err != nil {
				gioErr = fmt.Errorf("%w: GIO: %w", ErrUnsupported, err)
				return
			}
			purego.RegisterLibFunc(fn, lib, name)
		}
		var gerr *gError
		if mprisNode = gDBusNodeInfoNewForXML(mprisXML, &gerr); mprisNode == 0 {
			gioErr = fmt.Errorf("nowplaying: the MPRIS interfaces: %w", takeError(gerr))
			return
		}
		mprisVTable[0] = purego.NewCallback(mprisMethod)
		mprisVTable[1] = purego.NewCallback(mprisGet)
		mprisVTable[2] = purego.NewCallback(mprisSet)
	})
	return gioErr
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

// linuxControls is the MPRIS player.
type linuxControls struct {
	ask  func(Request)
	conn uintptr
	ctx  uintptr
	ids  [2]uint32
	quit atomic.Bool
	done chan struct{}

	mu   sync.Mutex
	info *Info
	// base is the Info clients go on from: the one when the player came,
	// or that changed what they show last.
	base *Info
	// name is the bus name owned while something shows.
	name   string
	warned bool
}

func open(ask func(Request), _ func(func())) (platform, error) {
	if err := loadGIO(); err != nil {
		return nil, err
	}
	if mprisCurrent.Load() != nil {
		return nil, errors.New("nowplaying: the MPRIS player is open already")
	}
	l := &linuxControls{ask: ask, done: make(chan struct{})}
	ready := make(chan error, 1)
	go l.serve(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return l, nil
}

// serve registers the player's object and runs the main context it calls
// back on, until close.
func (l *linuxControls) serve(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(l.done)
	var gerr *gError
	if l.conn = gBusGetSync(gBusTypeSession, 0, &gerr); l.conn == 0 {
		ready <- fmt.Errorf("nowplaying: the session bus: %w", takeError(gerr))
		return
	}
	defer gObjectUnref(l.conn)
	l.ctx = gMainContextNew()
	gMainContextPushThreadDefault(l.ctx)
	defer func() {
		gMainContextPopThreadDefault(l.ctx)
		gMainContextUnref(l.ctx)
	}()
	mprisCurrent.Store(l)
	defer mprisCurrent.CompareAndSwap(l, nil)
	for i, name := range []string{rootInterface, playerInterface} {
		iface := gDBusNodeInfoLookupInterface(mprisNode, name)
		l.ids[i] = gDBusConnectionRegisterObject(l.conn, objectPath, iface, &mprisVTable[0], 0, 0, &gerr)
		if l.ids[i] == 0 {
			err := takeError(gerr)
			for _, id := range l.ids[:i] {
				gDBusConnectionUnregisterObject(l.conn, id)
			}
			ready <- fmt.Errorf("nowplaying: the MPRIS player: %w", err)
			return
		}
	}
	ready <- nil
	for !l.quit.Load() {
		gMainContextIteration(l.ctx, true)
	}
	for _, id := range l.ids {
		gDBusConnectionUnregisterObject(l.conn, id)
	}
}

func (l *linuxControls) close() {
	l.quit.Store(true)
	gMainContextWakeup(l.ctx)
	<-l.done
}

func (l *linuxControls) show(next *Info) {
	l.mu.Lock()
	l.info = next
	l.mu.Unlock()
	c := diff(l.base, next)
	switch {
	case next == nil:
		l.base = nil
		l.release()
		return
	case l.base == nil || (l.name == "" && c.track):
		// Clients read every property of a player that comes.
		l.base = next
		l.own()
		return
	case !c.any():
		return
	}
	l.base = next
	var props []string
	if c.track {
		props = append(props, "Metadata", "CanSeek")
	}
	if c.track || c.playing {
		props = append(props, "PlaybackStatus")
	}
	if c.volume {
		props = append(props, "Volume")
	}
	if c.rate {
		props = append(props, "Rate")
	}
	if len(props) > 0 {
		entries := make([]uintptr, len(props))
		for i, p := range props {
			entries[i] = entry(p, l.property(p))
		}
		l.emit("org.freedesktop.DBus.Properties", "PropertiesChanged",
			tuple(gVariantNewString(playerInterface), array("{sv}", entries), array("s", nil)))
	}
	// Clients move the position on at the rate: at another, they go on
	// from where it is now.
	if c.seeked || c.rate {
		l.emit(playerInterface, "Seeked", tuple(gVariantNewInt64(micro(next.position(time.Now())))))
	}
}

// own takes the player's bus name, or one of this process when another
// player has it, which brings the player to the desktop's controls.
func (l *linuxControls) own() {
	if l.name != "" {
		return
	}
	for _, name := range []string{busName, busName + ".instance" + strconv.Itoa(os.Getpid())} {
		reply, err := l.callBus("RequestName", tuple(gVariantNewString(name), gVariantNewUint32(dbusNameFlagDoNotQueue)))
		if err != nil {
			l.warn(err)
			return
		}
		if reply == dbusRequestNamePrimaryOwner || reply == dbusRequestNameAlreadyOwner {
			l.name = name
			return
		}
	}
	l.warn(errors.New("its bus names are taken"))
}

// release lets go of the bus name, which takes the player out of the
// desktop's controls.
func (l *linuxControls) release() {
	if l.name == "" {
		return
	}
	if _, err := l.callBus("ReleaseName", tuple(gVariantNewString(l.name))); err != nil {
		l.warn(err)
	}
	l.name = ""
}

// callBus calls a method of the bus that replies a number.
func (l *linuxControls) callBus(method string, params uintptr) (uint32, error) {
	typ := gVariantTypeNew("(u)")
	defer gVariantTypeFree(typ)
	var gerr *gError
	reply := gDBusConnectionCallSync(l.conn, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
		method, params, typ, 0, callTimeoutMs, 0, &gerr)
	if reply == 0 {
		return 0, fmt.Errorf("%s: %w", method, takeError(gerr))
	}
	defer gVariantUnref(reply)
	v := gVariantGetChildValue(reply, 0)
	defer gVariantUnref(v)
	return gVariantGetUint32(v), nil
}

func (l *linuxControls) emit(iface, signal string, params uintptr) {
	var gerr *gError
	if !gDBusConnectionEmitSignal(l.conn, nil, objectPath, iface, signal, params, &gerr) {
		l.warn(fmt.Errorf("%s: %w", signal, takeError(gerr)))
	}
}

// warn logs the first thing that goes wrong.
func (l *linuxControls) warn(err error) {
	if !l.warned {
		l.warned = true
		log.Printf("mixmusic: the desktop's media controls: %v", err)
	}
}

// property returns the value of one of the player's properties, whose
// names the two interfaces don't share.
func (l *linuxControls) property(name string) uintptr {
	l.mu.Lock()
	info := l.info
	l.mu.Unlock()
	switch name {
	case "CanQuit", "HasTrackList":
		return gVariantNewBoolean(false)
	case "CanRaise", "CanGoNext", "CanGoPrevious", "CanControl":
		return gVariantNewBoolean(true)
	case "Identity":
		return gVariantNewString("MixMusic")
	case "DesktopEntry":
		return gVariantNewString("mixmusic")
	case "SupportedUriSchemes", "SupportedMimeTypes":
		return array("s", nil)
	case "PlaybackStatus":
		switch {
		case info == nil:
			return gVariantNewString("Stopped")
		case info.Playing:
			return gVariantNewString("Playing")
		}
		return gVariantNewString("Paused")
	case "Rate":
		if info == nil {
			return gVariantNewDouble(1)
		}
		return gVariantNewDouble(info.rate())
	case "MinimumRate":
		return gVariantNewDouble(MinRate)
	case "MaximumRate":
		return gVariantNewDouble(MaxRate)
	case "Metadata":
		return metadata(info)
	case "Volume":
		if info == nil {
			return gVariantNewDouble(1)
		}
		return gVariantNewDouble(info.Volume)
	case "Position":
		if info == nil {
			return gVariantNewInt64(0)
		}
		return gVariantNewInt64(micro(info.position(time.Now())))
	case "CanPlay", "CanPause":
		return gVariantNewBoolean(info != nil)
	case "CanSeek":
		return gVariantNewBoolean(info != nil && info.Duration > 0)
	}
	// GDBus asks only for the properties of mprisXML. A NULL without an
	// error would abort it.
	return gVariantNewBoolean(false)
}

// method does what a method call asks, and returns the name of the D-Bus
// error to reply, if any.
func (l *linuxControls) method(name string, params uintptr) string {
	l.mu.Lock()
	info := l.info
	l.mu.Unlock()
	switch name {
	case "Raise":
		l.ask(Request{Command: Raise})
	case "Quit":
		// CanQuit is false.
	case "Next":
		l.ask(Request{Command: Next})
	case "Previous":
		l.ask(Request{Command: Previous})
	case "Pause", "Stop":
		l.ask(Request{Command: Pause})
	case "PlayPause":
		l.ask(Request{Command: Toggle})
	case "Play":
		l.ask(Request{Command: Play})
	case "Seek":
		if info == nil || info.Duration <= 0 {
			break
		}
		p := info.position(time.Now()) + time.Duration(int64Arg(params, 0))*time.Microsecond
		if p > info.Duration {
			// As MPRIS has a seek past the end do.
			l.ask(Request{Command: Next})
			break
		}
		l.ask(Request{Command: Seek, Position: max(p, 0)})
	case "SetPosition":
		if info == nil || info.Duration <= 0 || stringArg(params, 0) != trackID(info.ID) {
			break // the track changed meanwhile
		}
		p := time.Duration(int64Arg(params, 1)) * time.Microsecond
		if p >= 0 && p <= info.Duration {
			l.ask(Request{Command: Seek, Position: p})
		}
	case "OpenUri":
		return "org.freedesktop.DBus.Error.NotSupported"
	}
	return ""
}

func mprisMethod(_ uintptr, _, _, _, method *byte, params, inv, _ uintptr) {
	if l := mprisCurrent.Load(); l != nil {
		if name := l.method(goString(method), params); name != "" {
			gDBusMethodInvocationReturnDBusError(inv, name, "MixMusic doesn't open addresses")
			return
		}
	}
	gDBusMethodInvocationReturnValue(inv, 0)
}

func mprisGet(_ uintptr, _, _, _, prop *byte, _, _ uintptr) uintptr {
	l := mprisCurrent.Load()
	if l == nil {
		l = &linuxControls{}
	}
	return l.property(goString(prop))
}

func mprisSet(_ uintptr, _, _, _, prop *byte, value, _, _ uintptr) bool {
	l := mprisCurrent.Load()
	if l == nil {
		return true
	}
	switch goString(prop) {
	case "Volume":
		l.ask(Request{Command: SetVolume, Volume: min(max(gVariantGetDouble(value), 0), 1)})
	case "Rate":
		// A rate of 0 pauses, as MPRIS has it.
		if v := gVariantGetDouble(value); v > 0 {
			l.ask(Request{Command: SetRate, Rate: min(max(v, MinRate), MaxRate)})
		} else if v == 0 {
			l.ask(Request{Command: Pause})
		}
	}
	return true
}

// metadata is the Metadata of info.
func metadata(info *Info) uintptr {
	if info == nil {
		return array("{sv}", []uintptr{entry("mpris:trackid", gVariantNewObjectPath(noTrack))})
	}
	entries := []uintptr{
		entry("mpris:trackid", gVariantNewObjectPath(trackID(info.ID))),
		entry("xesam:title", gVariantNewString(info.Title)),
		entry("xesam:artist", stringArray(info.Artists)),
	}
	if info.Duration > 0 {
		entries = append(entries, entry("mpris:length", gVariantNewInt64(micro(info.Duration))))
	}
	if info.Album != "" {
		entries = append(entries, entry("xesam:album", gVariantNewString(info.Album)))
	}
	if info.Artwork != "" {
		entries = append(entries, entry("mpris:artUrl", gVariantNewString(info.Artwork)))
	}
	return array("{sv}", entries)
}

// trackID is the MPRIS track ID of a track's ID.
func trackID(id string) string {
	if id == "" {
		return trackPrefix + "0"
	}
	return trackPrefix + hex.EncodeToString([]byte(id))
}

func micro(d time.Duration) int64 { return int64(d / time.Microsecond) }

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

func stringArray(ss []string) uintptr {
	items := make([]uintptr, len(ss))
	for i, s := range ss {
		items[i] = gVariantNewString(s)
	}
	return array("s", items)
}

func int64Arg(params uintptr, i int) int64 {
	v := gVariantGetChildValue(params, uintptr(i))
	defer gVariantUnref(v)
	return gVariantGetInt64(v)
}

func stringArg(params uintptr, i int) string {
	v := gVariantGetChildValue(params, uintptr(i))
	defer gVariantUnref(v)
	return gVariantGetString(v, 0)
}
