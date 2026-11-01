# Plugins

A plugin adds to MixMusic: a page, a command, an item in the menu of every
track, a widget on the now playing page. It is a folder holding a manifest
and an ES module, written in TypeScript and compiled to the JavaScript
MixMusic runs:

```
my-plugin/
├── plugin.json
├── index.ts    the plugin
└── index.js    what it compiles to, which MixMusic runs
```

```json
{
  "id": "my-plugin",
  "name": "My Plugin",
  "version": "1.0.0",
  "author": "Your Name",
  "description": "What it does, in one sentence.",
  "main": "index.js",
  "api": 2,
  "permissions": ["playback"],
  "files": ["plugin.json", "index.js"]
}
```

`api` is the version of the plugin API the plugin is written for: 2, that
of MixMusic's own interface. MixMusic runs plugins of `api` 2 only: those
written for the web interface of MixMusic 0.4 and before need their
author to move them to the API of this page.

`permissions` tell users what the plugin does that they should know of
before they enable it: `playback` (controls the player), `network` (talks to
the internet), `notifications` (shows desktop notifications), `storage`
(keeps values). They are shown, not enforced: every plugin can use the
whole API, which is why MixMusic asks the user to trust a plugin before
enabling it.

## The module

The module's default export is called with the API when the user enables
the plugin, and may return a function MixMusic calls when it stops it:

```ts
import type { MixMusicAPI } from "./sdk"; // sdk/index.d.ts, copied beside it

export default function activate(api: MixMusicAPI) {
  api.commands.register({
    id: "shout",
    title: "Shout what plays",
    shortcut: "Mod+Shift+S",
    run: () => {
      const track = api.player.state().track;
      api.ui.toast(track ? `Now: ${track.title}` : "Nothing plays");
    },
  });

  const off = api.player.on("trackchange", () => {
    const track = api.player.state().track;
    if (track) console.log("playing", track.title, "from", track.source);
  });
  return off;
}
```

Everything a plugin adds through the API (pages, sidebar items, commands,
track actions, widgets and listeners) is removed for it when it stops, and
its timers stop: return a function only for what else you started.

A plugin runs in a JavaScript engine of MixMusic's own, not in a browser:
the language is all there, with `async` functions, classes and the rest, as
are `console`, `setTimeout`, `setInterval`, `queueMicrotask`,
`performance.now()` and `fetch`, but not the DOM, `localStorage` or
`window`. The module may import the plugin's own files, as
`import { helper } from "./helper.js"`, and nothing else. `console.log`
writes to MixMusic's log, `mixmusic.log` in its logs folder.

### From TypeScript to JavaScript

MixMusic runs JavaScript, so a plugin ships what its TypeScript compiles
to: `"main"` and `"files"` name `index.js`, and `index.ts` stays with its
source, as the registry leaves it out. Any compiler that keeps the module an
ES module does; with TypeScript's own:

```sh
tsc --target ES2022 --module ESNext --moduleResolution bundler --strict index.ts
```

The plugins of this repository's `community/plugins` are written so, and
keep only their TypeScript: `bun run build:plugins` compiles them, as it
does the plugin API's own `internal/plugins/api.ts`, and the Registry
workflow publishes their JavaScript to the registry branch, where the
marketplace downloads it. A plugin of your own may be written in
JavaScript straight away too: the same module, without the types.

## The API

[sdk/index.d.ts](../sdk/index.d.ts) types the whole API; copy it next to your
plugin and import its types, as above, or in a JavaScript plugin point your
editor at it with a JSDoc `@param {import("./sdk").MixMusicAPI} api`.

| | |
|---|---|
| `api.player` | the state of the player (`state()`), the transport, the queue (`playTracks`, `playNext`, `enqueue`), events (`on("trackchange" \| "statechange" \| "progress" \| "queuechange")`) and the `analyser()` of what plays, which has the reading methods of the web's AnalyserNode: MixMusic's own player, and on Windows Spotify too, in MixMusic or the Spotify app on this computer |
| `api.catalog` | search both services, and read albums, artists, playlists, radios and lyrics |
| `api.library` | liked songs and the user's mixes |
| `api.ui` | `toast`, `navigate`, `theme`, `addSidebarItem`, `addPage`, `addTrackAction`, `addWidget`, and the elements pages show |
| `api.commands` | `register` a command of the command palette, with an optional shortcut |
| `api.settings` | the user's settings, read only |
| `api.storage` | strings, or JSON, the plugin keeps between runs, up to 1 MB |
| `api.notify`, `api.openExternal` | a desktop notification; a web page in the browser |

