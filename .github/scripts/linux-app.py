#!/usr/bin/env python3
# Starts MixMusic as a Linux desktop starts it, and checks what Linux alone
# has, from outside, as a desktop sees it (#146):
#
#   linux-app.py [--libs nix|system] <mixmusic> <out>
#
# It runs under a virtual X display and a session bus of its own:
#
#   xvfb-run -a -s "-screen 0 1600x1000x24" dbus-run-session -- \
#       /usr/bin/python3 .github/scripts/linux-app.py result/bin/mixmusic out
#
# with xdotool, ffmpeg, PulseAudio's pactl and parec, xmessage, and
# PyGObject's Gio. A sound server should run: the tone's checks fail
# without one.
#
# MixMusic gets a profile of its own (MIXMUSIC_DATA_DIR) that holds one
# track, downloaded: a 440 Hz tone that ffmpeg makes, which plays without
# the network, with MixMusic's icon from GitHub as its cover. MixMusic
# starts twice:
#
#   - in capture mode, which saves its own frames of home, Settings › About
#     and Now playing, playing the track into no device; then
#   - as a user starts it, while this script presses keys with xdotool,
#     edits settings.json, plays a stand-in for a desktop's tray
#     (a StatusNotifierWatcher), calls the MPRIS player, reads frames of the
#     display with ffmpeg and listens to the sound server.
#
# Each check is a line of <out>/checks.json and of the job's summary, with
# what it saw; the frames, the logs and the D-Bus answers are in <out>. It
# exits 1 if a check failed. --libs says where GTK should come from: the
# Nix store, or the system's own folders.

import argparse
import array
import collections
import json
import math
import os
import shutil
import signal
import struct
import subprocess
import sys
import threading
import time
import urllib.request
import zlib

import gi

gi.require_version("Gio", "2.0")
from gi.repository import Gio, GLib  # noqa: E402

VIDEO = "mixmusicCI1"  # the downloaded track's video ID: 11 characters
TITLE = "Check tone"
ARTIST = "MixMusic CI"
TONE_HZ = 440
# The track's cover: the system's controls show covers of https addresses
# alone.
COVER = "https://raw.githubusercontent.com/fzn0x/mixmusic/main/resources/icon.png"
MPRIS = "org.mpris.MediaPlayer2.mixmusic"  # outside a Flatpak
MPRIS_PATH = "/org/mpris/MediaPlayer2"
PLAYER = "org.mpris.MediaPlayer2.Player"
WATCHER = "org.kde.StatusNotifierWatcher"
WATCHER_PATH = "/StatusNotifierWatcher"
ITEM = "org.kde.StatusNotifierItem"
MENU = "com.canonical.dbusmenu"
# The tray menu's items, as MixMusic makes them (internal/app/desktop.go).
TRAY_ITEMS = ["Play / Pause", "Next", "Previous", "Show MixMusic", "Mini player"]

WATCHER_XML = """<node>
  <interface name="org.kde.StatusNotifierWatcher">
    <method name="RegisterStatusNotifierItem"><arg name="service" type="s" direction="in"/></method>
    <method name="RegisterStatusNotifierHost"><arg name="service" type="s" direction="in"/></method>
    <property name="RegisteredStatusNotifierItems" type="as" access="read"/>
    <property name="IsStatusNotifierHostRegistered" type="b" access="read"/>
    <property name="ProtocolVersion" type="i" access="read"/>
    <signal name="StatusNotifierItemRegistered"><arg type="s"/></signal>
    <signal name="StatusNotifierItemUnregistered"><arg type="s"/></signal>
    <signal name="StatusNotifierHostRegistered"/>
  </interface>
</node>"""


def watcher(out):
    """Serves org.kde.StatusNotifierWatcher, as a desktop's panel does, and
    writes the items that register to out: the stand-in for a tray."""
    bus = Gio.bus_get_sync(Gio.BusType.SESSION, None)
    items = []

    def method(conn, sender, path, iface, name, params, inv):
        if name == "RegisterStatusNotifierItem":
            service = params.unpack()[0]
            # A path names an object of the sender; a bus name, its item at
            # /StatusNotifierItem.
            if service.startswith("/"):
                item = {"bus": sender, "path": service}
            else:
                item = {"bus": service, "path": "/StatusNotifierItem"}
            items.append(item)
            with open(out + ".tmp", "w") as f:
                json.dump(items, f)
            os.replace(out + ".tmp", out)
            conn.emit_signal(None, WATCHER_PATH, WATCHER, "StatusNotifierItemRegistered",
                             GLib.Variant("(s)", (item["bus"] + item["path"],)))
        inv.return_value(None)

    def get(conn, sender, path, iface, name):
        return {
            "RegisteredStatusNotifierItems": GLib.Variant("as", [i["bus"] + i["path"] for i in items]),
            "IsStatusNotifierHostRegistered": GLib.Variant("b", True),
            "ProtocolVersion": GLib.Variant("i", 0),
        }[name]

    info = Gio.DBusNodeInfo.new_for_xml(WATCHER_XML).interfaces[0]
    bus.register_object(WATCHER_PATH, info, method, get, None)
    bus.call_sync("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "RequestName",
                  GLib.Variant("(su)", (WATCHER, 4)), None, Gio.DBusCallFlags.NONE, 5000, None)
    GLib.MainLoop().run()


