#!/usr/bin/env bash
# Adds a Recommends line to a .deb, which mygo build writes none of.
#
#   .github/scripts/deb-recommends.sh <file.deb> <package>[, <package>...]
#
# The package is rewritten in place: debian-binary, control.tar.gz and the
# data archive as it was, in that order, as dpkg wants them. Only ar and tar
# are used, as the build shell has no dpkg.
set -euo pipefail

deb="$(cd "$(dirname "${1:?usage: deb-recommends.sh <file.deb> <packages>}")" && pwd)/$(basename "$1")"
recommends="${2:?usage: deb-recommends.sh <file.deb> <packages>}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir "$work/in" "$work/control"
(cd "$work/in" && ar x "$deb")
controls=("$work"/in/control.tar.*)
datas=("$work"/in/data.tar.*)
if [ ${#controls[@]} -ne 1 ] || [ ${#datas[@]} -ne 1 ]; then
	echo "deb-recommends.sh: want one control.tar.* and one data.tar.* in $deb" >&2
	exit 1
fi
tar --force-local -xf "${controls[0]}" -C "$work/control"
control="$work/control/control"
if [ ! -s "$control" ]; then
	echo "deb-recommends.sh: no control file in $deb" >&2
	exit 1
fi

# A Recommends the package has already keeps its packages; a missing one
# goes at the end of the fields.
if grep -q '^Recommends:' "$control"; then
	sed -i "s/^Recommends:[[:space:]]*\(.*\)$/Recommends: \1, $recommends/" "$control"
else
	# The file ends with a newline; drop blank lines at its end first.
	sed -i -e :a -e '/^\n*$/{$d;N;ba' -e '}' "$control"
	printf 'Recommends: %s\n' "$recommends" >>"$control"
fi

(cd "$work/control" && tar --owner=0 --group=0 --numeric-owner -czf "$work/control.tar.gz" .)
rm -f "$deb"
(cd "$work" && cp in/debian-binary debian-binary && cp "${datas[0]}" . &&
	ar rc "$deb" debian-binary control.tar.gz "$(basename "${datas[0]}")")
echo "deb-recommends.sh: $(basename "$deb") recommends $recommends"
