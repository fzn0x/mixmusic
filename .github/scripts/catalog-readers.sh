#!/usr/bin/env bash
# Lists the Go packages whose tests read the catalogs of the interface's
# words: package locales, and every package that imports it, itself or
# through another package, its tests' imports counted. ci.yml tests these
# alone for a pull request that changes translations and nothing else
# (.github/scripts/changes.sh).
set -euo pipefail

locales="$(go list -m)/locales"
{
	echo "$locales"
	# A package's test variant is "pkg [pkg.test]", and its test binary
	# "pkg.test": both name the package itself.
	go list -test -f '{{.ImportPath}} {{join .Deps " "}}' ./... |
		awk -v want="$locales" '{ for (i = 2; i <= NF; i++) if ($i == want) { print $1; next } }'
} | sed 's/\.test$//' | sort -u