class Checks:
    """The checks made, and what each saw."""

    def __init__(self, out):
        self.out, self.list = out, []

    def add(self, name, ok, detail=""):
        self.list.append({"check": name, "ok": bool(ok), "saw": detail})
        print(("ok    " if ok else "FAIL  ") + name + (": " + detail if detail else ""), flush=True)
        return bool(ok)

    def save(self, title):
        with open(os.path.join(self.out, "checks.json"), "w") as f:
            json.dump(self.list, f, indent=2)
        lines = [f"### {title}", "", "| | Check | Saw |", "|---|---|---|"]
        for c in self.list:
            saw = c["saw"].replace("|", "\\|").replace("\n", " ")
            lines.append(f"| {'ok' if c['ok'] else '**failed**'} | {c['check']} | {saw} |")
        text = "\n".join(lines) + "\n\n"
        with open(os.path.join(self.out, "checks.md"), "w") as f:
            f.write(text)
        if os.environ.get("GITHUB_STEP_SUMMARY"):
            with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as f:
                f.write(text)
        return all(c["ok"] for c in self.list)


def until(fn, timeout, every=0.25):
    """Calls fn until it returns something true, for at most timeout
    seconds: it returns that, or the last thing fn returned."""
    end = time.monotonic() + timeout
    while True:
        v = fn()
        if v or time.monotonic() >= end:
            return v
        time.sleep(every)


def run(*args, timeout=30):
    return subprocess.run(args, capture_output=True, text=True, timeout=timeout)


# D-Bus, as a desktop calls it.

bus = None


def call(dest, path, iface, method, args=None, reply=None, timeout=5000):
    v = bus.call_sync(dest, path, iface, method, args, GLib.VariantType(reply) if reply else None,
                      Gio.DBusCallFlags.NONE, timeout, None)
    return v.unpack() if v is not None else None


def has_owner(name):
    return call("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "NameHasOwner",
                GLib.Variant("(s)", (name,)))[0]


def get_all(dest, path, iface):
    return call(dest, path, "org.freedesktop.DBus.Properties", "GetAll", GLib.Variant("(s)", (iface,)))[0]


def get(dest, path, iface, name):
    try:
        return call(dest, path, "org.freedesktop.DBus.Properties", "Get", GLib.Variant("(ss)", (iface, name)))[0]
    except GLib.Error:
        return None


def status():
    return get(MPRIS, MPRIS_PATH, PLAYER, "PlaybackStatus")


def save_json(out, name, value):
    with open(os.path.join(out, name), "w") as f:
        json.dump(value, f, indent=2, default=str)


# Frames of the display, which ffmpeg reads.

def png(path, w, h, rgb):
    """Writes RGB pixels as a PNG."""
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF)
    rows = b"".join(b"\0" + rgb[y * w * 3:(y + 1) * w * 3] for y in range(h))
    with open(path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0))
                + chunk(b"IDAT", zlib.compress(rows, 6)) + chunk(b"IEND", b""))


