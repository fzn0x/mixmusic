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
| `radius` | the corner radius of tiles and cards in the Tiles layout, as `12px`; smaller things round by 60% of it |
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

The Blend, Expressive and Crate layouts fill what is chosen, as the
current page, and their secondary buttons with `surface-3`: `text` on it
needs 4.5:1 as well. They set the accent as small text on the panels, so
there MixMusic darkens or lightens yours until it reads at 4.5:1 on
`surface`.

### Fonts

MixMusic ships Selawik, Quicksand, Mochi's round face, Roboto Flex, the
Material themes' face, at weights 400, 500 and 700, and Archivo
SemiCondensed at 800, the Walnut themes' headings'; other families must be
on the user's computer, so end each list with a family every system has:
`system-ui` or `sans-serif` for the system's font, `monospace` for its
monospaced one.

### Layouts

A theme sets the colors, the faces and the corners; the layout is the
user's to choose, in Settings › Appearance, with any theme:

- **Blend**, the default: Expressive's rounded shapes and springs, on
  Crate's dense rows, big covers in cards and bold headings.
- **Expressive**, after Material 3 Expressive: tonal fills, big rounded
  shapes, pill buttons, a floating player bar and springy motion.
- **Crate**: dense rows, big covers in cards and bold, tight headings.
- **Tiles**, MixMusic's first: flat tiles, giant light lowercase words
  and outlined boxes. Only Tiles rounds with your `radius`; the others
  have corners of their own.

The layouts set your `display` face at their own weights: light in Tiles,
regular in Expressive, and heavy in Blend and Crate. A face with those
weights looks its best in all of them.

## Effects

The themes MixMusic ships with add effects of their own, which MixMusic
draws and the user tunes in Settings › Appearance: Mix's tile textures,
Aurora's drifting lights and frosted glass, Sakura's petals, Terminal's
scanlines and glow, Greenroom's and Glasshouse's grounds in the colour of
the cover playing, Anime's speed lines and screentone, and Mochi's rising
sparkles, and the Material themes' colours, which MixMusic makes of a
seed colour in Material's HCT color space, as Material's dynamic color
does: the cover's while it plays and "Follow the artwork" is on, the
accent you chose otherwise, or MixMusic's pink. A theme of yours sets the
tokens above, and MixMusic draws
everything else with them.

Themes made for the web interface of MixMusic 0.4 and before still load:
MixMusic reads their colors, fonts and radius, and ignores their stylesheet
and their options.

## Share it

Themes in [community/themes](../community/themes) can be installed by everyone
from the marketplace. [community/README.md](../community/README.md) tells how
to add yours.
