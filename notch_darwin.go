//go:build darwin

package main

// AppKit's part of the player around the notch (#46), through the
// Objective-C runtime with purego, on the main thread: the camera housing
// of the built-in screen, the window over the menu bar, and the keyboard,
// which the open panel takes and gives back without activating MixMusic.

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

const (
	// The window's style alone: borderless, as MyGo's Frameless keeps a
	// titled style, which AppKit holds below the menu bar.
	nsNonactivatingPanel = 1 << 7
	// NSStatusWindowLevel, over the menu bar, with its items.
	nsStatusWindowLevel = 25
	// NSWindowAnimationBehaviorNone: the view animates the panel.
	nsAnimationNone = 2
	// An NSTrackingArea's options: entered and exited, moved, always, and
	// in the visible rect. MyGo's own is only while the app is active,
	// which the panel never makes MixMusic: without this one, the pointer
	// over the panel in front of another app would tell nothing.
	trackingAlways = 0x01 | 0x02 | 0x80 | 0x200
	// NSEvent's masks of the left, right and other mouse buttons' presses,
	// and of their releases.
	maskPresses  = 1<<1 | 1<<3 | 1<<25
	maskReleases = 1<<2 | 1<<4 | 1<<26
)

type nsPoint struct{ X, Y float64 }
type nsSize struct{ Width, Height float64 }
type nsRect struct {
	Origin nsPoint
	Size   nsSize
}

func sel(name string) objc.SEL  { return objc.RegisterName(name) }
func class(name string) objc.ID { return objc.ID(objc.GetClass(name)) }

// withPool runs fn in an autorelease pool.
func withPool(fn func()) {
	pool := class("NSAutoreleasePool").Send(sel("new"))
	defer pool.Send(sel("drain"))
	fn()
}

// findHousing finds the camera housing of the built-in screen, once the
// app is ready: tests, which don't run it, find none.
func findHousing() (housing, bool) {
	if !mygo.App.IsReady() {
		return housing{}, false
	}
	var screens []notchScreen
	mygo.RunOnMain(func() { withPool(func() { screens = notchScreens() }) })
	return housingOf(mygo.Screen.Displays(), screens)
}

// notchScreens tells the tops of the screens that know of a camera
// housing, as AppKit sees them, from macOS 12. Main thread.
func notchScreens() []notchScreen {
	var out []notchScreen
	screens := class("NSScreen").Send(sel("screens"))
	key := class("NSString").Send(sel("stringWithUTF8String:"), "NSScreenNumber")
	for i := range objc.Send[uint](screens, sel("count")) {
		s := screens.Send(sel("objectAtIndex:"), i)
		if !objc.Send[bool](s, sel("respondsToSelector:"), sel("auxiliaryTopLeftArea")) {
			continue
		}
		num := s.Send(sel("deviceDescription")).Send(sel("objectForKey:"), key)
		left := objc.Send[nsRect](s, sel("auxiliaryTopLeftArea"))
		right := objc.Send[nsRect](s, sel("auxiliaryTopRightArea"))
		out = append(out, notchScreen{
			id:     int64(objc.Send[uint32](num, sel("unsignedIntValue"))),
			left:   left.Size.Width,
			right:  right.Size.Width,
			height: max(left.Size.Height, right.Size.Height),
			// In whatever coordinates AppKit gives them, both alike.
			gap: right.Origin.X - (left.Origin.X + left.Size.Width),
		})
	}
	return out
}

// macNotch is AppKit's part of the window around the notch. Its fields
// belong to the main thread.
type macNotch struct {
	ns     objc.ID // the NSWindow
	keys   keyHandBack
	local  objc.ID // the monitor of the presses MixMusic gets
	global objc.ID // and of those other apps and the menu bar get
	blocks []objc.Block
	closed bool
}

// newNotchHost puts the window over the menu bar, on every Space, and
// follows the presses, for the keyboard. Main thread.
func newNotchHost(w *mygo.Window) notchHost {
	m := &macNotch{ns: objc.ID(w.NativeHandle())}
	if m.ns == 0 {
		return m
	}
	withPool(func() {
		m.ns.Send(sel("setStyleMask:"), uint(nsNonactivatingPanel))
		m.ns.Send(sel("setLevel:"), nsStatusWindowLevel)
		m.ns.Send(sel("setCanHide:"), false)
		m.ns.Send(sel("setAnimationBehavior:"), nsAnimationNone)
		// Compact, a click leaves the keyboard in the app the user is in.
		m.ns.Send(sel("setBecomesKeyOnlyIfNeeded:"), true)
		m.track()
		m.watch()
	})
	return m
}