def decode_png(path):
    """Reads a PNG's pixels as RGB, with ffmpeg."""
    with open(path, "rb") as f:
        w, h = struct.unpack(">II", f.read(24)[16:24])
    rgb = subprocess.run(["ffmpeg", "-loglevel", "error", "-i", path, "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
                         capture_output=True, check=True).stdout
    return w, h, rgb


def colours(w, h, rgb, step=4):
    """Counts the colours of every step-th pixel, both ways."""
    count = collections.Counter()
    for y in range(0, h, step):
        row = y * w * 3
        for x in range(0, w, step):
            i = row + x * 3
            count[rgb[i:i + 3]] += 1
    return count


def dominant(w, h, rgb):
    """The colour of most pixels, as #rrggbb, and its share of them."""
    count = colours(w, h, rgb)
    c, n = count.most_common(1)[0]
    return "#" + c.hex(), n / sum(count.values())


def near(a, b, tolerance=3):
    return all(abs(int(a[i:i + 2], 16) - int(b[i:i + 2], 16)) <= tolerance for i in (1, 3, 5))


def drawn(w, h, rgb):
    """Tells whether a frame shows something drawn: many colours, none of
    them nearly everywhere."""
    count = colours(w, h, rgb)
    share = count.most_common(1)[0][1] / sum(count.values())
    return len(count) >= 64 and share < 0.97, f"{len(count)} colours, the commonest on {share:.0%}"


class Display:
    def __init__(self, out):
        self.out = out
        self.size = tuple(int(n) for n in run("xdotool", "getdisplaygeometry").stdout.split())

    def grab(self, region, name=None):
        """The pixels of region, (x, y, w, h), of the display, saved as
        name.png when named."""
        x, y, w, h = region
        x, y = max(x, 0), max(y, 0)
        w, h = min(w, self.size[0] - x), min(h, self.size[1] - y)
        rgb = subprocess.run(
            ["ffmpeg", "-loglevel", "error", "-f", "x11grab", "-draw_mouse", "0",
             "-video_size", f"{self.size[0]}x{self.size[1]}", "-i", os.environ["DISPLAY"],
             "-frames:v", "1", "-vf", f"crop={w}:{h}:{x}:{y}", "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
            capture_output=True, check=True, timeout=30).stdout
        if name:
            png(os.path.join(self.out, name + ".png"), w, h, rgb)
        return w, h, rgb


# The sound server, listened to.

def monitor():
    sink = run("pactl", "get-default-sink").stdout.strip()
    return sink + ".monitor" if sink else ""


def listen(source, seconds=1.5):
    """Records the sound server's output for seconds, and returns its level
    in dBFS and its frequency, from its rising zero crossings."""
    p = subprocess.Popen(["parec", "-d", source, "--raw", "--format=s16le", "--rate=48000", "--channels=1",
                          "--latency-msec=20"], stdout=subprocess.PIPE)
    data = bytearray()

    def read():
        while True:
            b = p.stdout.read(4096)
            if not b:
                return
            data.extend(b)
    t = threading.Thread(target=read, daemon=True)
    t.start()
    time.sleep(seconds + 0.3)
    p.terminate()
    p.wait()
    t.join(5)
    samples = array.array("h", bytes(data[:len(data) // 2 * 2]))[48000 * 3 // 10:]  # past parec's start
    if not samples:
        return -math.inf, 0.0
    rms = math.sqrt(sum(s * s for s in samples) / len(samples)) / 32768
    rising = sum(1 for a, b in zip(samples, samples[1:]) if a < 0 <= b)
    return (20 * math.log10(rms) if rms > 0 else -math.inf), rising / (len(samples) / 48000)


# The profile MixMusic starts with.

def make_tone(work):
    """Makes the tone, which plays for 10 minutes."""
    tone = os.path.join(work, "tone.webm")
    subprocess.run(["ffmpeg", "-loglevel", "error", "-y", "-f", "lavfi", "-i",
                    f"sine=frequency={TONE_HZ}:sample_rate=48000:duration=600", "-ac", "2",
                    "-c:a", "libopus", "-b:a", "96k", "-f", "webm", tone], check=True)
    return tone


def fetch(url):
    """Downloads url, trying again twice."""
    for attempt in range(3):
        try:
            with urllib.request.urlopen(url, timeout=30) as r:
                return r.read()
        except OSError:
            if attempt == 2:
                raise
            time.sleep(5)


def make_profile(root, tone, cover_url):
    """A profile whose queue holds the track, downloaded, paused at its
    start, as MixMusic left it."""
    data, downloads = os.path.join(root, "data"), os.path.join(root, "downloads")
    os.makedirs(data)
    os.makedirs(os.path.join(downloads, "audio"))
    audio = os.path.join(downloads, "audio", VIDEO + ".webm")
    shutil.copy(tone, audio)
    track = {"id": "yt:" + VIDEO, "source": "youtube", "title": TITLE, "artists": [{"name": ARTIST}],
             "album": {"name": "Linux checks"}, "durationMs": 600000,
             "artwork": [{"url": cover_url, "width": 512, "height": 512}]}
    now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    index = {"version": 1, "seq": 1,
             "tracks": {track["id"]: {"track": track, "quality": "best", "pins": ["track"], "seq": 1, "added": now}},
             "files": {VIDEO: {"path": f"audio/{VIDEO}.webm", "bytes": os.path.getsize(audio), "format": "webm",
                               "codec": "opus", "bitrate": 96, "quality": "best", "done": now}},
             "collections": {}}
    with open(os.path.join(downloads, "index.json"), "w") as f:
        json.dump(index, f, indent=2)
    with open(os.path.join(data, "queue.json"), "w") as f:
        json.dump({"tracks": [track], "index": 0, "position": 0, "shuffle": False, "repeat": "off"}, f, indent=2)
    return os.path.join(data, "settings.json")


def stored(folder):
    """The images the artwork store keeps in folder: each file is a line
    of JSON, then the image."""
    images = []
    for name in os.listdir(folder) if os.path.isdir(folder) else []:
        with open(os.path.join(folder, name), "rb") as f:
            images.append(f.read().split(b"\n", 1)[-1])
    return images


def app_env(profile, extra=None):
    env = dict(os.environ, MIXMUSIC_DATA_DIR=profile, GDK_BACKEND="x11", NO_AT_BRIDGE="1")
    env.update(extra or {})
    return env


def windows(pid):
    """The visible windows of pid, the largest first, as (id, x, y, w, h)."""
    found = []
    for wid in run("xdotool", "search", "--onlyvisible", "--pid", str(pid)).stdout.split():
        g = dict(line.split("=", 1) for line in run("xdotool", "getwindowgeometry", "--shell", wid).stdout.split())
        if "WIDTH" in g:
            found.append((wid, int(g["X"]), int(g["Y"]), int(g["WIDTH"]), int(g["HEIGHT"])))
    return sorted(found, key=lambda w: -w[3] * w[4])


def libraries(pid):
    """The files of the libraries pid mapped, by the names of the files,
    such as libgtk-3.so.0.2409.32."""
    libs = {}
    with open(f"/proc/{pid}/maps") as f:
        for line in f:
            fields = line.split(None, 5)
            path = fields[5].strip() if len(fields) == 6 else ""
            if ".so" in path:
                libs.setdefault(os.path.basename(path), path)
    return libs


def inotify_inodes(pid):
    """The inodes the inotify instances of pid watch."""
    inodes = set()
    for fd in os.listdir(f"/proc/{pid}/fdinfo"):
        try:
            with open(f"/proc/{pid}/fdinfo/{fd}") as f:
                text = f.read()
        except OSError:
            continue
        for line in text.splitlines():
            if line.startswith("inotify wd:"):
                inodes.update(int(p[4:], 16) for p in line.split() if p.startswith("ino:"))
    return inodes


def read_settings(path):
    try:
        with open(path) as f:
            return json.load(f)
    except (OSError, ValueError):
        return None


def edit_settings(path, change, in_place=False):
    """Changes settings.json as an editor does: in place, or by writing a
    new file over it."""
    data = until(lambda: read_settings(path), 5)
    change(data)
    text = json.dumps(data, indent=2)
    if in_place:
        with open(path, "w") as f:
            f.write(text)
    else:
        with open(path + ".edit", "w") as f:
            f.write(text)
        os.replace(path + ".edit", path)


def capture_run(c, mixmusic, profile, out):
    """MixMusic in capture mode, which saves its own frames: home,
    Settings › About, and Now playing as the track plays, into no device."""
    shots = os.path.join(out, "capture")
    env = app_env(profile, {
        "MIXMUSIC_CAPTURE": shots,
        "MIXMUSIC_CAPTURE_ROUTES": "/|/settings/about|act:resume|act:wait:4s|/now-playing|act:title",
        "MIXMUSIC_CAPTURE_WAIT": "8s",
        "MIXMUSIC_CAPTURE_SIZE": "1280x800",
        "MIXMUSIC_CAPTURE_QUIT": "1",
        "MIXMUSIC_SILENT": "1",
    })
    with open(os.path.join(out, "capture-run.log"), "w") as log:
        p = subprocess.Popen([mixmusic], env=env, stdout=log, stderr=subprocess.STDOUT)
        try:
            code = p.wait(timeout=180)
        except subprocess.TimeoutExpired:
            p.kill()
            code = "still running after 180 s"
    c.add("Capture mode: MixMusic starts, saves its frames and quits", code == 0, f"exit {code}")
    for name in ["01-home", "02-settings-about", "03-now-playing"]:
        path = os.path.join(shots, name + ".png")
        if not os.path.exists(path):
            c.add(f"Capture mode: {name}.png shows the window drawn", False, "no frame")
            continue
        ok, saw = drawn(*decode_png(path))
        c.add(f"Capture mode: {name}.png shows the window drawn", ok, saw)
    title = ""
    log_file = os.path.join(profile, "logs", "mixmusic.log")
    if os.path.exists(log_file):
        shutil.copy(log_file, os.path.join(out, "capture-mixmusic.log"))
        with open(log_file) as f:
            for line in f:
                if "the window's title is" in line:
                    title = line.split("the window's title is", 1)[1].strip()
    c.add("Capture mode: the downloaded track plays without the network, and names the window",
          TITLE in title, f"title {title or 'not logged'}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--libs", choices=["nix", "system"], default="system")
    ap.add_argument("mixmusic")
    ap.add_argument("out")
    ap.add_argument("--watcher", help=argparse.SUPPRESS)
    args = ap.parse_args()
    if args.watcher:
        watcher(args.watcher)
        return

    global bus
    out = os.path.abspath(args.out)
    os.makedirs(out, exist_ok=True)
    mixmusic = os.path.abspath(args.mixmusic)
    c = Checks(out)
    work = os.path.join(out, "work")
    os.makedirs(work, exist_ok=True)
    tone = make_tone(work)
    cover = fetch(COVER)
    bus = Gio.bus_get_sync(Gio.BusType.SESSION, None)

    capture = os.path.join(work, "capture-profile")
    make_profile(capture, tone, COVER)
    capture_run(c, mixmusic, capture, out)

    # The stand-in for the panel's tray, before MixMusic starts, as on a
    # desktop.
    items_file = os.path.join(out, "tray-items.json")
    watcher_proc = subprocess.Popen([sys.executable, os.path.abspath(__file__), "--watcher", items_file, "-", "-"])
    until(lambda: has_owner(WATCHER), 10)
    # An idle null sink wakes every 2 seconds, and starts a new stream at
    # its next waking: silence played at a low latency keeps it awake, as
    # in ci.yml's audio job.
    source = monitor()
    keep_awake = subprocess.Popen(["pacat", "--latency-msec=10", "--format=s16le", "--rate=48000", "--channels=2",
                                   "/dev/zero"]) if source else None

    profile = os.path.join(work, "profile")
    settings = make_profile(profile, tone, COVER)
    pywal = os.path.join(work, "xdg-cache", "wal", "colors.json")
    display = Display(out)
    started = time.monotonic()
    app_log = open(os.path.join(out, "mixmusic-stderr.log"), "w")
    app = subprocess.Popen([mixmusic], env=app_env(profile, {"XDG_CACHE_HOME": os.path.join(work, "xdg-cache")}),
                           stdout=app_log, stderr=subprocess.STDOUT)
    try:
        check_app(c, args, app, started, display, settings, profile, pywal, items_file, source, cover, out)
    finally:
        if app.poll() is None:
            app.send_signal(signal.SIGTERM)
            try:
                app.wait(10)
            except subprocess.TimeoutExpired:
                app.kill()
        for p in [watcher_proc, keep_awake]:
            if p:
                p.kill()
        app_log.close()
        for name in ["mixmusic.log", "crash.log"]:
            path = os.path.join(profile, "logs", name)
            if os.path.exists(path):
                shutil.copy(path, os.path.join(out, name))
        if os.path.exists(settings):
            shutil.copy(settings, os.path.join(out, "settings.json"))
        shutil.rmtree(work, ignore_errors=True)
    ok = c.save(f"MixMusic on Linux ({args.libs} libraries)")
    sys.exit(0 if ok else 1)


def check_app(c, args, app, started, display, settings, profile, pywal, items_file, source, cover, out):
    # It starts, and shows its window.
    found = until(lambda: app.poll() is not None or windows(app.pid), 90)
    if not c.add("MixMusic starts and shows its window", app.poll() is None and found,
                 f"in {time.monotonic() - started:.1f} s" if found and app.poll() is None
                 else f"exited with {app.poll()}" if app.poll() is not None else "no window in 90 s"):
        return
    time.sleep(5)  # its first frames
    wid, *region = windows(app.pid)[0]
    region = tuple(region)
    ok, saw = drawn(*display.grab(region, "01-window"))
    c.add("The window on the display is drawn", ok, f"{region[2]}x{region[3]}, {saw}")

    # Its libraries: the system's for the release build, the Nix store's
    # for the Nix build.
    libs = libraries(app.pid)
    save_json(out, "libraries.json", libs)
    gtk = next((p for n, p in libs.items() if n.startswith("libgtk-3.so")), "")
    loader = next((p for n, p in libs.items() if n.startswith("ld-linux")), "")
    want_nix = args.libs == "nix"
    c.add(f"GTK and the loader are the {'Nix store' if want_nix else 'system'}'s",
          gtk and loader and gtk.startswith("/nix/store/") == want_nix and loader.startswith("/nix/store/") == want_nix,
          f"{gtk or 'no GTK'}, {loader or 'no loader'}")

    # The tray icon, which libayatana-appindicator registers with the
    # desktop's StatusNotifierWatcher.
    def tray_item():
        try:
            with open(items_file) as f:
                return json.load(f)[-1]
        except (OSError, ValueError, IndexError):
            return None
    item = until(tray_item, 30)
    tray = None
    if c.add("The tray icon registers with the desktop's StatusNotifierWatcher", item,
             f"{item['bus']} {item['path']}" if item else "no item in 30 s"):
        tray = item
        props = get_all(item["bus"], item["path"], ITEM)
        save_json(out, "tray-item.json", props)
        icon = os.path.join(props.get("IconThemePath", ""), props.get("IconName", "") + ".png")
        with open(os.path.join(os.path.dirname(__file__), "..", "..", "resources", "tray.png"), "rb") as f:
            same = os.path.isfile(icon) and open(icon, "rb").read() == f.read()
        c.add("The tray item is active, with MixMusic's icon", props.get("Status") == "Active" and same,
              f"Status {props.get('Status')}, icon {icon}{' = resources/tray.png' if same else ' differs or is missing'}")
        layout = call(item["bus"], props["Menu"], MENU, "GetLayout", GLib.Variant("(iias)", (0, -1, [])))[1]
        save_json(out, "tray-menu.json", layout)
        labels = {}

        def walk(node):
            node_id, node_props, children = node
            if node_props.get("label"):
                labels[node_props["label"].replace("_", "")] = node_id
            for child in children:
                walk(child)
        walk(layout)
        tray = dict(item, menu=props["Menu"], labels=labels)
        missing = [t for t in TRAY_ITEMS if t not in labels]
        quit_item = any(label.startswith("Quit") for label in labels)
        c.add("The tray menu has the player's commands and Quit", not missing and quit_item,
              ", ".join(labels) + (f"; missing {', '.join(missing)}" if missing else ""))

    # settings.json, which MixMusic writes at its start and watches.
    until(lambda: read_settings(settings), 20)
    pid = app.pid
    watched = inotify_inodes(pid)
    folder = os.stat(os.path.dirname(settings)).st_ino
    c.add("settings.json is watched through inotify", folder in watched or os.stat(settings).st_ino in watched,
          f"{len(watched)} inotify watches, the data folder's inode {folder} "
          + ("among them" if folder in watched else "not among them"))

    # The zoom keys in the window, which the GTK handler of #251 takes for
    # the keypad's +: read back from settings.json, which MixMusic saves.
    run("xdotool", "windowfocus", "--sync", wid, timeout=10)
    zoom = lambda: (read_settings(settings) or {}).get("zoom")  # noqa: E731
    for i, (keys, want) in enumerate([("ctrl+KP_Add", 1.1), ("ctrl+equal", 1.25), ("ctrl+KP_Subtract", 1.1),
                                      ("ctrl+minus", 1.0), ("ctrl+KP_Add", 1.1), ("ctrl+0", 1.0)]):
        before = zoom()
        run("xdotool", "key", keys)
        got = until(lambda: zoom() if zoom() == want else None, 5) or zoom()
        display.grab(region, f"02-zoom-{i + 1}-{keys.replace('+', '-')}")
        c.add(f"{keys.replace('ctrl+', 'Ctrl+')} zooms to {want:.0%}", got == want, f"zoom {before} → {got}")

    # settings.json edited while MixMusic runs (#81): the theme, written
    # over the file, and the zoom, in place. Most of the window is the
    # colours' bg, or their surface, which the panel and the cards wear.
    def shows(*want, timeout=10):
        seen = []

        def look():
            colour, share = dominant(*display.grab(region))
            seen.append(colour)
            return any(near(colour, w) for w in want)
        t = time.monotonic()
        ok = until(look, timeout, every=0.3)
        return ok, (f"{seen[-1] if seen else 'nothing'} on most of the window, after {time.monotonic() - t:.1f} s"
                    f" (bg or surface: {' or '.join(want)})")

    base, _ = dominant(*display.grab(region))
    edit_settings(settings, lambda s: s.update(theme="daylight", followSystem=False))
    ok, saw = shows("#ffffff", "#f6f6f6")  # builtin/themes/daylight
    display.grab(region, "03-settings-theme-daylight")
    c.add("settings.json's theme, changed while it runs, shows", ok, f"from {base}: {saw}")
    before = display.grab(region)[2]
    edit_settings(settings, lambda s: s.update(zoom=1.5), in_place=True)
    time.sleep(2)
    w, h, after = display.grab(region, "04-settings-zoom-150")
    changed = sum(1 for i in range(0, len(after), 3 * 97) if after[i:i + 3] != before[i:i + 3]) / (len(after) / (3 * 97))
    c.add("settings.json's zoom, changed in place while it runs, shows",
          changed > 0.05 and (read_settings(settings) or {}).get("zoom") == 1.5, f"{changed:.0%} of the window changed")
    edit_settings(settings, lambda s: s.update(zoom=1))

    # Colour files (#74): pywal's colors.json, where pywal writes it under
    # XDG_CACHE_HOME, followed as it changes, and matugen's, in the data
    # folder.
    foreground = "#e8e4f0"

    def pywal_file(background):
        os.makedirs(os.path.dirname(pywal), exist_ok=True)
        palette = {f"color{i}": v for i, v in enumerate(
            [background, "#c94f6d", "#81b29a", "#dbc074", "#719cd6", "#9d79d6", "#63cdcf", "#dfdfe0"] * 2)}
        with open(pywal + ".tmp", "w") as f:
            json.dump({"wallpaper": "/tmp/wall.png", "alpha": "100",
                       "special": {"background": background, "foreground": foreground, "cursor": foreground},
                       "colors": palette}, f, indent=2)
        os.replace(pywal + ".tmp", pywal)

    def surface(bg):
        # pywal's surface, 4.5% of the way to its foreground (pywalTokens).
        return "#" + "".join(f"{round(int(bg[i:i + 2], 16) * 0.955 + int(foreground[i:i + 2], 16) * 0.045):02x}"
                             for i in (1, 3, 5))
    pywal_file("#1e2a4a")
    edit_settings(settings, lambda s: s.update(colorSource="pywal"))
    ok, saw = shows("#1e2a4a", surface("#1e2a4a"))
    display.grab(region, "05-colours-pywal")
    c.add("pywal's colors.json in $XDG_CACHE_HOME/wal colours the window", ok, saw)
    pywal_file("#4a1e2a")
    ok, saw = shows("#4a1e2a", surface("#4a1e2a"))
    display.grab(region, "06-colours-pywal-changed")
    c.add("A change of pywal's colors.json shows while it runs", ok, saw)
    matugen = os.path.join(os.path.dirname(settings), "matugen.json")
    tokens = {"surface": "#24302e", "surface-2": "#2e3b39", "surface-3": "#394745", "border": "#4a5856",
              "text": "#e0f0ec", "text-muted": "#a8bab6", "accent": "#5fd4bd", "accent-text": "#00382f"}
    with open(matugen, "w") as f:
        json.dump({"dark": dict(tokens, bg="#12332d"), "light": dict(tokens, bg="#f0fbf8", text="#10201c")}, f)
    edit_settings(settings, lambda s: s.update(colorSource="matugen", theme="mix"))
    ok, saw = shows("#12332d", tokens["surface"])
    display.grab(region, "07-colours-matugen")
    c.add("matugen.json in the data folder colours the window", ok, saw)
    edit_settings(settings, lambda s: s.update(colorSource=""))
    shows(base)
    c.add("settings.json was never watched by polling",
          "looking at it every" not in open(os.path.join(profile, "logs", "mixmusic.log")).read(), "no fallback logged")

    # The media keys: a press of the keyboard's play key, which MixMusic
    # grabs on X11, plays the queue it kept: the track, downloaded. The
    # MPRIS player comes once something plays.
    run("xdotool", "key", "XF86AudioPlay")
    came = until(lambda: has_owner(MPRIS), 20)
    playing = came and until(lambda: status() == "Playing", 10)
    if not c.add("The keyboard's play key plays, and MPRIS's player comes", playing,
                 f"{MPRIS} {'on the bus' if came else 'not on the bus in 20 s'}, {status() if came else ''}"):
        if not came:
            # Space plays too, from the window: the MPRIS checks still run.
            run("xdotool", "windowfocus", "--sync", wid, timeout=10)
            run("xdotool", "key", "space")
            if not until(lambda: has_owner(MPRIS), 20):
                return
    xml = call(MPRIS, MPRIS_PATH, "org.freedesktop.DBus.Introspectable", "Introspect")[0]
    with open(os.path.join(out, "mpris-introspect.xml"), "w") as f:
        f.write(xml)
    c.add("MPRIS's player introspects, with its two interfaces",
          'interface name="org.mpris.MediaPlayer2"' in xml and f'interface name="{PLAYER}"' in xml, f"{len(xml)} bytes")
    root = get_all(MPRIS, MPRIS_PATH, "org.mpris.MediaPlayer2")
    c.add("MPRIS names MixMusic", root.get("Identity") == "MixMusic" and root.get("DesktopEntry") == "mixmusic",
          f"Identity {root.get('Identity')}, DesktopEntry {root.get('DesktopEntry')}, CanRaise {root.get('CanRaise')}, "
          f"CanQuit {root.get('CanQuit')}")
    art = until(lambda: get_all(MPRIS, MPRIS_PATH, PLAYER)["Metadata"].get("mpris:artUrl"), 30)
    player = get_all(MPRIS, MPRIS_PATH, PLAYER)
    save_json(out, "mpris.json", {"org.mpris.MediaPlayer2": root, PLAYER: player})
    meta = player["Metadata"]
    c.add("MPRIS shows the track", meta.get("xesam:title") == TITLE and meta.get("xesam:artist") == [ARTIST]
          and meta.get("mpris:length", 0) > 0, f"{meta.get('xesam:title')} by {meta.get('xesam:artist')}, "
          f"{meta.get('mpris:length', 0) / 1e6:.0f} s")
    cover_dir = os.path.join(profile, "cache", "now-playing")
    path = GLib.filename_from_uri(art)[0] if art and art.startswith("file://") else ""
    kept = open(path, "rb").read() if path.startswith(cover_dir + os.sep) and os.path.isfile(path) else None
    in_store = kept is not None and kept in stored(os.path.join(profile, "cache", "artwork"))
    c.add("MPRIS's cover is a file of the cache's now-playing folder, byte for byte the artwork store's (#196)",
          kept == cover and in_store,
          f"{art or 'no mpris:artUrl in 30 s'}: " + (
              "not in now-playing" if kept is None else
              f"{len(kept)} bytes, {'the' if kept == cover else 'not the'} cover's, "
              f"{'as' if in_store else 'not as'} the store keeps it"))
    level, hz = listen(source) if source else (-math.inf, 0)
    c.add("The track sounds from the sound server", level > -60 and abs(hz - TONE_HZ) < 15,
          f"{level:.1f} dBFS at {hz:.0f} Hz" if source else "no sound server")
    p1 = get(MPRIS, MPRIS_PATH, PLAYER, "Position")
    time.sleep(1)
    p2 = get(MPRIS, MPRIS_PATH, PLAYER, "Position")
    c.add("MPRIS's position moves on as it plays", p1 is not None and p2 is not None and p2 > p1,
          f"{(p1 or 0) / 1e6:.2f} s, then {(p2 or 0) / 1e6:.2f} s")
    display.grab(region, "08-playing")

    # The desktop's controls, through MPRIS.
    call(MPRIS, MPRIS_PATH, PLAYER, "PlayPause")
    paused = until(lambda: status() == "Paused", 5)
    level, _ = listen(source) if source else (-math.inf, 0)
    c.add("MPRIS's PlayPause pauses, and the sound stops", paused and level < -70,
          f"{status()}, {level:.1f} dBFS")
    call(MPRIS, MPRIS_PATH, PLAYER, "PlayPause")
    c.add("MPRIS's PlayPause plays again", until(lambda: status() == "Playing", 5), status())
    call(MPRIS, MPRIS_PATH, PLAYER, "Seek", GLib.Variant("(x)", (120_000_000,)))
    moved = until(lambda: (get(MPRIS, MPRIS_PATH, PLAYER, "Position") or 0) > 115_000_000, 5)
    c.add("MPRIS's Seek moves on 2 minutes", moved, f"at {(get(MPRIS, MPRIS_PATH, PLAYER, 'Position') or 0) / 1e6:.1f} s")
    c.add("The window's title names what plays", TITLE in run("xdotool", "getwindowname", wid).stdout,
          run("xdotool", "getwindowname", wid).stdout.strip())

    # A global shortcut of the settings (#80), on X11, with another window
    # in front.
    edit_settings(settings, lambda s: s.update(globalShortcuts={"player.toggle": "Mod+Alt+P"}))
    time.sleep(2)
    other = subprocess.Popen(["xmessage", "-geometry", "+0+0", "Another window in front"])
    front = until(lambda: run("xdotool", "search", "--onlyvisible", "--classname", "xmessage").stdout.split(), 10)
    if front:
        run("xdotool", "windowfocus", "--sync", front[0], timeout=10)
    focused = run("xdotool", "getwindowfocus").stdout.strip()
    run("xdotool", "key", "ctrl+alt+p")
    in_front = bool(front) and focused == front[0]
    c.add("A global shortcut of settings.json pauses with another window in front (X11)",
          in_front and until(lambda: status() == "Paused", 5),
          f"{status()}, with {'xmessage' if in_front else 'window ' + focused} focused")
    other.kill()

    def click(label):
        call(tray["bus"], tray["menu"], MENU, "Event",
             GLib.Variant("(isvu)", (tray["labels"][label], "clicked", GLib.Variant("i", 0), 0)))

    # The tray menu's Play / Pause, as a panel clicks it, once the player
    # takes a toggle again: it takes one in 400 ms (toggleDebounce).
    if tray and "Play / Pause" in tray["labels"]:
        time.sleep(1)
        click("Play / Pause")
        c.add("The tray menu's Play / Pause plays", until(lambda: status() == "Playing", 5), status())
        tip = get(tray["bus"], tray["path"], ITEM, "Title") or ""
        c.add("The tray item's title names what plays", TITLE in tip, tip)

        # Its Mini player opens a window of its own.
        click("Mini player")
        mini = until(lambda: [w for w in windows(app.pid) if w[0] != wid], 10)
        if mini:
            time.sleep(2)
            display.grab(tuple(mini[0][1:]), "09-mini-player")
        c.add("The tray menu's Mini player opens it", mini,
              f"{mini[0][3]}x{mini[0][4]} window" if mini else "no second window in 10 s")

    # The tray menu's Quit quits: MPRIS's player can't (CanQuit).
    quit_label = next((label for label in (tray or {}).get("labels", {}) if label.startswith("Quit")), "")
    if quit_label:
        click(quit_label)
    try:
        code = app.wait(20)
    except subprocess.TimeoutExpired:
        code = "still running after 20 s"
    c.add("The tray menu's Quit quits MixMusic", code == 0, f"exit {code}")
    crash = os.path.join(profile, "logs", "crash.log")
    c.add("No crash was logged", not os.path.exists(crash) or os.path.getsize(crash) == 0,
          f"crash.log {os.path.getsize(crash) if os.path.exists(crash) else 0} bytes")


if __name__ == "__main__":
    main()
