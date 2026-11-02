# Themes

A theme sets the colors, the typefaces and the corners of MixMusic, which
draws everything with them. A theme is a folder:

```
my-theme/
├── theme.json   the manifest: colors, fonts, radius
└── preview.png  optional: an image for the marketplace
```

## Make one

1. In Settings › Appearance, choose **open themes folder**.
2. Copy a theme there, for example
   [community/themes/paper-mint](../community/themes/paper-mint), into a folder
   named after its new `id`.
3. Change `id`, `name` and `author`, then the colors.
4. Run **Reload add-ons** from the command palette (<kbd>Ctrl</kbd> <kbd>K</kbd>)
   and pick the theme in Settings › Appearance. Themes that fail to load are
   listed there, with the reason.

Every change to `theme.json` shows after a reload.

## theme.json

```json
{
  "id": "my-theme",
  "name": "My Theme",
  "version": "1.0.0",
  "author": "Your Name",
  "description": "What it looks like, in one sentence.",
  "scheme": "dark",
  "colors": {
    "bg": "#000000",
    "surface": "#0b0b0b",
    "surface-2": "#181818",
    "surface-3": "#272727",
    "border": "rgba(255, 255, 255, 0.16)",
    "text": "#ffffff",
    "text-muted": "#a3a3a3",
    "accent": "#00bde3",
    "accent-text": "#000000",
    "danger": "#ff5a4f",
    "success": "#a4e600",
    "youtube": "#ff4040",
    "spotify": "#1ed760",
    "tile-1": "#00bde3",
    "tile-2": "#e6007e",
    "tile-3": "#a4e600",
    "tile-4": "#ffb000",
    "tile-text": "#000000"
  },
  "fonts": {
    "body": "Selawik, 'Segoe UI', system-ui, sans-serif",
    "display": "Selawik, 'Segoe UI', system-ui, sans-serif"
  },
  "radius": "0px",
  "files": ["theme.json"]
}
```

