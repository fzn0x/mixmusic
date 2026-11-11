#!/usr/bin/env bash
# Installs packages from Ubuntu's archive on a runner, with apt-get
# install's arguments:
#
#   bash .github/scripts/apt-install.sh --no-install-recommends pulseaudio
#
# It asks the archive over HTTPS and over IPv4: on 2026-11-11 the runners
# couldn't reach Ubuntu's mirrors over plain HTTP for hours, every index
# timing out, while HTTPS answered; and they have no route for IPv6.
#
# It tries three mirrors, each only when it answers a quick look at its
# index: the runners' own, azure.archive.ubuntu.com, then Ubuntu's own
# archive, then kernel.org's mirror of it. The runners' mirror sometimes
# sends a few dozen kilobytes a second (on 2026-11-06 it took longer than 10
# minutes for 74 MB), so a download that runs too long goes on from the next
# mirror, keeping what already came. Only the download is timed, never
# dpkg's install.
#
# When no mirror can be had for $APT_BUDGET seconds (8 minutes), Ubuntu's
# archive is out of reach: rather than fail the job for it, it installs
# nothing, says so in a warning, and exits 0, leaving the marker
# $RUNNER_TEMP/apt-skipped and APT_SKIPPED=1 in the job's environment. The
# steps that need the packages check either and skip themselves, and a
# later apt-install.sh in the job skips at once.
set -euo pipefail

skipped="${RUNNER_TEMP:-/tmp}/apt-skipped"
if [ -f "$skipped" ]; then
	echo "::warning::Ubuntu's archive was out of reach earlier in this job: not installing $*."
	exit 0
fi

apt=(apt-get -y -o Acquire::Retries=3 -o Acquire::ForceIPv4=true)
budget=${APT_BUDGET:-480}
start=$SECONDS
codename="$(. /etc/os-release 2>/dev/null && echo "${VERSION_CODENAME:-}" || true)"
mirrors=(https://azure.archive.ubuntu.com/ubuntu https://archive.ubuntu.com/ubuntu https://mirrors.edge.kernel.org/ubuntu)

# The runner's own lists, kept to point them at each mirror in turn.
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

# use points apt's lists at the mirror $1, for the archive and its security
# pocket, which every mirror carries.
use() {
	for i in "${!lists[@]}"; do
		sudo cp "$kept/$i" "${lists[$i]}"
		sudo sed -i -E "s#https?://((azure\.)?archive|security)\.ubuntu\.com/ubuntu/?#$1/#g" "${lists[$i]}"
	done
}

# answers says whether the mirror $1 serves the release's index.
answers() {
	curl -fsS -4 -m 20 -o /dev/null "$1/dists/$codename/InRelease"
}

# left is how much of the budget is left, in seconds.
left() {
	echo $((budget - (SECONDS - start)))
}

wait=15
while :; do
	for m in "${mirrors[@]}"; do
		t=$(left)
		[ "$t" -lt 60 ] && break 2
		if ! answers "$m"; then
			echo "::warning::$m didn't answer."
			continue
		fi
		use "$m"
		[ "$t" -gt 420 ] && t=420
		if sudo timeout -k 10 90 "${apt[@]}" update &&
			sudo timeout -k 10 "$t" "${apt[@]}" install --download-only "$@"; then
			sudo "${apt[@]}" install "$@"
			exit 0
		fi
		echo "::warning::The download from $m failed or ran too long: going on from the next mirror."
	done
	[ "$(left)" -lt $((wait + 60)) ] && break
	echo "::warning::No mirror of Ubuntu's archive could be had: trying again in $wait s."
	sleep "$wait"
	wait=$((wait * 2))
	[ "$wait" -gt 120 ] && wait=120
done

echo "::warning::Ubuntu's archive was out of reach for $(((SECONDS - start) / 60)) minutes: not installing $*, and skipping the checks that need it."
touch "$skipped"
if [ -n "${GITHUB_ENV:-}" ]; then
	echo "APT_SKIPPED=1" >>"$GITHUB_ENV"
fi
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
	echo "Ubuntu's archive was out of reach: the checks that need $* were skipped." >>"$GITHUB_STEP_SUMMARY"
fi
exit 0
