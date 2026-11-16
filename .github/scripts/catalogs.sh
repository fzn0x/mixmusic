#!/usr/bin/env bash
# Checks the catalogs of the interface's words, locales/*.json, with MyGo's
# own checks (a message that doesn't parse, an argument a translation has
# and English doesn't), and that locales/catalog_gen.go is what MyGo writes
# of them: a catalog changed without `mygo generate` fails. The tests of
# locales check the rest (sorted keys, unused messages, missing
# translations, plural cases).
#
# It runs MyGo's CLI by Go, from the module's cache, and on a copy of the
# catalogs beside a mygo.json of only them: mygo.config.ts would need Bun,
# and its bindings would build and start the app.
set -euo pipefail

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cp -R locales "$work/"
rm "$work/locales/catalog_gen.go"
echo '{"name": "MixMusic", "i18n": {"dir": "locales", "source": "en"}}' >"$work/mygo.json"

go run github.com/egoist/mygo/cmd/mygo i18n check "$work"
# It warns that the copy has no go.mod, which only the import check needs.
go run github.com/egoist/mygo/cmd/mygo generate "$work"
if ! diff -u locales/catalog_gen.go "$work/locales/catalog_gen.go" >"$work/diff"; then
  head -40 "$work/diff"
  echo "locales/catalog_gen.go is stale: run \`bunx mygo generate\` and commit it." >&2
  exit 1
fi
echo "The catalogs check, and locales/catalog_gen.go is up to date."