| Field | |
|---|---|
| `id` | lowercase letters, digits and dashes; the folder's name |
| `version` | a semantic version, as `1.2.0`; raise it with every change you publish |
| `scheme` | `dark` or `light`: the controls MixMusic draws, such as switches and sliders, follow it |
| `colors` | every color below, as `#rgb`, `#rrggbb`, `#rrggbbaa`, `rgb()`, `rgba()`, `hsl()`, `oklch()` or `transparent` |
| `fonts` | font-family lists, of which MixMusic uses the first the computer has: `body` for text, `display` for titles and the large lowercase words (the body's when left out) |
| `radius` | the corner radius of tiles and cards, as `12px`; smaller things round by 60% of it |
| `preview` | an image of the theme, for the marketplace |
| `files` | every file of the theme: the registry installs exactly these |

### Colors

| Color | Used for |
|---|---|
| `bg` | the window's ground |
| `surface`, `surface-2`, `surface-3` | panels, then hovered and pressed things on them |
| `border` | hairlines between rows and panels |
| `text`, `text-muted` | ink, and secondary ink; outlined controls are drawn in `text` |
| `accent`, `accent-text` | everything live: the playing row, the selection, the active page, primary buttons, and ink on them |
| `danger`, `success` | errors and confirmations |
| `youtube`, `spotify` | the marks telling where a track comes from |
| `tile-1` … `tile-4`, `tile-text` | the flat tiles of the home page and the stats, and ink on them |

Keep text readable: `text-muted` on `bg`, `accent-text` on `accent` and
`tile-text` on every tile need a contrast of at least 4.5:1. When the user
picks an accent of their own, or lets it follow the artwork, MixMusic picks
the accent's ink itself, and keeps the accent readable on your `bg`.

### Fonts

MixMusic ships Selawik, and Quicksand, Mochi's round face; other families
must be on the user's computer, so end each list with a family every
system has: `system-ui` or `sans-serif` for the system's font, `monospace`
for its monospaced one.

## Effects

The themes MixMusic ships with add effects of their own, which MixMusic
draws and the user tunes in Settings › Appearance: Mix's tile textures,
Aurora's drifting lights and frosted glass, Sakura's petals, Terminal's
scanlines and glow, Greenroom's and Glasshouse's grounds in the colour of
the cover playing, Anime's speed lines and screentone, and Mochi's rising
sparkles. A theme of yours sets the tokens above, and MixMusic draws
everything else with them.

Themes made for the web interface of MixMusic 0.4 and before still load:
MixMusic reads their colors, fonts and radius, and ignores their stylesheet
and their options.

## Colours from a file

pywal and matugen make a palette from your wallpaper, and write it to a
file each time you change the wallpaper. MixMusic can wear those colours,
and follow them as they change. In Settings › Appearance › colour and type,
set **Colours from a file** to pywal, matugen, or a file of your own.
Under it, MixMusic shows the file it reads and when it last read it, or
why it can't.

The file's colours replace the theme's. The theme keeps its type, corners
and effects, so Mochi's round face or Terminal's scanlines can wear the
wallpaper's colours. The theme cards keep showing each theme's own colours.

- **The accent** works as it does on any theme. While Follow the artwork is
  on, the cover playing sets it; otherwise your Accent colour does, if you
  picked one, then the file's. Follow the artwork is on by default: turn it
  off to keep the file's accent while music plays.
- **Light or dark.** A file with one set of colours, as pywal's, decides
  whether MixMusic is light or dark, title bar included. With Follow the
  computer on, MixMusic wears the theme you chose for the file's mode. A
  file with both, as matugen's, gives its light colours to a light theme
  and its dark ones to a dark theme, so Follow the computer keeps working.
- **Readable text.** Colours made from a wallpaper can be anything, so
  MixMusic changes the ones that don't read, keeping their hue: text,
  muted text and danger get 4.5:1 on the ground, the ink on the accent and
  on every tile 4.5:1, and the accent, success and the YouTube Music and
  Spotify marks 3:1. Your file's exact colours may differ a little on
  screen for that.
- **Glass.** Where a theme's surfaces show its ground through, as Aurora's
  and Glasshouse's, they stay glass in the file's colours.

### pywal

MixMusic reads `colors.json` where pywal writes it: in `$PYWAL_CACHE_DIR`
if it is set, else in `$XDG_CACHE_HOME/wal`, else in `~/.cache/wal`
(`%USERPROFILE%\.cache\wal` on Windows). pywal 3.3.0 and older always
write to `~/.cache/wal`, so MixMusic reads whichever of the two has a
`colors.json`. There's nothing to set up: run `wal`, and MixMusic follows.

If pywal writes somewhere MixMusic doesn't look, as when you set
`PYWAL_CACHE_DIR` only in your shell, which apps started from the desktop
don't see, choose that `colors.json` as a file of your own.

### matugen

MixMusic reads `matugen.json` in its data folder: `~/.config/MixMusic` on
Linux, `~/Library/Application Support/MixMusic` on macOS, and
`%AppData%\MixMusic` on Windows. Give matugen MixMusic's template, which
writes MixMusic's colours for dark and light (**copy the template** in
Settings copies it):

```json
{
  "dark":
  {
    "bg": "{{ colors.surface.dark.hex }}",
    "surface": "{{ colors.surface_container_low.dark.hex }}",
    "surface-2": "{{ colors.surface_container_high.dark.hex }}",
    "surface-3": "{{ colors.surface_container_highest.dark.hex }}",
    "border": "{{ colors.outline_variant.dark.hex }}",
    "text": "{{ colors.on_surface.dark.hex }}",
    "text-muted": "{{ colors.on_surface_variant.dark.hex }}",
    "accent": "{{ colors.primary.dark.hex }}",
    "accent-text": "{{ colors.on_primary.dark.hex }}",
    "danger": "{{ colors.error.dark.hex }}",
    "tile-1": "{{ colors.primary.dark.hex }}",
    "tile-2": "{{ colors.tertiary.dark.hex }}",
    "tile-3": "{{ colors.secondary.dark.hex }}",
    "tile-4": "{{ colors.source_color.dark.hex }}",
    "tile-text": "{{ colors.on_primary.dark.hex }}"
  },
  "light":
  {
    "bg": "{{ colors.surface.light.hex }}",
    "surface": "{{ colors.surface_container_low.light.hex }}",
    "surface-2": "{{ colors.surface_container_high.light.hex }}",
    "surface-3": "{{ colors.surface_container_highest.light.hex }}",
    "border": "{{ colors.outline_variant.light.hex }}",
    "text": "{{ colors.on_surface.light.hex }}",
    "text-muted": "{{ colors.on_surface_variant.light.hex }}",
    "accent": "{{ colors.primary.light.hex }}",
    "accent-text": "{{ colors.on_primary.light.hex }}",
    "danger": "{{ colors.error.light.hex }}",
    "tile-1": "{{ colors.primary.light.hex }}",
    "tile-2": "{{ colors.tertiary.light.hex }}",
    "tile-3": "{{ colors.secondary.light.hex }}",
    "tile-4": "{{ colors.source_color.light.hex }}",
    "tile-text": "{{ colors.on_primary.light.hex }}"
  }
}
```

