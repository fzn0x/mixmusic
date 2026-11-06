#!/usr/bin/env bash
# Installs packages from Ubuntu's archive on a runner, with apt-get
# install's arguments:
#
#   bash .github/scripts/apt-install.sh --no-install-recommends pulseaudio
#
# The runners' mirror, azure.archive.ubuntu.com, sometimes sends a few
# dozen kilobytes a second: on 2026-11-06 it took longer than 10 minutes
# for 74 MB, and a job waited on it until it was cancelled. So the packages
# download first, for 3 minutes at most; if that runs out, the download
# goes on from Ubuntu's own archive, keeping what already came. Only the
# download is timed, never dpkg's install.
set -euo pipefail

apt=(apt-get -y -o Acquire::Retries=3)

fetch() {
	sudo timeout -k 10 90 "${apt[@]}" update &&
		sudo timeout -k 10 "$1" "${apt[@]}" install --download-only "${@:2}"
}

if ! fetch 180 "$@"; then
	echo "::warning::Ubuntu's mirror on the runner was too slow: going on from archive.ubuntu.com."
	for f in /etc/apt/apt-mirrors.txt /etc/apt/sources.list /etc/apt/sources.list.d/*; do
		if [ -f "$f" ]; then
			sudo sed -i 's|azure\.archive\.ubuntu\.com|archive.ubuntu.com|g' "$f"
		fi
	done
	fetch 420 "$@"
fi
sudo "${apt[@]}" install "$@"