// track has the view MyGo draws in hear the pointer whichever app is
// active.
func (m *macNotch) track() {
	view := surfaceView(m.ns.Send(sel("contentView")))
	if view == 0 {
		return
	}
	area := class("NSTrackingArea").Send(sel("alloc")).
		Send(sel("initWithRect:options:owner:userInfo:"), nsRect{}, uint(trackingAlways), view, objc.ID(0))
	view.Send(sel("addTrackingArea:"), area)
	// The view keeps it, and lets it go with itself.
	area.Send(sel("release"))
}

// surfaceView finds the view MyGo draws the window's content in: v, or a
// view inside it.
func surfaceView(v objc.ID) objc.ID {
	if v == 0 {
		return 0
	}
	if objc.Send[bool](v, sel("isKindOfClass:"), class("MyGoSurfaceView")) {
		return v
	}
	subviews := v.Send(sel("subviews"))
	for i := range objc.Send[uint](subviews, sel("count")) {
		if s := surfaceView(subviews.Send(sel("objectAtIndex:"), i)); s != 0 {
			return s
		}
	}
	return 0
}

// watch counts the presses the window gets, and gives the keyboard back
// on a press outside it, which goes on unchanged to what it was on.
func (m *macNotch) watch() {
	local := objc.NewBlock(func(_ objc.Block, ev objc.ID) objc.ID {
		m.pressed(ev)
		return ev
	})
	global := objc.NewBlock(func(_ objc.Block, ev objc.ID) {
		if !m.closed {
			// The press went to another app, or the menu bar: none of the
			// window's is down.
			m.keys.lost()
			m.handBack()
		}
	})
	m.blocks = []objc.Block{local, global}
	events := class("NSEvent")
	m.local = events.Send(sel("addLocalMonitorForEventsMatchingMask:handler:"), uint64(maskPresses|maskReleases), local).Send(sel("retain"))
	m.global = events.Send(sel("addGlobalMonitorForEventsMatchingMask:handler:"), uint64(maskPresses), global).Send(sel("retain"))
}

// pressed counts a press or a release MixMusic gets, before its window
// does.
func (m *macNotch) pressed(ev objc.ID) {
	if m.closed {
		return
	}
	mine := ev.Send(sel("window")) == m.ns
	switch objc.Send[uint](ev, sel("type")) {
	case 1, 3, 25: // NSEventTypeLeftMouseDown, RightMouseDown, OtherMouseDown
		if mine {
			m.keys.press()
		} else {
			// Another window of MixMusic's takes the keyboard itself.
			m.keys.lost()
		}
	case 2, 4, 26: // and their ups
		if mine && m.keys.release() {
			// Once the window has had the release, and MyGo the click.
			mygo.App.WhenReady(m.handBack)
		}
	}
}

// keyboard gives the window the keyboard as the panel opens, or gives it
// back as it closes, after the frame.
func (m *macNotch) keyboard(take bool) {
	mygo.App.WhenReady(func() {
		if m.closed || m.ns == 0 {
			return
		}
		if take {
			m.keys.keep()
			m.ns.Send(sel("setBecomesKeyOnlyIfNeeded:"), false)
			m.ns.Send(sel("makeKeyWindow"))
			return
		}
		m.ns.Send(sel("setBecomesKeyOnlyIfNeeded:"), true)
		if m.keys.ask() {
			m.handBack()
		}
	})
}

// handBack gives the keyboard back to the app the user was in, while the
// window has it. resignKeyWindow would do nothing, being AppKit's
// notification, not a request: the window goes out and straight back in,
// which leaves the key window to that app, without activating MixMusic.
func (m *macNotch) handBack() {
	if m.closed || m.ns == 0 || !objc.Send[bool](m.ns, sel("isKeyWindow")) {
		return
	}
	m.ns.Send(sel("orderOut:"), objc.ID(0))
	m.ns.Send(sel("orderFrontRegardless"))
}

// close removes the monitors, as the window closes. Main thread.
func (m *macNotch) close() {
	if m.closed {
		return
	}
	m.closed = true
	events := class("NSEvent")
	for _, mon := range []objc.ID{m.local, m.global} {
		if mon != 0 {
			events.Send(sel("removeMonitor:"), mon)
			mon.Send(sel("release"))
		}
	}
	for _, b := range m.blocks {
		b.Release()
	}
	m.local, m.global, m.blocks = 0, 0, nil
}