Each brace of it is on a line of its own: two braces together are how
matugen's expressions start and end. Save it as `templates/mixmusic.json`
beside matugen's `config.toml`, which is in `~/.config/matugen` on Linux,
`~/Library/Application Support/com.InioX.matugen` on macOS, and
`%AppData%\InioX\matugen\config` on Windows, and add to `config.toml`:

```toml
[templates.mixmusic]
input_path = "templates/mixmusic.json"
# Linux
output_path = "~/.config/MixMusic/matugen.json"
# macOS
# output_path = "~/Library/Application Support/MixMusic/matugen.json"
# Windows
# output_path = "~/AppData/Roaming/MixMusic/matugen.json"
```

matugen reads `input_path` from the folder `config.toml` is in. Then run
`matugen image <your wallpaper>`, and MixMusic follows.

### A file of your own

Choose any file that holds one of these, and MixMusic tells which it is by
what it holds:

- pywal's `colors.json`;
- what `matugen image <your wallpaper> --json hex` prints, saved to a file,
  as with `> ~/matugen-colors.json`: matugen 2.x, 3.x and 4.x print it in
  shapes of their own, and MixMusic reads all three;
- what MixMusic's matugen template writes;
- a `theme.json`, whose `colors` MixMusic takes, and its `scheme`; or a
  file of those colours alone, such as one a pywal template writes (pywal
  writes the templates of `~/.config/wal/templates` into its cache folder).

A path may start with `~`, your home folder; one that Windows Explorer's
**Copy as path** puts in quotes works too.

### How the colours map

| Token | From pywal | From matugen's JSON |
|---|---|---|
| `bg` | `special.background` | `surface` |
| `surface`, `surface-2`, `surface-3` | the background mixed 4.5%, 9% and 15% toward the foreground | `surface_container_low`, `surface_container_high`, `surface_container_highest` |
| `border` | the foreground at 16% | `outline_variant` |
| `text` | `special.foreground` | `on_surface` |
| `text-muted` | `color8` if it reads at 4.5:1, else the foreground mixed toward the background | `on_surface_variant` |
| `accent` | the most vivid of `color1` to `color6` | `primary` |
| `accent-text` | black or white | `on_primary` |
| `danger` | the theme's | `error` |
| `success`, `youtube`, `spotify` | the theme's | the theme's |
| `tile-1` to `tile-4` | the four most vivid of `color1` to `color6` | `primary`, `tertiary`, `secondary`, `source_color` |
| `tile-text` | black or white, whichever reads on all four | `on_primary` |

MixMusic's template and a `theme.json` give the tokens themselves. The
fourth tile of matugen's JSON is the wallpaper's own colour, unless it is
all but the primary: then it falls between the first two.

### Changes and failures

- MixMusic watches the file. When pywal or matugen writes it again, the
  main window and the mini player change colour within a moment, with no
  reload. **Reload add-ons** reads it again too, as a file on a network
  share may need.
- Both tools empty the file before they write it again, so MixMusic may
  find it half written. It reads it again a moment later, and until the
  file reads whole, it keeps the colours it read last, never the broken
  ones. If the file still doesn't read, Settings says what's wrong.
- If the file goes away, the theme's colours come back. If it isn't there
  yet because pywal or matugen hasn't run, MixMusic looks for it every few
  seconds.
- If the file is over 1 MB, is a folder, or holds no colours MixMusic
  knows, MixMusic keeps the colours it read last, or the theme's if it
  has read none, and Settings says why.
- pywal 3.3.0 on Windows writes the wallpaper's path into `colors.json`
  without escaping its backslashes, which isn't valid JSON. MixMusic reads
  the colours from it anyway.

## Share it

Themes in [community/themes](../community/themes) can be installed by everyone
from the marketplace. [community/README.md](../community/README.md) tells how
to add yours.
