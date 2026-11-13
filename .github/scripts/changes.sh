#!/usr/bin/env bash
# Tells ci.yml's jobs what a pull request changes, as the step outputs of
# its "What changed" job, so that one that changes only the website, the
# docs or the changelog's fragments runs the checks of those, and one that
# changes only files nothing reads, as CONTRIBUTORS.md, runs none:
#
#   go        full: every Go check, as before. some: on Linux, the tests of
#             `packages` alone, and the changelog's check. none: nothing.
#   packages  the Go packages whose tests read what changed, for `some`.
#   site      true: the website builds (the Website job).
#
# Pushes to main and merge queues test everything, and so does a pull
# request this can't read the files of, or that changes any other file: a
# change to Go code, the plugins, the builds or CI itself.
#
#   - id: changes
#     run: bash .github/scripts/changes.sh
#     env:
#       GH_TOKEN: ${{ github.token }}
#       PR: ${{ github.event.pull_request.number }}
set -uo pipefail

out() {
	echo "$1=$2" >>"${GITHUB_OUTPUT:-/dev/null}"
	echo "$1=$2"
}

# all tests everything: why, then the outputs.
all() {
	echo "Everything runs: $1."
	out go full
	out packages ""
	out site true
	exit 0
}

[ "${GITHUB_EVENT_NAME:-}" = pull_request ] || all "a ${GITHUB_EVENT_NAME:-run} that isn't a pull request's"
[ -n "${PR:-}" ] || all "no pull request number"

# The files changed, by their new name and, renamed, their old one.
files="$(gh api --paginate "repos/$GITHUB_REPOSITORY/pulls/$PR/files" \
	-q '.[] | .filename, (.previous_filename // empty)')" || all "the pull request's files couldn't be read"
[ -n "$files" ] || all "the pull request lists no file"
# GitHub lists 3000 files of a pull request at most.
[ "$(wc -l <<<"$files")" -lt 3000 ] || all "the pull request changes 3000 files or more"

site=false docs=false changelog=false
while IFS= read -r f; do
	case "$f" in
	# tools/sitefinish's tests read site/i18n.
	site/* | tools/sitefinish/*) site=true ;;
	# tools/sitedocs makes the docs' pages of these; internal/colorfile's
	# tests read docs/themes.md.
	docs/* | tools/sitedocs/* | README*.md | CONTRIBUTING.md | CODE_OF_CONDUCT.md | SECURITY.md)
		site=true
		docs=true
		;;
	changelog.d/*) changelog=true ;;
	# Nothing builds, tests or publishes these.
	CONTRIBUTORS.md | LICENSE* | .github/ISSUE_TEMPLATE/*) ;;
	*) all "$f changed" ;;
	esac
done <<<"$files"

packages=""
$site && packages+=" ./tools/sitefinish ./tools/sitedocs"
$docs && packages+=" ./internal/colorfile"
$changelog && packages+=" ./tools/changelog"
packages="${packages# }"
if [ -n "$packages" ]; then
	echo "Only the tests of what changed run: $packages."
	out go some
else
	echo "Nothing that CI checks changed."
	out go none
fi
out packages "$packages"
out site "$site"
