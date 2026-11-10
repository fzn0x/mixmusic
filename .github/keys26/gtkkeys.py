#!/usr/bin/env python3
"""Temporary (#407): what GTK 3 makes of the keypad's keys, from its keymap
to the events its window's key-press-event gets, as xdotool presses the
keys given. Removed before the pull request is ready."""
import subprocess
import sys
import time

import gi

gi.require_version("Gdk", "3.0")
gi.require_version("Gtk", "3.0")
from gi.repository import Gdk, Gtk  # noqa: E402

TITLE = "keys26 GTK"


def pump(seconds):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        while Gtk.events_pending():
            Gtk.main_iteration_do(False)
        time.sleep(0.005)


def run(*args):
    p = subprocess.Popen(args)
    while p.poll() is None:
        pump(0.01)
    return p.returncode


def on_key(_widget, e):
    kind = "press  " if e.type == Gdk.EventType.KEY_PRESS else "release"
    print(f"  {kind} keyval=0x{e.keyval:x} {Gdk.keyval_name(e.keyval)} hardware_keycode={e.hardware_keycode}"
          f" state=0x{int(e.state):x} group={e.group} is_modifier={e.is_modifier}", flush=True)
    return False


win = Gtk.Window(title=TITLE)
win.set_default_size(200, 200)
win.connect("key-press-event", on_key)
win.connect("key-release-event", on_key)
win.show_all()
pump(0.5)

keymap = Gdk.Keymap.get_for_display(Gdk.Display.get_default())
print("GTK", Gtk.get_major_version(), Gtk.get_minor_version(), Gtk.get_micro_version(), flush=True)
for name in ("KP_Add", "KP_Subtract", "equal", "plus", "minus", "XF86Next_VMode", "XF86Prev_VMode"):
    keyval = Gdk.keyval_from_name(name)
    ok, entries = keymap.get_entries_for_keyval(keyval)
    where = [(k.keycode, k.group, k.level) for k in entries] if ok else "nowhere"
    print(f"{name} (0x{keyval:x}) at (keycode, group, level): {where}", flush=True)
for name in ("KP_Add", "KP_Subtract"):
    ok, entries = keymap.get_entries_for_keyval(Gdk.keyval_from_name(name))
    if not ok:
        continue
    code = entries[0].keycode
    ok, keys, keyvals = keymap.get_entries_for_keycode(code)
    print(f"keycode {code}: {[(k.group, k.level, Gdk.keyval_name(v)) for k, v in zip(keys, keyvals)]}", flush=True)
    for mods, label in ((0, "no modifier"), (Gdk.ModifierType.CONTROL_MASK, "Ctrl"),
                        (Gdk.ModifierType.CONTROL_MASK | Gdk.ModifierType.SHIFT_MASK, "Ctrl+Shift"),
                        (Gdk.ModifierType.CONTROL_MASK | Gdk.ModifierType.MOD1_MASK, "Ctrl+Alt")):
        r = keymap.translate_keyboard_state(code, Gdk.ModifierType(mods), 0)
        print(f"  {label}: translated={r[0]} keyval={Gdk.keyval_name(r[1])} group={r[2]} level={r[3]}"
              f" consumed=0x{int(r[4]):x}", flush=True)

run("xdotool", "search", "--sync", "--onlyvisible", "--name", f"^{TITLE}$", "windowfocus", "--sync")
pump(0.3)
for keys in sys.argv[1:]:
    print(f"--- xdotool key {keys}", flush=True)
    run("xdotool", "key", keys)
    pump(0.5)
