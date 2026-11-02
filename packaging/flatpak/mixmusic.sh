#!/bin/sh
# Starts MixMusic in its Flatpak.
#
# MyGo writes the tray icon into a temporary folder and gives the desktop's
# tray the folder's path, which the tray reads outside the sandbox, where
# the sandbox's /tmp isn't. $XDG_RUNTIME_DIR/app/$FLATPAK_ID has the same
# path on both sides: inside, $XDG_RUNTIME_DIR/app is a link to the folder
# the desktop's is bound to. Keep the path unresolved, as Go's os.TempDir
# and MyGo do.
dir="$XDG_RUNTIME_DIR/app/$FLATPAK_ID"
if [ -n "$XDG_RUNTIME_DIR" ] && [ -n "$FLATPAK_ID" ] && mkdir -p "$dir" 2>/dev/null; then
	export TMPDIR="$dir"
fi
exec /app/lib/mixmusic/mixmusic "$@"