### Pages and widgets

A page and a widget show elements, which their `render` function sets on
the view it gets, and sets again when they change; it may return a
function MixMusic calls when the page closes:

```js
api.ui.addSidebarItem({ id: "home", label: "my plugin", page: "home", icon: svgString });
api.ui.addPage({
  name: "home",
  title: "my plugin",
  render(view) {
    const { text, button, row } = api.ui;
    let count = 0;
    const draw = () =>
      view.set(
        text(`Clicked ${count} times.`, { size: "subtitle" }),
        row([button("click", () => (count++, draw()), { primary: true }), button("search lofi", () => api.ui.navigate("/search?q=lofi"))]),
      );
    draw();
    return () => {}; // runs when the page closes
  },
});
```

The page lives at `/plugin/<plugin id>/<name>` and gets MixMusic's giant
title; a widget shows on the now playing page, under the lyrics, with its
title. Elements take the theme's colours and type, as MixMusic's own pages
do. `icon` is the SVG of a 24×24 icon drawn in `currentColor`.

| element | |
|---|---|
| `text(text, { tone, size })` | a paragraph; `tone` is `"muted"`, `"accent"` or `"danger"`, `size` `"small"`, `"subtitle"` or `"title"` |
| `heading(text)` | a section's title |
| `button(label, onClick, { primary })` | a button, outlined, or filled with the accent |
| `link(label, onClick or path)` | a link that does something, or shows a page |
| `tiles([...])`, `tile(title, onClick, { tone, caption })` | blocks of the theme's four tile colours, `tone` 1 to 4 |
| `row([...], { gap })`, `column([...], { gap })` | elements side by side, or one under the other |
| `toggle(label, on, onChange)` | a switch: it shows `on`, and `onChange` gets what the user chose, which shows once the plugin sets it |
| `input({ placeholder, value, onSubmit })` | a text field: `onSubmit` gets its text when the user presses Enter |
| `image(url, { size, round })` | artwork |
| `spacer(size)` | room |
| `canvas({ width, height, animate, draw })` | a drawing |

Elements are plain objects, such as `{ type: "text", text: "hi" }`, which a
plugin may write itself; `null` and `false` show nothing, and arrays are
flattened.

### Drawing

A canvas calls `draw(ctx, width, height)` to draw its frame, at every frame
while `animate` is set, unless the computer asks for less motion; `ctx` has
a part of the web's `CanvasRenderingContext2D`: `save`, `restore`,
`translate`, `rotate`, `scale`, `transform`, `setTransform`,
`resetTransform`, `beginPath`, `moveTo`, `lineTo`, `arc`, `rect`,
`quadraticCurveTo`, `bezierCurveTo`, `closePath`, `stroke`, `fill`,
`fillRect`, `strokeRect` and `clearRect`, and `fillStyle`, `strokeStyle`,
`lineWidth`, `lineCap` and `globalAlpha`. Its sizes are in points, as the
interface's, and `api.ui.theme()` gives the colours of the theme:

```js
api.ui.addWidget({
  id: "pulse",
  title: "pulse",
  render(view) {
    const levels = new Uint8Array(128);
    view.set(
      api.ui.canvas({
        height: 120,
        animate: true,
        draw(ctx, width, height) {
          api.player.analyser().getByteFrequencyData(levels);
          ctx.fillStyle = api.ui.theme().accent;
          ctx.fillRect(0, height * (1 - levels[4] / 255), width, height);
        },
      }),
    );
  },
});
```

### Tracks

Tracks have an `id` whose prefix tells their service: `yt:` for YouTube
Music, `sp:` for Spotify. A track's `source` says the same: `youtube` or
`spotify`.

```js
api.ui.addTrackAction({
  id: "share",
  label: "Search the web for it",
  run: (track) => api.openExternal(`https://duckduckgo.com/?q=${encodeURIComponent(`${track.title} ${track.artists[0]?.name ?? ""}`)}`),
});
```

## Try it

1. In Settings › Plugins, choose **open plugins folder** and put your plugin's
   folder there, named after its `id`.
2. Choose **reload**, then enable it. A plugin that fails to start says why,
   in a notice and in the list; reload tries it again.

## Share it

[community/README.md](../community/README.md) tells how to add your plugin to
the community registry, from which everyone can install it.
