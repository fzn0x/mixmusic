#!/usr/bin/env bash
# Makes the downloads mygo build does not: a portable zip of each Windows
# build, and an RPM package and an AppImage of each Linux build, from what
# mygo build left in build/<target>/.
#
#   .github/scripts/package.sh <version> [out]
#
# The RPM installs the files the .deb installs. The AppImage holds the app
# alone and runs on the system's GTK 3 and WebKitGTK 4.1, as the other
# Linux downloads do.
set -euo pipefail
shopt -s nullglob

v="${1:?usage: package.sh <version> [out]}"
out="${2:-release}"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# The tools, pinned, and checked against the checksums of their files when
# they were picked.
nfpm="github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.48.0"
appimagetool_url="https://github.com/AppImage/appimagetool/releases/download/1.9.1/appimagetool-x86_64.AppImage"
appimagetool_sha256="ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0"
runtime_url="https://github.com/AppImage/type2-runtime/releases/download/20251108"
declare -A runtime_sha256=(
	[x86_64]="2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d"
	[aarch64]="00cbdfcf917cc6c0ff6d3347d59e0ca1f7f45a6df1a428a0d6d8a78664d87444"
)

# fetch downloads url to dest and checks its SHA-256.
fetch() {
	curl -fsSL --retry 3 -o "$3" "$1"
	echo "$2  $3" | sha256sum -c --quiet -
}

# Windows: the app and the files beside it, in a MixMusic folder that runs
# from wherever it is unpacked.
for dir in build/windows-*/; do
	arch="$(basename "$dir")"
	arch="${arch#windows-}"
	stage="$work/portable-$arch/MixMusic"
	mkdir -p "$stage"
	cp "$dir"MixMusic.exe "$stage/"
	for f in "$dir"*.png; do cp "$f" "$stage/"; done
	(cd "$work/portable-$arch" && zip -qr9 "$out/mixmusic-$v-windows-$arch-portable.zip" MixMusic)
	echo "made mixmusic-$v-windows-$arch-portable.zip"
done

linux=(build/linux-*/)
[ ${#linux[@]} -eq 0 ] && exit 0
fetch "$appimagetool_url" "$appimagetool_sha256" "$work/appimagetool"
chmod +x "$work/appimagetool"

for dir in "${linux[@]}"; do
	arch="$(basename "$dir")"
	arch="${arch#linux-}" # amd64, arm64
	debs=("$dir"*.deb)
	if [ ${#debs[@]} -ne 1 ]; then
		echo "package.sh: want one .deb in $dir, found ${#debs[@]}" >&2
		exit 1
	fi
	deb="$(cd "$(dirname "${debs[0]}")" && pwd)/$(basename "${debs[0]}")"

	# The files the .deb installs.
	root="$work/root-$arch"
	mkdir -p "$root" "$work/deb-$arch"
	(cd "$work/deb-$arch" && ar x "$deb")
	tar --force-local -xf "$work/deb-$arch"/data.tar.* -C "$root"

	# RPM: the same files, the same paths, and the libraries of Fedora's
	# names; the tray icon, the keyring and the AAC decoder are wanted, not
	# needed.
	config="$work/nfpm-$arch.yaml"
	{
		echo "name: mixmusic"
		echo "arch: $arch"
		echo "platform: linux"
		echo "version: $v"
		echo "release: 1"
		echo "maintainer: MixMusic contributors"
		echo "description: Spotify and YouTube Music in one customizable player"
		echo "homepage: https://github.com/fzn0x/mixmusic"
		echo "license: MIT"
		echo "depends: [gtk3, webkit2gtk4.1, '(pulseaudio-libs or alsa-lib)']"
		echo "recommends: [libayatana-appindicator-gtk3, libsecret]"
		echo "rpm: {compression: xz}"
		echo "contents:"
		echo "  - dst: /opt/mixmusic"
		echo "    type: dir"
		# The files of the app and its entry; the command is a link to the
		# app, made here, whatever the .deb's file of it unpacked to.
		(cd "$root" && find . -type f -not -path './usr/bin/*' | sed 's|^\./||' | sort) | while read -r f; do
			mode="0$(stat -c %a "$root/$f")"
			[ "$f" = opt/mixmusic/mixmusic ] && mode=0755
			echo "  - src: $root/$f"
			echo "    dst: /$f"
			echo "    file_info: {mode: $mode}"
		done
		echo "  - src: /opt/mixmusic/mixmusic"
		echo "    dst: /usr/bin/mixmusic"
		echo "    type: symlink"
	} >"$config"
	cat "$config"
	go run "$nfpm" package --config "$config" --packager rpm --target "$out/mixmusic-$v-linux-$arch.rpm"
	echo "made mixmusic-$v-linux-$arch.rpm"

	# AppImage: the app and the files beside it in usr/bin, and its entry
	# and icon at the top, as AppImages keep them.
	appdir="$work/AppDir-$arch"
	mkdir -p "$appdir/usr/bin" "$appdir/usr/share"
	cp -a "$root/opt/mixmusic/." "$appdir/usr/bin/"
	chmod 0755 "$appdir/usr/bin/mixmusic"
	cp -a "$root/usr/share/icons" "$appdir/usr/share/"
	cp "$dir"mixmusic.desktop "$appdir/mixmusic.desktop"
	cp "$root/usr/share/icons/hicolor/256x256/apps/mixmusic.png" "$appdir/mixmusic.png"
	cat >"$appdir/AppRun" <<'EOF'
#!/bin/sh
here="$(dirname "$(readlink -f "$0")")"
exec "$here/usr/bin/mixmusic" "$@"
EOF
	chmod +x "$appdir/AppRun"
	case "$arch" in
	amd64) appimage_arch=x86_64 ;;
	arm64) appimage_arch=aarch64 ;;
	*)
		echo "package.sh: no AppImage runtime for $arch" >&2
		exit 1
		;;
	esac
	fetch "$runtime_url/runtime-$appimage_arch" "${runtime_sha256[$appimage_arch]}" "$work/runtime-$appimage_arch"
	# The tool is an AppImage itself; runners have no FUSE to mount it.
	APPIMAGE_EXTRACT_AND_RUN=1 ARCH="$appimage_arch" "$work/appimagetool" --no-appstream \
		--runtime-file "$work/runtime-$appimage_arch" "$appdir" "$out/mixmusic-$v-linux-$arch.AppImage"
	echo "made mixmusic-$v-linux-$arch.AppImage"
done
