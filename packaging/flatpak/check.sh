#!/usr/bin/env bash
# Checks a bundle of MixMusic's Flatpak, as ci.yml and release.yml build
# it, in their flatpak-builder container:
#
#   packaging/flatpak/check.sh <bundle> [shots]
#
# It installs the bundle, finds the app, its launcher and Node.js where
# the manifest puts them, loads in the sandbox each library MixMusic and
# MyGo load, and runs Node.js on the runtime's OpenSSL, zlib and ICU. With
# shots, it starts MixMusic in the sandbox too, under a virtual X server
# and a session bus of its own, which saves screenshots of home, Settings ›
# About and Settings › Playback, with its tools, into shots, then quits.
set -euo pipefail

bundle="${1:?usage: check.sh <bundle> [shots]}"
shots="${2:-}"
id=io.github.fzn0x.mixmusic

flatpak install --user --noninteractive --assumeyes --bundle "$bundle"
flatpak info --user "$id"
flatpak info --user --show-permissions "$id"

echo "== The app, its launcher and Node.js"
flatpak run --user --command=sh "$id" -c '
	set -e
	test -x /app/lib/mixmusic/mixmusic
	test -x /app/bin/mixmusic
	test "$(command -v node)" = /app/bin/node
	test -f /app/share/applications/io.github.fzn0x.mixmusic.desktop
	test -f /app/share/metainfo/io.github.fzn0x.mixmusic.metainfo.xml
	test -f /app/share/icons/hicolor/scalable/apps/io.github.fzn0x.mixmusic.svg
	cat /app/share/applications/io.github.fzn0x.mixmusic.desktop
'
flatpak run --user --command=node "$id" -e '
	require("crypto").createHash("sha256").update("mixmusic").digest("hex");
	require("zlib").gzipSync("mixmusic");
	new Intl.DateTimeFormat("ja-JP", { dateStyle: "full" }).format(0);
	console.log("Node.js " + process.version + ", ICU " + process.versions.icu + ", OpenSSL " + process.versions.openssl);
'

echo "== The libraries MixMusic loads"
flatpak run --user --command=python3 "$id" -c '
import ctypes
for lib in [
    "libglib-2.0.so.0", "libgobject-2.0.so.0", "libgio-2.0.so.0",
    "libgtk-3.so.0", "libgdk-3.so.0", "libcairo.so.2", "libgdk_pixbuf-2.0.so.0", "libepoxy.so.0",
    "libayatana-appindicator3.so.1", "libdbusmenu-glib.so.4", "libdbusmenu-gtk3.so.4",
    "libwebkit2gtk-4.1.so.0", "libjavascriptcoregtk-4.1.so.0", "libsoup-3.0.so.0",
    "libsecret-1.so.0", "libpulse-simple.so.0",
]:
    ctypes.CDLL(lib)
    print("loaded", lib)
'

[ -n "$shots" ] || exit 0

echo "== MixMusic in the sandbox"
mkdir -p "$shots"
runtime_dir="$(mktemp -d)"
chmod 700 "$runtime_dir"
# The sandbox sees its own folder of the home folder, at the same path.
inside="$HOME/.var/app/$id/capture"
# GTK loads images through glycin, whose loaders run in sandboxes of their
# own that flatpak-spawn asks the Flatpak portal for, as on a desktop: the
# session bus gets flatpak's portal, which nothing starts here.
portal="$(command -v flatpak-portal || ls /app/libexec/flatpak-portal /usr/libexec/flatpak-portal 2>/dev/null | head -n 1 || true)"
if [ -z "$portal" ]; then
	echo "No flatpak-portal: images won't load" >&2
fi
export PORTAL="$portal" ID="$id" INSIDE="$inside"
status=0
XDG_RUNTIME_DIR="$runtime_dir" timeout 300 dbus-run-session -- bash -c '
	if [ -n "$PORTAL" ]; then
		"$PORTAL" --replace &
		for _ in $(seq 50); do
			gdbus call --session --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus \
				--method org.freedesktop.DBus.NameHasOwner org.freedesktop.portal.Flatpak 2>/dev/null | grep -q true && break
			sleep 0.1
		done
	fi
	xvfb-run --auto-servernum -s "-screen 0 1600x1000x24" \
		flatpak run --user \
		--env=MIXMUSIC_CAPTURE="$INSIDE" \
		--env=MIXMUSIC_CAPTURE_ROUTES="/|/settings/about|/settings/playback|act:scroll:end|act:shot:settings-playback-tools" \
		--env=MIXMUSIC_CAPTURE_WAIT=15s \
		--env=MIXMUSIC_CAPTURE_SIZE=1440x900 \
		--env=MIXMUSIC_CAPTURE_QUIT=1 \
		--env=MIXMUSIC_SILENT=1 \
		"$ID"
' || status=$?
cp -r "$inside/." "$shots/" 2>/dev/null || true
while IFS= read -r log; do
	echo "-- $log"
	cat "$log"
	cp "$log" "$shots/"
done < <(find "$HOME/.var/app/$id" -name '*.log' -type f)
ls -l "$shots"
if [ "$status" -ne 0 ]; then
	echo "MixMusic exited with $status" >&2
	exit "$status"
fi
for shot in 01-home 02-settings-about 03-settings-playback 04-settings-playback-tools; do
	if [ ! -s "$shots/$shot.png" ]; then
		echo "MixMusic saved no $shot.png" >&2
		exit 1
	fi
done
