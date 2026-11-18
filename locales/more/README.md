# Languages added at start

Each file here is MixMusic in one more language: the messages of
[../en.json](../en.json), translated. MixMusic adds them to its catalog as
it starts ([../more.go](../more.go)), so the Language setting lists each,
named in itself, and the computer's language picks it.

MyGo's `generate`, `i18n check` and `build` read the JSON files at the root
of `locales/` alone, and not this folder. So:

- A language added here touches no other file. Every new language lands
  here first: the doc comment of `../catalog_gen.go` lists each language at
  the root, so two pull requests that each added one there would conflict.
- The languages the Windows installer can be in have moved to the root of
  `locales/`, as `../de.json` (see below), and are fixed there, in the same
  layout.
- The Windows installer, the languages macOS lists for the app and the
  Linux desktop entry don't know these languages: there MixMusic is named
  in English. MyGo 0.8.1's Windows build fails for a language its
  installer's table lacks, such as Hindi, Bengali, Urdu, Hausa, Amharic,
  Tamil, Telugu, Swahili and Filipino, so those stay here.

[CONTRIBUTING.md](../../CONTRIBUTING.md#adding-or-fixing-a-language)
tells how to add or fix a language.

## A file

`<tag>.json`, named after the language's BCP 47 tag in its canonical
form: `hi.json`, `pt-BR.json`, `zh-Hant.json`, `fil.json`; not `pt_BR.json`,
`zh-TW.json` or `tl.json`. Not a language of `locales/` itself. UTF-8, in
en.json's layout:

```json
{
  "@@locale": "hi",
  "@@x-machine-translated": true,
  "aboutContribute": "योगदान करें",
  "collectionDeleteFailed": "{title} को हटाया नहीं जा सका: {error}",
  "mygo.cancel": "रद्द करें",
  "mygo.copy": "कॉपी करें"
}
```

- `@@locale` comes first and is the file's tag.
- `@@x-machine-translated: true` comes next while a machine's translation
  hasn't been read through by a speaker of the language. Settings › About
  says so, in the language, with a link to CONTRIBUTING. The speaker who
  has read and fixed the whole file removes the line.
- `@@x-translators`, a list of names, comes next when people translated
  the file or fixed it: `"@@x-translators": ["Ada", "Grace"]`. Settings ›
  About names them, in the language.
- Then the messages, each a key of en.json, sorted bytewise as there, and
  without en.json's `@key` descriptions. A message keeps the English one's
  placeholders (`{title}`) and select cases, and has the plural cases of its
  own language (`one`, `few`, `many` and `other` in Russian) with English's
  exact ones (`=0`). A message the file lacks shows in English.
- `mygo.<key>` messages say MyGo's own words, those of its controls and
  menus, with the keys of MyGo's catalog (`internal/l10n/en.json` in the
  MyGo module of go.mod). For `ja`, `zh-Hans` and `zh-Hant`, take MyGo's
  own translations (`locales/<tag>/<tag>.json` in that module) rather than
  a machine's. The app links MyGo's packages of those only for the
  languages at the root of `locales/`, so here, MyGo's words a catalog
  lacks show in English.

`bash .github/scripts/catalogs.sh` runs MyGo's checks of each file beside
en.json, and `go test ./locales` MixMusic's.

## Moving a language to locales/

A language the Windows installer can be in (MyGo's table:
`nsisLanguages` in MyGo's `cmd/mygo/languages.go`) moves to the root after
it lands here, in a pull request of its own, so that the installer, macOS
and Linux name MixMusic in it. Those of 2026 moved together, in #564:

1. `git mv locales/more/<tag>.json locales/<tag>.json` for each.
2. Give each catalog `mygo.appName`, MixMusic's name in it, and add
   `resources/darwin/<tag>.lproj/InfoPlist.strings` with the reasons of
   `mygo.config.ts`'s `infoPlist`: `go test ./locales` asks for both.
3. Generate `locales/catalog_gen.go` again, as `.github/scripts/catalogs.sh`
   runs MyGo, or with `bunx mygo generate`. For a language MyGo has its own
   translation of, it imports that package, which changes `vendorHash` in
   `nix/package.nix` (`nix build` tells the new one) and
   `packaging/flathub/go-modules.json` (`go run ./tools/flathub`).
4. Run `bash .github/scripts/catalogs.sh` and `go test ./locales`.

Nothing else changes: the app reads both, and `@@x-machine-translated`
means the same at the root.
