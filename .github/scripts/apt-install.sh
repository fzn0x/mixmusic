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
# keeping what already came. When all three fail, it tries them all again,
# a little later each time, for up to an hour ($APT_BUDGET seconds): a
# mirror that is down for a while doesn't fail the job. Only the download is
# timed, never dpkg's install.
set -euo pipefail

apt=(apt-get -y -o Acquire::Retries=3 -o Acquire::ForceIPv4=true)
budget=${APT_BUDGET:-3600}
start=$SECONDS

# The runner's own lists, kept to start each round from them.
lists=()
for f in /etc/apt/apt-mirrors.txt /etc/apt/sources.list /etc/apt/sources.list.d/*; do
	if [ -f "$f" ]; then
		lists+=("$f")
	fi
done
kept="$(mktemp -d)"
for i in "${!lists[@]}"; do
	cp "${lists[$i]}" "$kept/$i"
done

restore() {
	for i in "${!lists[@]}"; do
		sudo cp "$kept/$i" "${lists[$i]}"
	done
}

# archive rewrites where apt's lists take the archive from, with sed's
# expression $1.
archive() {
	for f in "${lists[@]}"; do
		sudo sed -i -E "$1" "$f"
	done
}

fetch() {
	sudo timeout -k 10 90 "${apt[@]}" update &&
		sudo timeout -k 10 "$1" "${apt[@]}" install --download-only "${@:2}"
}

# round tries the runner's mirror, Ubuntu's archive, then kernel.org's
# mirror of it, each over HTTPS.
round() {
	restore
	archive 's#http://((azure\.)?archive|security)\.ubuntu\.com#https://\1.ubuntu.com#g'
	fetch 180 "$@" && return
	echo "::warning::Ubuntu's mirror on the runner was too slow or out of reach: going on from archive.ubuntu.com."
	archive 's#azure\.archive\.ubuntu\.com#archive.ubuntu.com#g'
	fetch 420 "$@" && return
	echo "::warning::archive.ubuntu.com was out of reach too: going on from kernel.org's mirror of it."
	archive 's#https?://(archive|security)\.ubuntu\.com/ubuntu#https://mirrors.edge.kernel.org/ubuntu#g'
	fetch 420 "$@"
}

wait=${APT_WAIT:-60}
until round "$@"; do
	spent=$((SECONDS - start))
	if [ $((spent + wait)) -ge "$budget" ]; then
		echo "::error::No mirror of Ubuntu's archive answered in $((spent / 60)) minutes."
		exit 1
	fi
	echo "::warning::No mirror answered: trying them all again in $wait s ($((spent / 60)) of $((budget / 60)) minutes spent)."
	sleep "$wait"
	wait=$((wait * 2))
	[ "$wait" -gt 600 ] && wait=600
done
sudo "${apt[@]}" install "$@"
