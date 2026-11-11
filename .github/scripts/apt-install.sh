#!/usr/bin/env bash
# Installs packages from Ubuntu's archive on a runner, with apt-get
# install's arguments:
#
#   bash .github/scripts/apt-install.sh --no-install-recommends pulseaudio
#
# apt asks the archive over HTTPS and over IPv4. On 2026-11-11 the runners
# couldn't reach Ubuntu's mirrors over plain HTTP for hours, every index
# timing out, while HTTPS answered; and they have no route for IPv6.
#
# The runners' mirror, azure.archive.ubuntu.com, sometimes sends a few
# dozen kilobytes a second: on 2026-11-06 it took longer than 10 minutes
# for 74 MB, and a job waited on it until it was cancelled. So the packages
# download first, for 3 minutes at most; if that runs out, the download
# goes on from Ubuntu's own archive, then from kernel.org's mirror of it,
# keeping what already came. Only the download is timed, never dpkg's
# install.
set -euo pipefail

apt=(apt-get -y -o Acquire::Retries=3 -o Acquire::ForceIPv4=true)

# archive rewrites where apt's lists take the archive from, with sed's
# expression $1: the runner's mirror list and the sources.
archive() {
	for f in /etc/apt/apt-mirrors.txt /etc/apt/sources.list /etc/apt/sources.list.d/*; do
		if [ -f "$f" ]; then
			sudo sed -i -E "$1" "$f"
		fi
	done
}

fetch() {
	sudo timeout -k 10 90 "${apt[@]}" update &&
		sudo timeout -k 10 "$1" "${apt[@]}" install --download-only "${@:2}"
}

archive 's#http://((azure\.)?archive|security)\.ubuntu\.com#https://\1.ubuntu.com#g'
if ! fetch 180 "$@"; then
	echo "::warning::Ubuntu's mirror on the runner was too slow or out of reach: going on from archive.ubuntu.com."
	archive 's#azure\.archive\.ubuntu\.com#archive.ubuntu.com#g'
	if ! fetch 420 "$@"; then
		echo "::warning::archive.ubuntu.com was out of reach too: going on from kernel.org's mirror of it."
		archive 's#https?://(archive|security)\.ubuntu\.com/ubuntu#https://mirrors.edge.kernel.org/ubuntu#g'
		fetch 420 "$@"
	fi
fi
sudo "${apt[@]}" install "$@"
