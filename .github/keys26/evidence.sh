#!/usr/bin/env bash
# Temporary (#407): what the keypad's keys become on this runner, from the
# X server's keymap to what GTK 3 hands a window. Removed before the pull
# request is ready.
set -u
out="${1:-keys26}"
mkdir -p "$out"
exec > >(tee "$out/evidence.txt") 2>&1

section() {
	echo
	echo "=== $*"
}
keys=(ctrl+KP_Add ctrl+KP_Subtract KP_Add ctrl+equal ctrl+minus ctrl+alt+KP_Add)

section "System"
. /etc/os-release
echo "$PRETTY_NAME, image ${ImageOS:-?} ${ImageVersion:-?}"
dpkg-query -W -f='${Package} ${Version}\n' xvfb xserver-common xdotool libxdo3 xkb-data x11-xkb-utils \
	libgtk-3-0t64 libx11-6 libxkbcommon0 libxkbfile1 xinput x11-utils 2>&1

section "Xvfb"
Xvfb :99 -screen 0 1280x1024x24 -nolisten tcp >"$out/xvfb.log" 2>&1 &
export DISPLAY=:99
for _ in $(seq 100); do
	xdpyinfo >/dev/null 2>&1 && break
	sleep 0.1
done
xdpyinfo | grep -iE 'version|vendor|XKEYBOARD|keycode range'
echo "--- Xvfb's own output"
cat "$out/xvfb.log"

section "setxkbmap -query, -print"
setxkbmap -query
setxkbmap -print

section "xkbcomp -xkb :99 (excerpts; whole in keymap.xkb)"
xkbcomp -xkb :99 "$out/keymap.xkb"
grep -nE '<KPAD>|<KPSU>|<AE11>|<AE12>' "$out/keymap.xkb"
for key in KPAD KPSU AE11 AE12; do
	awk -v k="key <$key>" 'index($0, k) { p = 1 } p { print } p && /};/ { p = 0 }' "$out/keymap.xkb"
done
awk '/type "CTRL\+ALT"/ { p = 1 } p { print } p && /};/ { p = 0 }' "$out/keymap.xkb"
grep -n -A4 -E 'interpret XF86_(Next|Prev)_VMode' "$out/keymap.xkb"

section "xmodmap -pke (keypad, = and -), -pm"
xmodmap -pke >"$out/xmodmap-pke.txt"
grep -nE 'KP_Add|KP_Subtract|= equal|= minus|VMode' "$out/xmodmap-pke.txt"
xmodmap -pm

section "The keycodes xdotool sends (xinput test-xi2 --root, raw events)"
for k in "${keys[@]}"; do
	echo "--- xdotool key $k"
	stdbuf -oL xinput test-xi2 --root >"$out/xi2-$k.txt" 2>&1 &
	xi=$!
	sleep 0.5
	xdotool key "$k"
	sleep 0.5
	kill "$xi"
	wait "$xi" 2>/dev/null
	grep -A2 -E 'EVENT type .*\(Raw' "$out/xi2-$k.txt" | grep -E 'EVENT|detail'
done

section "xev's window, focused (X's events as a client gets them)"
stdbuf -oL xev -event keyboard -name keys26-xev >"$out/xev.txt" 2>&1 &
ev=$!
xdotool search --sync --onlyvisible --name '^keys26-xev$' windowfocus --sync
sleep 0.3
for k in "${keys[@]}"; do
	echo "--- xdotool key $k" >>"$out/xev-marks.txt"
	xdotool key "$k"
	sleep 0.4
done
kill "$ev"
wait "$ev" 2>/dev/null
grep -E 'KeyPress|KeyRelease|state|XLookupString' "$out/xev.txt" | sed 's/^ *//'

section "GTK 3's window (key-press-event and key-release-event)"
GDK_BACKEND=x11 NO_AT_BRIDGE=1 python3 .github/keys26/gtkkeys.py "${keys[@]}"

kill %1 2>/dev/null
true
